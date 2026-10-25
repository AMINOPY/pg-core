package fusion

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
)

// ContextDialer is implemented by net.Dialer and path-specific proxy/tunnel
// dialers that can open an outbound connection.
type ContextDialer interface {
	DialContext(context.Context, string, string) (net.Conn, error)
}

// Route binds a measured path snapshot to the dialer that actually uses it.
type Route struct {
	Path   Path
	Dialer ContextDialer
}

// Router allocates each new outbound connection to one eligible route. It does
// not merge bytes across routes; each connection uses exactly one dialer.
type Router struct {
	mu        sync.Mutex
	revision  uint64
	allocator *Allocator
	dialers   map[string]ContextDialer
}

// NewRouter creates a router from path snapshots and their dialers.
func NewRouter(routes []Route) (*Router, error) {
	r := &Router{dialers: make(map[string]ContextDialer)}
	paths, dialers, err := normalizeRoutes(routes)
	if err != nil {
		return nil, err
	}
	allocator, err := NewAllocator(paths)
	if err != nil {
		return nil, err
	}
	r.allocator = allocator
	r.dialers = dialers
	return r, nil
}

// Update replaces path snapshots and dialers atomically from the router caller's
// perspective. Paths with active connection leases cannot be removed.
func (r *Router) Update(routes []Route) error {
	return r.updateRoutes(routes, nil)
}

// updateIfRevision prevents a slower probe cycle from overwriting a newer
// route snapshot supplied by another caller.
func (r *Router) updateIfRevision(routes []Route, expected uint64) error {
	return r.updateRoutes(routes, &expected)
}

func (r *Router) updateRoutes(routes []Route, expected *uint64) error {
	if r == nil {
		return errors.New("fusion router is required")
	}
	paths, dialers, err := normalizeRoutes(routes)
	if err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if expected != nil && r.revision != *expected {
		return ErrConcurrentRouteUpdate
	}
	if err := r.allocator.Update(paths); err != nil {
		return err
	}
	r.dialers = dialers
	r.revision++
	return nil
}

// DialContext selects a path, reserves its capacity, and opens the connection.
// A lease is released on dial failure or when the returned connection is closed.
func (r *Router) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	if r == nil {
		return nil, errors.New("fusion router is required")
	}
	if ctx == nil {
		return nil, errors.New("context is required")
	}

	r.mu.Lock()
	path, release, err := r.allocator.Acquire()
	if err != nil {
		r.mu.Unlock()
		return nil, err
	}
	dialer := r.dialers[path.ID]
	r.mu.Unlock()

	if dialer == nil {
		release()
		return nil, fmt.Errorf("%w: no dialer registered for path %q", ErrInvalidPaths, path.ID)
	}
	conn, err := dialer.DialContext(ctx, network, address)
	if err != nil {
		release()
		return nil, err
	}
	if conn == nil {
		release()
		return nil, errors.New("path dialer returned a nil connection without an error")
	}
	return &leasedConn{Conn: conn, release: release}, nil
}

// Snapshot returns current path metrics and allocator-owned active connection
// counts. Returned slices can be safely modified by the caller.
func (r *Router) Snapshot() []Path {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.allocator.Snapshot()
}

func normalizeRoutes(routes []Route) ([]Path, map[string]ContextDialer, error) {
	paths := make([]Path, 0, len(routes))
	dialers := make(map[string]ContextDialer, len(routes))
	for _, route := range routes {
		if route.Path.ID == "" || route.Dialer == nil {
			return nil, nil, fmt.Errorf("%w: each route requires a path ID and dialer", ErrInvalidPaths)
		}
		if _, exists := dialers[route.Path.ID]; exists {
			return nil, nil, fmt.Errorf("%w: duplicate route ID %q", ErrInvalidPaths, route.Path.ID)
		}
		dialers[route.Path.ID] = route.Dialer
		paths = append(paths, route.Path)
	}
	return paths, dialers, nil
}

type leasedConn struct {
	net.Conn
	release   func()
	closeOnce sync.Once
	closeErr  error
}

func (c *leasedConn) Close() error {
	c.closeOnce.Do(func() {
		c.closeErr = c.Conn.Close()
		c.release()
	})
	return c.closeErr
}
