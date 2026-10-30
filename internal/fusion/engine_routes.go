package fusion

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// SOCKS5Path describes a path exposed by a separately managed engine. The
// engine must already be running and listening on ProxyAddress. This connects
// PG Core's flow router to real engine data paths for TCP connections; it does
// not create a TUN device or install system-wide routes.
type SOCKS5Path struct {
	Path
	ProxyAddress string
}

// RoutesFromSOCKS5 binds each path snapshot to the corresponding engine's
// existing local SOCKS5 listener. Call NewRouter with the returned routes, then
// use Router.DialContext for outbound TCP flows that should use the selected
// engine. Listener ownership and engine start/stop remain with the host.
func RoutesFromSOCKS5(paths []SOCKS5Path) ([]Route, error) {
	if len(paths) == 0 {
		return nil, ErrInvalidPaths
	}
	routes := make([]Route, 0, len(paths))
	for _, path := range paths {
		if strings.TrimSpace(path.ID) == "" {
			return nil, fmt.Errorf("%w: path ID is required", ErrInvalidPaths)
		}
		address := strings.TrimSpace(path.ProxyAddress)
		host, portText, err := net.SplitHostPort(address)
		if err != nil || host == "" || strings.TrimSpace(portText) == "" {
			return nil, fmt.Errorf("%w: path %q has an invalid SOCKS5 address", ErrInvalidPaths, path.ID)
		}
		// Engine control listeners must remain local. Requiring a literal
		// loopback IP avoids accidentally routing traffic through a remote
		// proxy or resolving a hostname to an unexpected address.
		ip := net.ParseIP(host)
		if ip == nil || !ip.IsLoopback() {
			return nil, fmt.Errorf("%w: path %q SOCKS5 listener must use a loopback IP", ErrInvalidPaths, path.ID)
		}
		port, err := strconv.Atoi(portText)
		if err != nil || port < 1 || port > 65535 {
			return nil, fmt.Errorf("%w: path %q has an invalid SOCKS5 port", ErrInvalidPaths, path.ID)
		}
		dialer := SOCKS5Dialer{ProxyAddress: net.JoinHostPort(ip.String(), strconv.Itoa(port))}
		routes = append(routes, Route{Path: path.Path, Dialer: dialer})
	}
	if _, _, err := normalizeRoutes(routes); err != nil {
		return nil, err
	}
	if _, err := NewAllocator(pathsToSnapshots(routes)); err != nil {
		return nil, err
	}
	return routes, nil
}

func pathsToSnapshots(routes []Route) []Path {
	paths := make([]Path, 0, len(routes))
	for _, route := range routes {
		paths = append(paths, route.Path)
	}
	return paths
}

// ProbeSOCKS5Path measures whether a route can open a TCP connection through
// its engine to the explicitly selected probe target. The target should be a
// controlled or trusted endpoint. This is stronger than checking process
// liveness, but only proves the target and protocol exercised by this probe.
func ProbeSOCKS5Path(ctx context.Context, route Route, target string) (time.Duration, error) {
	if ctx == nil {
		return 0, errors.New("context is required")
	}
	if route.Dialer == nil {
		return 0, errors.New("route dialer is required")
	}
	if strings.TrimSpace(target) == "" {
		return 0, errors.New("probe target is required")
	}
	started := time.Now()
	conn, err := route.Dialer.DialContext(ctx, "tcp", target)
	if err != nil {
		return 0, err
	}
	if conn == nil {
		return 0, errors.New("route dialer returned a nil connection without an error")
	}
	if err := conn.Close(); err != nil {
		return time.Since(started), fmt.Errorf("close route probe connection: %w", err)
	}
	return time.Since(started), nil
}


// SOCKS5ChainPath binds a measured route to an ordered chain of existing
// SOCKS5 listeners. The first hop is dialed directly; later hops are reached
// through the preceding SOCKS5 CONNECT tunnel.
type SOCKS5ChainPath struct {
	Path
	Hops []string
	// Control protects the first-hop socket before connect (VpnService.protect on Android).
	// A nil callback is suitable only outside Android VPN routing.
	Control func(network, address string, raw syscall.RawConn) error
}

// RoutesFromSOCKS5Chains creates route dialers for multi-hop SOCKS5 paths.
// Each hop must already be running and support no-auth TCP CONNECT. The caller
// retains ownership of all proxy engines and listeners.
func RoutesFromSOCKS5Chains(paths []SOCKS5ChainPath) ([]Route, error) {
	if len(paths) == 0 {
		return nil, ErrInvalidPaths
	}
	routes := make([]Route, 0, len(paths))
	for _, path := range paths {
		if strings.TrimSpace(path.ID) == "" {
			return nil, fmt.Errorf("%w: path ID is required", ErrInvalidPaths)
		}
		if len(path.Hops) == 0 {
			return nil, fmt.Errorf("%w: path %q requires at least one SOCKS5 hop", ErrInvalidPaths, path.ID)
		}
		hops := make([]string, len(path.Hops))
		for i, hop := range path.Hops {
			hops[i] = strings.TrimSpace(hop)
			if err := validateSOCKS5ProxyAddress(hops[i]); err != nil {
				return nil, fmt.Errorf("%w: path %q hop %d: %v", ErrInvalidPaths, path.ID, i+1, err)
			}
		}
		routes = append(routes, Route{
			Path: path.Path,
			Dialer: SOCKS5ChainDialer{Hops: hops, Control: path.Control},
		})
	}
	if _, _, err := normalizeRoutes(routes); err != nil {
		return nil, err
	}
	if _, err := NewAllocator(pathsToSnapshots(routes)); err != nil {
		return nil, err
	}
	return routes, nil
}
