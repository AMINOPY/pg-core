package fusion

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
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
		host, portText, err := net.SplitHostPort(strings.TrimSpace(path.ProxyAddress))
		if err != nil || host == "" || strings.TrimSpace(portText) == "" {
			return nil, fmt.Errorf("%w: path %q has an invalid SOCKS5 address", ErrInvalidPaths, path.ID)
		}
		dialer := SOCKS5Dialer{ProxyAddress: path.ProxyAddress}
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
