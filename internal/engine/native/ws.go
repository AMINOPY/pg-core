package native

import (
	"bufio"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
)

// Minimal RFC 6455 WebSocket, standard library only. It carries a byte stream
// in binary frames; text/continuation frames are accepted as data too.

const (
	wsGUID                                            = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"
	maxFrame                                          = 16 << 20
	writeChunk                                        = 64 << 10
	opCont, opText, opBinary, opClose, opPing, opPong = 0, 1, 2, 8, 9, 10
)

type wsConn struct {
	c      net.Conn
	br     *bufio.Reader
	client bool

	rem     int64
	masked  bool
	mask    [4]byte
	maskPos int

	wmu    sync.Mutex
	closed bool
}

func newWS(c net.Conn, br *bufio.Reader, client bool) *wsConn {
	if br == nil {
		br = bufio.NewReader(c)
	}
	return &wsConn{c: c, br: br, client: client}
}

func (w *wsConn) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	for w.rem == 0 {
		if err := w.nextFrame(); err != nil {
			return 0, err
		}
	}
	n := int64(len(p))
	if n > w.rem {
		n = w.rem
	}
	m, err := w.br.Read(p[:n])
	if w.masked {
		for i := 0; i < m; i++ {
			p[i] ^= w.mask[(w.maskPos+i)&3]
		}
		w.maskPos = (w.maskPos + m) & 3
	}
	w.rem -= int64(m)
	return m, err
}

func (w *wsConn) nextFrame() error {
	var h [2]byte
	if _, err := io.ReadFull(w.br, h[:]); err != nil {
		return err
	}
	op := h[0] & 0x0f
	masked := h[1]&0x80 != 0
	l := int64(h[1] & 0x7f)
	switch l {
	case 126:
		var b [2]byte
		if _, err := io.ReadFull(w.br, b[:]); err != nil {
			return err
		}
		l = int64(binary.BigEndian.Uint16(b[:]))
	case 127:
		var b [8]byte
		if _, err := io.ReadFull(w.br, b[:]); err != nil {
			return err
		}
		u := binary.BigEndian.Uint64(b[:])
		if u > maxFrame {
			return errors.New("websocket frame too large")
		}
		l = int64(u)
	}
	if l > maxFrame {
		return errors.New("websocket frame too large")
	}
	var mask [4]byte
	if masked {
		if _, err := io.ReadFull(w.br, mask[:]); err != nil {
			return err
		}
	}
	switch op {
	case opCont, opText, opBinary:
		w.rem, w.masked, w.mask, w.maskPos = l, masked, mask, 0
		return nil
	case opClose:
		return io.EOF
	case opPing, opPong:
		if l > 125 {
			return errors.New("websocket control frame too large")
		}
		payload := make([]byte, l)
		if _, err := io.ReadFull(w.br, payload); err != nil {
			return err
		}
		if masked {
			for i := range payload {
				payload[i] ^= mask[i&3]
			}
		}
		if op == opPing {
			return w.writeFrame(opPong, payload)
		}
		return nil
	default:
		return fmt.Errorf("unsupported websocket opcode %d", op)
	}
}

func (w *wsConn) Write(p []byte) (int, error) {
	total := 0
	for len(p) > 0 {
		n := len(p)
		if n > writeChunk {
			n = writeChunk
		}
		if err := w.writeFrame(opBinary, p[:n]); err != nil {
			return total, err
		}
		total += n
		p = p[n:]
	}
	return total, nil
}

func (w *wsConn) writeFrame(op byte, payload []byte) error {
	w.wmu.Lock()
	defer w.wmu.Unlock()
	if w.closed && op != opClose {
		return io.ErrClosedPipe
	}
	hdr := make([]byte, 0, 14)
	hdr = append(hdr, 0x80|op)
	maskBit := byte(0)
	if w.client {
		maskBit = 0x80
	}
	switch l := len(payload); {
	case l < 126:
		hdr = append(hdr, maskBit|byte(l))
	case l <= 0xffff:
		hdr = append(hdr, maskBit|126, byte(l>>8), byte(l))
	default:
		var b [8]byte
		binary.BigEndian.PutUint64(b[:], uint64(l))
		hdr = append(hdr, maskBit|127)
		hdr = append(hdr, b[:]...)
	}
	body := payload
	if w.client {
		var key [4]byte
		if _, err := rand.Read(key[:]); err != nil {
			return err
		}
		hdr = append(hdr, key[:]...)
		body = make([]byte, len(payload))
		for i := range payload {
			body[i] = payload[i] ^ key[i&3]
		}
	}
	if _, err := w.c.Write(append(hdr, body...)); err != nil {
		return err
	}
	return nil
}

// CloseWrite tells the peer that no more data will be sent.
func (w *wsConn) CloseWrite() error {
	return w.writeFrame(opClose, nil)
}

func (w *wsConn) Close() error {
	w.wmu.Lock()
	already := w.closed
	w.closed = true
	w.wmu.Unlock()
	if !already {
		_ = w.writeFrame(opClose, nil)
	}
	return w.c.Close()
}

func acceptKey(key string) string {
	sum := sha1.Sum([]byte(key + wsGUID))
	return base64.StdEncoding.EncodeToString(sum[:])
}

func isWebSocket(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("Upgrade"), "websocket") &&
		strings.Contains(strings.ToLower(r.Header.Get("Connection")), "upgrade") &&
		r.Header.Get("Sec-WebSocket-Key") != ""
}

// upgrade completes the server side of the handshake.
func upgrade(w http.ResponseWriter, r *http.Request) (*wsConn, error) {
	hj, ok := w.(http.Hijacker)
	if !ok {
		return nil, errors.New("response writer cannot be hijacked")
	}
	c, rw, err := hj.Hijack()
	if err != nil {
		return nil, err
	}
	resp := "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: " +
		acceptKey(r.Header.Get("Sec-WebSocket-Key")) + "\r\n\r\n"
	if _, err := rw.WriteString(resp); err != nil {
		c.Close()
		return nil, err
	}
	if err := rw.Flush(); err != nil {
		c.Close()
		return nil, err
	}
	return newWS(c, rw.Reader, false), nil
}

// clientHandshake performs the client side over an established (TLS) connection.
func clientHandshake(c net.Conn, host, path string) (*wsConn, error) {
	var k [16]byte
	if _, err := rand.Read(k[:]); err != nil {
		return nil, err
	}
	key := base64.StdEncoding.EncodeToString(k[:])
	req, err := http.NewRequest(http.MethodGet, "https://"+host+path, nil)
	if err != nil {
		return nil, err
	}
	req.Host = host
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Sec-WebSocket-Key", key)
	req.Header.Set("Sec-WebSocket-Version", "13")
	if err := req.Write(c); err != nil {
		return nil, err
	}
	br := bufio.NewReader(c)
	resp, err := http.ReadResponse(br, req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusSwitchingProtocols || resp.Header.Get("Sec-WebSocket-Accept") != acceptKey(key) {
		return nil, fmt.Errorf("websocket upgrade refused (status %d)", resp.StatusCode)
	}
	return newWS(c, br, true), nil
}
