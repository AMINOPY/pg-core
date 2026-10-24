package native

import (
	"context"
	"crypto/subtle"
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

	host, port, err := readTarget(ws)
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
