package fusion

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
	"testing"
	"time"
)

func monitorRoutes(t *testing.T) (*Router, []Route) {
	t.Helper()
	dialer := testDialer{dial: func(context.Context, string, string) (net.Conn, error) {
		left, right := net.Pipe()
		_ = right.Close()
		return left, nil
	}}
	routes := []Route{
		{Path: Path{ID: "fast", Healthy: false, Weight: 1}, Dialer: dialer},
		{Path: Path{ID: "slow", Healthy: false, Weight: 1}, Dialer: dialer},
	}
	router, err := NewRouter(routes)
	if err != nil {
		t.Fatal(err)
	}
	return router, routes
}

func TestRefreshRoutesUpdatesHealthAndRTT(t *testing.T) {
	router, routes := monitorRoutes(t)
	failures, err := RefreshRoutes(context.Background(), router, routes, func(_ context.Context, route Route) (time.Duration, error) {
		if route.Path.ID == "fast" {
			return 12 * time.Millisecond, nil
		}
		return 0, errors.New("test endpoint unavailable")
	}, 2, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if len(failures) != 1 || failures["slow"] == nil {
		t.Fatalf("failures = %v, want only slow route failure", failures)
	}
	snapshot := router.Snapshot()
	if len(snapshot) != 2 {
		t.Fatalf("snapshot has %d routes, want 2", len(snapshot))
	}
	byID := map[string]Path{}
	for _, path := range snapshot {
		byID[path.ID] = path
	}
	if !byID["fast"].Healthy || byID["fast"].RTT != 12*time.Millisecond {
		t.Fatalf("fast route not refreshed correctly: %+v", byID["fast"])
	}
	if byID["slow"].Healthy {
		t.Fatalf("failed route must be unhealthy: %+v", byID["slow"])
	}
}

func TestRefreshRoutesHonorsConcurrencyLimit(t *testing.T) {
	router, routes := monitorRoutes(t)
	var active, peak atomic.Int32
	_, err := RefreshRoutes(context.Background(), router, routes, func(ctx context.Context, _ Route) (time.Duration, error) {
		now := active.Add(1)
		for {
			old := peak.Load()
			if now <= old || peak.CompareAndSwap(old, now) {
				break
			}
		}
		defer active.Add(-1)
		select {
		case <-time.After(10 * time.Millisecond):
			return time.Millisecond, nil
		case <-ctx.Done():
			return 0, ctx.Err()
		}
	}, 1, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if got := peak.Load(); got != 1 {
		t.Fatalf("peak concurrent probes = %d, want 1", got)
	}
}

func TestRefreshRoutesCancellationLeavesSnapshotUnchanged(t *testing.T) {
	router, routes := monitorRoutes(t)
	before := router.Snapshot()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := RefreshRoutes(ctx, router, routes, func(context.Context, Route) (time.Duration, error) {
		t.Fatal("probe must not run after parent context is cancelled")
		return 0, nil
	}, 1, time.Second)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	after := router.Snapshot()
	if len(after) != len(before) || after[0].Healthy != before[0].Healthy || after[1].Healthy != before[1].Healthy {
		t.Fatalf("router snapshot changed on cancellation: before=%+v after=%+v", before, after)
	}
}

func TestRefreshRoutesRejectsUnboundedInput(t *testing.T) {
	router, routes := monitorRoutes(t)
	probe := func(context.Context, Route) (time.Duration, error) { return time.Millisecond, nil }
	if _, err := RefreshRoutes(context.Background(), router, routes, probe, maxProbeWorkers+1, time.Second); err == nil {
		t.Fatal("expected excessive concurrency to be rejected")
	}
	if _, err := RefreshRoutes(context.Background(), router, routes, probe, 1, maxProbeTimeout+time.Second); err == nil {
		t.Fatal("expected excessive timeout to be rejected")
	}
}
