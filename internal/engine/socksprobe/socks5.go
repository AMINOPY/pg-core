// Package socksprobe implements bounded TCP reachability checks through a
// loopback SOCKS5 proxy. It does not validate TLS, HTTP semantics, or VPN routing.
package socksprobe

import (
	"context"
	"errors"
	"io"
	"net"
	"strconv"
	"strings"
	"time"
)

// ValidateTarget accepts an explicit host:port pair for a health probe.
func ValidateTarget(target string) error {
	host, portText, err := net.SplitHostPort(target)
	if err != nil || strings.TrimSpace(host) == "" || strings.ContainsAny(host, "\r\n\t ") {
		return errors.New("health target must be a host:port pair")
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return errors.New("health target port must be between 1 and 65535")
	}
	return nil
}

// ProbeTCP performs a SOCKS5 no-auth CONNECT to target. It uses fixed I/O
// deadlines so a stalled proxy cannot hang the caller indefinitely.
func ProbeTCP(ctx context.Context, proxyAddr, target string) error {
	if ctx == nil {
		return errors.New("context is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := ValidateTarget(target); err != nil {
		return err
	}
	conn, err := (&net.Dialer{Timeout: 500 * time.Millisecond}).DialContext(ctx, "tcp", proxyAddr)
	if err != nil {
		return err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(time.Second))
	if _, err := conn.Write([]byte{5, 1, 0}); err != nil {
		return err
	}
	reply := make([]byte, 2)
	if _, err := io.ReadFull(conn, reply); err != nil {
		return err
	}
	if reply[0] != 5 || reply[1] != 0 {
		return errors.New("SOCKS5 proxy does not allow no-auth negotiation")
	}
	host, portText, err := net.SplitHostPort(target)
	if err != nil {
		return errors.New("invalid SOCKS target address")
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return errors.New("invalid SOCKS target port")
	}
	ip := net.ParseIP(host)
	var request []byte
	if ip4 := ip.To4(); ip4 != nil {
		request = append([]byte{5, 1, 0, 1}, ip4...)
	} else if ip6 := ip.To16(); ip6 != nil {
		request = append([]byte{5, 1, 0, 4}, ip6...)
	} else {
		if len(host) == 0 || len(host) > 255 {
			return errors.New("invalid SOCKS target hostname")
		}
		request = append([]byte{5, 1, 0, 3, byte(len(host))}, []byte(host)...)
	}
	request = append(request, byte(port>>8), byte(port))
	if _, err := conn.Write(request); err != nil {
		return err
	}
	head := make([]byte, 4)
	if _, err := io.ReadFull(conn, head); err != nil {
		return err
	}
	if head[0] != 5 || head[1] != 0 {
		return errors.New("SOCKS5 connect request failed")
	}
	n := 0
	switch head[3] {
	case 1:
		n = 4
	case 4:
		n = 16
	case 3:
		length := make([]byte, 1)
		if _, err := io.ReadFull(conn, length); err != nil {
			return err
		}
		n = int(length[0])
	default:
		return errors.New("invalid SOCKS5 address type")
	}
	tail := make([]byte, n+2)
	_, err = io.ReadFull(conn, tail)
	return err
}
