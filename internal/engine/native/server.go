package native

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/binary"
	"io"
	"net"
	"net/http"
	"time"
)

// Server is the VPS-side counterpart (use it behind a CDN or TLS terminator).
// The Cloudflare Worker in deploy/ speaks the same protocol, so no VPS is needed.
type Server struct {
	// Path is the secret URL path, for example "/s3cr3t". Anything else gets 404.
	Path string
	// Dial connects to the requested target. Defaults to net.Dialer.
	Dial        func(ctx context.Context, network, addr string) (net.Conn, error)
	DialTimeout time.Duration
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !isWebSocket(r) || s.Path == "" || subtle.ConstantTimeCompare([]byte(r.URL.Path), []byte(s.Path)) != 1 {
		http.NotFound(w, r)
		return
	}
	ws, err := upgrade(w, r)
	if err != nil {
		return
	}
	defer ws.Close()

	var first [1]byte
	if _, err := io.ReadFull(ws, first[:]); err != nil {
		return
	}
	udpMode := first[0] == udpModeMarker
	var host string
	var port int
	if udpMode {
		host, port, err = readTarget(ws)
	} else {
		host, port, err = readTarget(io.MultiReader(bytes.NewReader(first[:]), ws))
	}
	if err != nil {
		return
	}
	timeout := s.DialTimeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()
	dial := s.Dial
	if dial == nil {
		dial = (&net.Dialer{}).DialContext
	}
	if udpMode {
		target, err := dial(ctx, "udp", hostPort(host, port))
		if err != nil {
			_, _ = ws.Write([]byte{1})
			return
		}
		defer target.Close()
		if _, err := ws.Write([]byte{0}); err != nil {
			return
		}
		var size [2]byte
		if _, err := io.ReadFull(ws, size[:]); err != nil {
			return
		}
		n := int(binary.BigEndian.Uint16(size[:]))
		if n == 0 {
			return
		}
		payload := make([]byte, n)
		if _, err := io.ReadFull(ws, payload); err != nil {
			return
		}
		if _, err := target.Write(payload); err != nil {
			return
		}
		_ = target.SetDeadline(time.Now().Add(timeout))
		response := make([]byte, 65507)
		got, err := target.Read(response)
		if err != nil || got == 0 || got > 65535 {
			return
		}
		binary.BigEndian.PutUint16(size[:], uint16(got))
		if err := writeAll(ws, append(size[:], response[:got]...)); err != nil {
			return
		}
		return
	}
	target, err := dial(ctx, "tcp", hostPort(host, port))
	if err != nil {
		_, _ = ws.Write([]byte{1})
		return
	}
	if _, err := ws.Write([]byte{0}); err != nil {
		target.Close()
		return
	}
	pipe(ws, target)
}
