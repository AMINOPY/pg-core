package native

import (
	"context"
	"errors"
	"io"
	"net"
	"net/netip"
	"time"
)

// serveSOCKS5 accepts SOCKS5 CONNECT requests (no authentication) and sends
// them through the tunnel. It must only listen on a loopback address.
func serveSOCKS5(ctx context.Context, ln net.Listener, d *Dialer) {
	for {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		go handleSOCKS(ctx, c, d)
	}
}

func handleSOCKS(ctx context.Context, c net.Conn, d *Dialer) {
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(15 * time.Second))
	host, port, err := socksHandshake(c)
	if err != nil {
		return
	}
	t, err := d.DialTarget(ctx, host, port)
	if err != nil {
		_, _ = c.Write([]byte{5, 5, 0, 1, 0, 0, 0, 0, 0, 0}) // connection refused
		return
	}
	_, _ = c.Write([]byte{5, 0, 0, 1, 0, 0, 0, 0, 0, 0})
	_ = c.SetDeadline(time.Time{})
	pipe(c, t)
}

func socksHandshake(c net.Conn) (string, int, error) {
	var h [2]byte
	if _, err := io.ReadFull(c, h[:]); err != nil || h[0] != 5 {
		return "", 0, errors.New("not socks5")
	}
	methods := make([]byte, h[1])
	if _, err := io.ReadFull(c, methods); err != nil {
		return "", 0, err
	}
	if _, err := c.Write([]byte{5, 0}); err != nil {
		return "", 0, err
	}
	var r [4]byte
	if _, err := io.ReadFull(c, r[:]); err != nil || r[0] != 5 {
		return "", 0, errors.New("bad request")
	}
	if r[1] != 1 {
		_, _ = c.Write([]byte{5, 7, 0, 1, 0, 0, 0, 0, 0, 0}) // only CONNECT
		return "", 0, errors.New("unsupported command")
	}
	var host string
	switch r[3] {
	case 1:
		var a [4]byte
		if _, err := io.ReadFull(c, a[:]); err != nil {
			return "", 0, err
		}
		host = netip.AddrFrom4(a).String()
	case 4:
		var a [16]byte
		if _, err := io.ReadFull(c, a[:]); err != nil {
			return "", 0, err
		}
		host = netip.AddrFrom16(a).String()
	case 3:
		var l [1]byte
		if _, err := io.ReadFull(c, l[:]); err != nil || l[0] == 0 {
			return "", 0, errors.New("bad domain")
		}
		n := make([]byte, l[0])
		if _, err := io.ReadFull(c, n); err != nil {
			return "", 0, err
		}
		host = string(n)
	default:
		return "", 0, errors.New("bad address type")
	}
	var p [2]byte
	if _, err := io.ReadFull(c, p[:]); err != nil {
		return "", 0, err
	}
	return host, int(p[0])<<8 | int(p[1]), nil
}
