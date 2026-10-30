package fusion

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// SOCKS5ChainDialer tunnels through an ordered list of already-running SOCKS5
// proxies. The first hop is reached directly; each subsequent hop is reached
// by issuing CONNECT through the preceding hop. The final CONNECT targets the
// requested destination. All hops use RFC 1928 no-auth CONNECT.
type SOCKS5ChainDialer struct {
	Hops   []string
	Dialer *net.Dialer
	// Control runs on the first-hop socket before connect completes. Android
	// callers must use it to invoke VpnService.protect(fd), failing closed.
	Control func(network, address string, raw syscall.RawConn) error
}

// DialContext implements ContextDialer for TCP flows through a SOCKS5 chain.
// It does not own or start the proxy listeners.
func (d SOCKS5ChainDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	if ctx == nil {
		return nil, errors.New("context is required")
	}
	switch network {
	case "tcp", "tcp4", "tcp6":
	default:
		return nil, fmt.Errorf("%w: %q", ErrUnsupportedNetwork, network)
	}
	if len(d.Hops) == 0 {
		return nil, fmt.Errorf("%w: at least one SOCKS5 hop is required", ErrInvalidSOCKS5Proxy)
	}
	for i, hop := range d.Hops {
		if err := validateSOCKS5ProxyAddress(hop); err != nil {
			return nil, fmt.Errorf("SOCKS5 hop %d: %w", i+1, err)
		}
	}
	if _, _, err := net.SplitHostPort(address); err != nil {
		return nil, fmt.Errorf("invalid SOCKS5 target address %q: %w", address, err)
	}

	dialer := d.Dialer
	if dialer == nil {
		dialer = &net.Dialer{}
	}
	if d.Control != nil {
		// Do not mutate a caller-owned Dialer: it may be shared by other routes.
		clone := *dialer
		previousControl := clone.Control
		clone.Control = func(network, address string, raw syscall.RawConn) error {
			if previousControl != nil {
				if err := previousControl(network, address, raw); err != nil {
					return err
				}
			}
			return d.Control(network, address, raw)
		}
		dialer = &clone
	}
	conn, err := dialer.DialContext(ctx, "tcp", d.Hops[0])
	if err != nil {
		return nil, fmt.Errorf("connect to first SOCKS5 hop: %w", err)
	}
	keep := false
	defer func() {
		if !keep {
			_ = conn.Close()
		}
	}()
	stopCancel := context.AfterFunc(ctx, func() { _ = conn.Close() })
	watchActive := true
	defer func() {
		if watchActive {
			stopCancel()
		}
	}()
	if deadline, ok := ctx.Deadline(); ok {
		if err := conn.SetDeadline(deadline); err != nil {
			return nil, fmt.Errorf("set SOCKS5 chain deadline: %w", err)
		}
	}

	for i := range d.Hops {
		target := address
		if i+1 < len(d.Hops) {
			target = d.Hops[i+1]
		}
		if err := socks5Connect(ctx, conn, target); err != nil {
			return nil, fmt.Errorf("SOCKS5 hop %d CONNECT to %s: %w", i+1, target, err)
		}
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
	watchActive = false
	if err := conn.SetDeadline(time.Time{}); err != nil {
		return nil, fmt.Errorf("clear SOCKS5 chain deadline: %w", err)
	}
	keep = true
	return conn, nil
}

func validateSOCKS5ProxyAddress(address string) error {
	host, portText, err := net.SplitHostPort(strings.TrimSpace(address))
	if err != nil || host == "" {
		return fmt.Errorf("%w: expected host:port", ErrInvalidSOCKS5Proxy)
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("%w: invalid port", ErrInvalidSOCKS5Proxy)
	}
	return nil
}

func socks5Connect(ctx context.Context, conn net.Conn, target string) error {
	request, err := socks5ConnectRequest(target)
	if err != nil {
		return err
	}
	if err := writeAll(conn, []byte{5, 1, 0}); err != nil {
		return contextIOError(ctx, "write SOCKS5 greeting", err)
	}
	var methodReply [2]byte
	if _, err := readFullContext(ctx, conn, methodReply[:], "read SOCKS5 method reply"); err != nil {
		return err
	}
	if methodReply != [2]byte{5, 0} {
		return fmt.Errorf("proxy rejected no-auth method (reply %v)", methodReply)
	}
	if err := writeAll(conn, request); err != nil {
		return contextIOError(ctx, "write SOCKS5 CONNECT request", err)
	}
	return readSOCKS5ConnectReply(ctx, conn)
}

func readFullContext(ctx context.Context, conn net.Conn, dst []byte, operation string) ([]byte, error) {
	if _, err := io.ReadFull(conn, dst); err != nil {
		return nil, contextIOError(ctx, operation, err)
	}
	return dst, nil
}
