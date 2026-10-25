package fusion

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
	"testing"
	"time"
)

type testDialer struct {
	dial func(context.Context, string, string) (net.Conn, error)
}

func (d testDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return d.dial(ctx, network, address)
}

func TestRouterHoldsLeaseUntilConnectionClose(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		conn, err := listener.Accept()
		if err == nil {
			defer conn.Close()
			buf := make([]byte, 1)
			_, _ = conn.Read(buf)
		}
	}()

	router, err := NewRouter([]Route{{
		Path: Path{ID: "local", Healthy: true, Weight: 1, MaxConcurrent: 1},
		Dialer: &net.Dialer{Timeout: time.Second},
	}})
	if err != nil {
		t.Fatal(err)
	}
	conn, err := router.DialContext(context.Background(), "tcp", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	if got := router.Snapshot()[0].ActiveFlows; got != 1 {
		t.Fatalf("active flows while connection open = %d, want 1", got)
	}
	if _, err := router.DialContext(context.Background(), "tcp", listener.Addr().String()); !errors.Is(err, ErrNoEligiblePath) {
		t.Fatalf("second dial error = %v, want ErrNoEligiblePath", err)
	}
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	if err := conn.Close(); err != nil {
		t.Fatalf("second close error = %v", err)
	}
	if got := router.Snapshot()[0].ActiveFlows; got != 0 {
		t.Fatalf("active flows after close = %d, want 0", got)
	}
	<-serverDone
}

func TestRouterReleasesLeaseAfterDialFailure(t *testing.T) {
	router, err := NewRouter([]Route{{
		Path: Path{ID: "fail", Healthy: true, Weight: 1, MaxConcurrent: 1},
		Dialer: testDialer{dial: func(context.Context, string, string) (net.Conn, error) {
			return nil, errors.New("expected dial failure")
		}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := router.DialContext(context.Background(), "tcp", "unused:1"); err == nil {
		t.Fatal("expected dial failure")
	}
	if got := router.Snapshot()[0].ActiveFlows; got != 0 {
		t.Fatalf("active flows after failed dial = %d, want 0", got)
	}
}

func TestRouterCannotRemoveRouteWithOpenConnection(t *testing.T) {
	left, right := net.Pipe()
	defer right.Close()
	router, err := NewRouter([]Route{{
		Path: Path{ID: "pipe", Healthy: true, Weight: 1},
		Dialer: testDialer{dial: func(context.Context, string, string) (net.Conn, error) {
			return left, nil
		}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	conn, err := router.DialContext(context.Background(), "tcp", "ignored:1")
	if err != nil {
		t.Fatal(err)
	}
	if err := router.Update([]Route{{
		Path: Path{ID: "other", Healthy: true, Weight: 1},
		Dialer: testDialer{dial: func(context.Context, string, string) (net.Conn, error) {
			return nil, errors.New("unused")
		}},
	}}); !errors.Is(err, ErrPathInUse) {
		t.Fatalf("update error = %v, want ErrPathInUse", err)
	}
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	if err := router.Update([]Route{{
		Path: Path{ID: "other", Healthy: true, Weight: 1},
		Dialer: testDialer{dial: func(context.Context, string, string) (net.Conn, error) {
			return nil, errors.New("unused")
		}},
	}}); err != nil {
		t.Fatalf("update after close: %v", err)
	}
}

func TestRouterRejectsMissingDialerAndReleasesIfMissing(t *testing.T) {
	if _, err := NewRouter([]Route{{Path: Path{ID: "missing", Healthy: true, Weight: 1}}}); !errors.Is(err, ErrInvalidPaths) {
		t.Fatalf("missing dialer error = %v, want ErrInvalidPaths", err)
	}
}

func TestRouterConcurrentCapacityNeverOverbooks(t *testing.T) {
	const cap = 4
	router, err := NewRouter([]Route{{
		Path: Path{ID: "capacity", Healthy: true, Weight: 1, MaxConcurrent: cap},
		Dialer: testDialer{dial: func(context.Context, string, string) (net.Conn, error) {
			return net.Pipe()
		}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	var success atomic.Int32
	var done = make(chan struct{}, 32)
	for i := 0; i < cap*8; i++ {
		go func() {
			conn, err := router.DialContext(context.Background(), "tcp", "ignored:1")
			if err == nil {
				success.Add(1)
				_ = conn.Close()
			}
			done <- struct{}{}
		}()
	}
	for i := 0; i < cap*8; i++ {
		<-done
	}
	if got := success.Load(); got < cap || got > cap*8 {
		t.Fatalf("successful dials = %d, want at least %d", got, cap)
	}
	if got := router.Snapshot()[0].ActiveFlows; got != 0 {
		t.Fatalf("active flows after all closes = %d, want 0", got)
	}
}
