package native

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

func validateHealthURL(raw string) error {
	if raw == "" {
		return nil
	}
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" || u.User != nil || (u.Scheme != "https" && u.Scheme != "http") {
		return errors.New("health_url must be an absolute HTTPS URL without credentials")
	}
	if u.Scheme == "http" {
		ip := net.ParseIP(u.Hostname())
		if u.Hostname() != "localhost" && (ip == nil || !ip.IsLoopback()) {
			return errors.New("health_url may use HTTP only for loopback test targets")
		}
	}
	if port := u.Port(); port != "" {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return errors.New("health_url has an invalid port")
		}
	}
	return nil
}

// checkDataPlane verifies an actual HTTP response through the engine's local
// SOCKS5 listener. Unlike an edge handshake, this proves a request traverses
// the tunnel to the configured destination.
func checkDataPlane(ctx context.Context, socksAddr, rawURL string) error {
	if err := validateHealthURL(rawURL); err != nil {
		return err
	}
	u, _ := url.Parse(rawURL)
	tr := &http.Transport{
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			return socks5Connect(ctx, socksAddr, address)
		},
	}
	defer tr.CloseIdleConnections()
	client := &http.Client{
		Transport: tr,
		Timeout:   8 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request through SOCKS5: %w", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("health endpoint returned HTTP %d", resp.StatusCode)
	}
	return nil
}

func socks5Connect(ctx context.Context, proxyAddr, targetAddr string) (net.Conn, error) {
	host, portText, err := net.SplitHostPort(targetAddr)
	if err != nil {
		return nil, err
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return nil, errors.New("invalid target port")
	}
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", proxyAddr)
	if err != nil {
		return nil, err
	}
	fail := func(err error) (net.Conn, error) {
		_ = conn.Close()
		return nil, err
	}
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	if _, err := conn.Write([]byte{5, 1, 0}); err != nil {
		return fail(err)
	}
	var greeting [2]byte
	if _, err := io.ReadFull(conn, greeting[:]); err != nil {
		return fail(err)
	}
	if greeting != [2]byte{5, 0} {
		return fail(errors.New("SOCKS5 proxy does not allow no-auth mode"))
	}
	var req []byte
	if ip := net.ParseIP(host); ip != nil {
		if v4 := ip.To4(); v4 != nil {
			req = append([]byte{5, 1, 0, 1}, v4...)
		} else {
			req = append([]byte{5, 1, 0, 4}, ip.To16()...)
		}
	} else {
		if len(host) == 0 || len(host) > 255 {
			return fail(errors.New("invalid SOCKS5 target hostname"))
		}
		req = append([]byte{5, 1, 0, 3, byte(len(host))}, []byte(host)...)
	}
	var p [2]byte
	binary.BigEndian.PutUint16(p[:], uint16(port))
	req = append(req, p[:]...)
	if _, err := conn.Write(req); err != nil {
		return fail(err)
	}
	var head [4]byte
	if _, err := io.ReadFull(conn, head[:]); err != nil {
		return fail(err)
	}
	if head[0] != 5 || head[1] != 0 {
		return fail(fmt.Errorf("SOCKS5 connect failed with reply %d", head[1]))
	}
	var addrLen int
	switch head[3] {
	case 1:
		addrLen = net.IPv4len
	case 4:
		addrLen = net.IPv6len
	case 3:
		var n [1]byte
		if _, err := io.ReadFull(conn, n[:]); err != nil {
			return fail(err)
		}
		addrLen = int(n[0])
	default:
		return fail(errors.New("invalid SOCKS5 address type"))
	}
	if _, err := io.CopyN(io.Discard, conn, int64(addrLen+2)); err != nil {
		return fail(err)
	}
	_ = conn.SetDeadline(time.Time{})
	return conn, nil
}
