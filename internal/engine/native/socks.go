package native

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"net/netip"
	"sync"
	"time"
)

// serveSOCKS5 accepts SOCKS5 CONNECT and UDP ASSOCIATE requests. It must only
// listen on a loopback address because this endpoint does not authenticate users.
func serveSOCKS5(ctx context.Context, ln net.Listener, d *Dialer) {
	var handlers sync.WaitGroup
	for {
		c, err := ln.Accept()
		if err != nil {
			break
		}
		handlers.Add(1)
		go func() {
			defer handlers.Done()
			handleSOCKS(ctx, c, d)
		}()
	}
	handlers.Wait()
}

func handleSOCKS(ctx context.Context, c net.Conn, d *Dialer) {
	defer c.Close()
	stopOnCancel := context.AfterFunc(ctx, func() { _ = c.Close() })
	defer stopOnCancel()
	_ = c.SetDeadline(time.Now().Add(15 * time.Second))
	cmd, host, port, err := socksHandshakeCommand(c)
	if err != nil {
		return
	}
	if cmd == 3 {
		handleSOCKSUDP(ctx, c, d)
		return
	}
	t, err := d.DialTarget(ctx, host, port)
	if err != nil {
		_, _ = c.Write([]byte{5, 5, 0, 1, 0, 0, 0, 0, 0, 0})
		return
	}
	_, _ = c.Write([]byte{5, 0, 0, 1, 0, 0, 0, 0, 0, 0})
	_ = c.SetDeadline(time.Time{})
	pipe(c, t)
}

// socksHandshake preserves the TCP-only helper contract for callers/tests that
// need to reject non-CONNECT commands explicitly.
func socksHandshake(c net.Conn) (string, int, error) {
	cmd, host, port, err := socksHandshakeCommand(c)
	if err != nil {
		return "", 0, err
	}
	if cmd != 1 {
		_, _ = c.Write([]byte{5, 7, 0, 1, 0, 0, 0, 0, 0, 0})
		return "", 0, errors.New("UDP ASSOCIATE is handled by the SOCKS server, not the CONNECT helper")
	}
	return host, port, nil
}

func socksHandshakeCommand(c net.Conn) (byte, string, int, error) {
	var h [2]byte
	if _, err := io.ReadFull(c, h[:]); err != nil || h[0] != 5 {
		return 0, "", 0, errors.New("not socks5")
	}
	methods := make([]byte, int(h[1]))
	if _, err := io.ReadFull(c, methods); err != nil {
		return 0, "", 0, err
	}
	noAuth := false
	for _, method := range methods {
		if method == 0 {
			noAuth = true
			break
		}
	}
	if !noAuth {
		_, writeErr := c.Write([]byte{5, 0xff})
		if writeErr != nil {
			return 0, "", 0, writeErr
		}
		return 0, "", 0, errors.New("client does not offer SOCKS5 no-auth method")
	}
	if _, err := c.Write([]byte{5, 0}); err != nil {
		return 0, "", 0, err
	}
	var r [4]byte
	if _, err := io.ReadFull(c, r[:]); err != nil || r[0] != 5 || r[2] != 0 {
		return 0, "", 0, errors.New("bad request")
	}
	if r[1] != 1 && r[1] != 3 {
		_, _ = c.Write([]byte{5, 7, 0, 1, 0, 0, 0, 0, 0, 0})
		return 0, "", 0, errors.New("unsupported command")
	}
	var host string
	switch r[3] {
	case 1:
		var a [4]byte
		if _, err := io.ReadFull(c, a[:]); err != nil {
			return 0, "", 0, err
		}
		host = netip.AddrFrom4(a).String()
	case 4:
		var a [16]byte
		if _, err := io.ReadFull(c, a[:]); err != nil {
			return 0, "", 0, err
		}
		host = netip.AddrFrom16(a).String()
	case 3:
		var l [1]byte
		if _, err := io.ReadFull(c, l[:]); err != nil || l[0] == 0 {
			return 0, "", 0, errors.New("bad domain")
		}
		n := make([]byte, int(l[0]))
		if _, err := io.ReadFull(c, n); err != nil {
			return 0, "", 0, err
		}
		host = string(n)
	default:
		return 0, "", 0, errors.New("bad address type")
	}
	var p [2]byte
	if _, err := io.ReadFull(c, p[:]); err != nil {
		return 0, "", 0, err
	}
	port := int(binary.BigEndian.Uint16(p[:]))
	if r[1] == 1 && port == 0 {
		return 0, "", 0, errors.New("invalid target port")
	}
	return r[1], host, port, nil
}

func handleSOCKSUDP(ctx context.Context, control net.Conn, d *Dialer) {
	relay, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		_, _ = control.Write([]byte{5, 1, 0, 1, 0, 0, 0, 0, 0, 0})
		return
	}
	defer relay.Close()
	addr := relay.LocalAddr().(*net.UDPAddr)
	reply := []byte{5, 0, 0, 1, 127, 0, 0, 1, 0, 0}
	binary.BigEndian.PutUint16(reply[8:10], uint16(addr.Port))
	if _, err := control.Write(reply); err != nil {
		return
	}
	_ = control.SetDeadline(time.Time{})

	stopCancel := context.AfterFunc(ctx, func() { _ = relay.Close() })
	defer stopCancel()
	// A SOCKS UDP association lives as long as its TCP control connection.
	go func() {
		var b [1]byte
		_, _ = control.Read(b[:])
		_ = relay.Close()
	}()

	var client *net.UDPAddr
	buf := make([]byte, 65535)
	for {
		n, peer, err := relay.ReadFromUDP(buf)
		if err != nil {
			return
		}
		if tcpPeer, ok := control.RemoteAddr().(*net.TCPAddr); ok && !peer.IP.Equal(tcpPeer.IP) {
			continue
		}
		if client == nil {
			client = peer
		} else if !client.IP.Equal(peer.IP) || client.Port != peer.Port {
			continue
		}
		host, port, payload, err := parseSOCKSUDP(buf[:n])
		if err != nil {
			continue
		}
		reqCtx, cancel := context.WithTimeout(ctx, d.timeout())
		response, err := d.DialDatagram(reqCtx, host, port, payload)
		cancel()
		if err != nil {
			continue
		}
		target, err := encodeTarget(host, port)
		if err != nil {
			continue
		}
		packet := make([]byte, 3+len(target)+len(response))
		packet[0], packet[1], packet[2] = 0, 0, 0
		copy(packet[3:], target)
		copy(packet[3+len(target):], response)
		if _, err := relay.WriteToUDP(packet, client); err != nil {
			return
		}
	}
}

func parseSOCKSUDP(packet []byte) (string, int, []byte, error) {
	if len(packet) < 4 || packet[0] != 0 || packet[1] != 0 || packet[2] != 0 {
		return "", 0, nil, errors.New("invalid or fragmented SOCKS UDP packet")
	}
	r := packet[3:]
	var host string
	var i int
	switch r[0] {
	case 1:
		if len(r) < 7 {
			return "", 0, nil, io.ErrUnexpectedEOF
		}
		host, i = netip.AddrFrom4([4]byte{r[1], r[2], r[3], r[4]}).String(), 5
	case 4:
		if len(r) < 19 {
			return "", 0, nil, io.ErrUnexpectedEOF
		}
		var a [16]byte
		copy(a[:], r[1:17])
		host, i = netip.AddrFrom16(a).String(), 17
	case 3:
		if len(r) < 2 || r[1] == 0 {
			return "", 0, nil, errors.New("invalid SOCKS UDP domain")
		}
		n := int(r[1])
		if len(r) < n+4 {
			return "", 0, nil, io.ErrUnexpectedEOF
		}
		end := 2 + n
		host, i = string(r[2:end]), end
	default:
		return "", 0, nil, errors.New("unsupported SOCKS UDP address type")
	}
	if len(r) < i+2 {
		return "", 0, nil, io.ErrUnexpectedEOF
	}
	port := int(binary.BigEndian.Uint16(r[i : i+2]))
	if port == 0 || len(r) == i+2 {
		return "", 0, nil, errors.New("invalid SOCKS UDP destination or empty payload")
	}
	return host, port, r[i+2:], nil
}
