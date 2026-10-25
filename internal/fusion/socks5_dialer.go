package fusion

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"time"
)

var (
	ErrInvalidSOCKS5Proxy = errors.New("invalid SOCKS5 proxy address")
	ErrUnsupportedNetwork = errors.New("SOCKS5 dialer supports TCP networks only")
)

// SOCKS5Dialer opens outbound connections through an existing local SOCKS5
// listener, such as one exposed by a separately managed proxy engine. It
// supports RFC 1928 no-auth CONNECT only; it does not start or configure that
// proxy process.
type SOCKS5Dialer struct {
	ProxyAddress string
	Dialer       *net.Dialer
}

// DialContext implements ContextDialer. Context cancellation applies to the
// proxy TCP connection and to the SOCKS5 handshake, not just the initial dial.
func (d SOCKS5Dialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	if ctx == nil {
		return nil, errors.New("context is required")
	}
	switch network {
	case "tcp", "tcp4", "tcp6":
	default:
		return nil, fmt.Errorf("%w: %q", ErrUnsupportedNetwork, network)
	}
	if _, _, err := net.SplitHostPort(d.ProxyAddress); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidSOCKS5Proxy, err)
	}
	proxyHost, proxyPort, err := net.SplitHostPort(d.ProxyAddress)
	if err != nil || proxyHost == "" {
		return nil, fmt.Errorf("%w: expected host:port", ErrInvalidSOCKS5Proxy)
	}
	port, err := strconv.Atoi(proxyPort)
	if err != nil || port < 1 || port > 65535 {
		return nil, fmt.Errorf("%w: invalid port", ErrInvalidSOCKS5Proxy)
	}
	request, err := socks5ConnectRequest(address)
	if err != nil {
		return nil, err
	}

	dialer := d.Dialer
	if dialer == nil {
		dialer = &net.Dialer{}
	}
	conn, err := dialer.DialContext(ctx, "tcp", d.ProxyAddress)
	if err != nil {
		return nil, fmt.Errorf("connect to SOCKS5 proxy: %w", err)
	}
	connected := false
	defer func() {
		if !connected {
			_ = conn.Close()
		}
	}()

	stopCancel := context.AfterFunc(ctx, func() { _ = conn.Close() })
	cancelWatchActive := true
	defer func() {
		if cancelWatchActive {
			stopCancel()
		}
	}()
	if deadline, ok := ctx.Deadline(); ok {
		if err := conn.SetDeadline(deadline); err != nil {
			return nil, fmt.Errorf("set SOCKS5 handshake deadline: %w", err)
		}
	}

	if err := writeAll(conn, []byte{5, 1, 0}); err != nil {
		return nil, contextIOError(ctx, "write SOCKS5 greeting", err)
	}
	var methodReply [2]byte
	if _, err := io.ReadFull(conn, methodReply[:]); err != nil {
		return nil, contextIOError(ctx, "read SOCKS5 method reply", err)
	}
	if methodReply != [2]byte{5, 0} {
		return nil, fmt.Errorf("SOCKS5 proxy rejected no-auth method (reply %v)", methodReply)
	}
	if err := writeAll(conn, request); err != nil {
		return nil, contextIOError(ctx, "write SOCKS5 CONNECT request", err)
	}
	if err := readSOCKS5ConnectReply(ctx, conn); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !stopCancel() {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		return nil, context.Canceled
	}
	cancelWatchActive = false
	if err := conn.SetDeadline(time.Time{}); err != nil {
		return nil, fmt.Errorf("clear SOCKS5 handshake deadline: %w", err)
	}
	connected = true
	return conn, nil
}

func socks5ConnectRequest(address string) ([]byte, error) {
	host, portText, err := net.SplitHostPort(address)
	if err != nil || host == "" {
		return nil, fmt.Errorf("invalid SOCKS5 target address %q", address)
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return nil, fmt.Errorf("invalid SOCKS5 target port %q", portText)
	}

	request := []byte{5, 1, 0}
	if ip := net.ParseIP(host); ip != nil {
		if ip4 := ip.To4(); ip4 != nil {
			request = append(request, 1)
			request = append(request, ip4...)
		} else {
			request = append(request, 4)
			request = append(request, ip.To16()...)
		}
	} else {
		if len(host) > 255 {
			return nil, errors.New("SOCKS5 target hostname exceeds 255 bytes")
		}
		request = append(request, 3, byte(len(host)))
		request = append(request, host...)
	}
	var portBytes [2]byte
	binary.BigEndian.PutUint16(portBytes[:], uint16(port))
	request = append(request, portBytes[:]...)
	return request, nil
}

func readSOCKS5ConnectReply(ctx context.Context, conn net.Conn) error {
	var header [4]byte
	if _, err := io.ReadFull(conn, header[:]); err != nil {
		return contextIOError(ctx, "read SOCKS5 CONNECT reply", err)
	}
	if header[0] != 5 || header[2] != 0 {
		return errors.New("malformed SOCKS5 CONNECT reply")
	}
	if header[1] != 0 {
		return fmt.Errorf("SOCKS5 CONNECT failed with reply code %d", header[1])
	}

	switch header[3] {
	case 1:
		if _, err := io.CopyN(io.Discard, conn, 4); err != nil {
			return contextIOError(ctx, "read SOCKS5 IPv4 bind address", err)
		}
	case 3:
		var length [1]byte
		if _, err := io.ReadFull(conn, length[:]); err != nil {
			return contextIOError(ctx, "read SOCKS5 domain bind length", err)
		}
		if length[0] == 0 {
			return errors.New("malformed SOCKS5 domain bind address")
		}
		if _, err := io.CopyN(io.Discard, conn, int64(length[0])); err != nil {
			return contextIOError(ctx, "read SOCKS5 domain bind address", err)
		}
	case 4:
		if _, err := io.CopyN(io.Discard, conn, 16); err != nil {
			return contextIOError(ctx, "read SOCKS5 IPv6 bind address", err)
		}
	default:
		return errors.New("unsupported SOCKS5 bind address type")
	}
	var bindPort [2]byte
	if _, err := io.ReadFull(conn, bindPort[:]); err != nil {
		return contextIOError(ctx, "read SOCKS5 bind port", err)
	}
	return nil
}

func contextIOError(ctx context.Context, operation string, err error) error {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	return fmt.Errorf("%s: %w", operation, err)
}

func writeAll(w io.Writer, data []byte) error {
	for len(data) > 0 {
		n, err := w.Write(data)
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		data = data[n:]
	}
	return nil
}
