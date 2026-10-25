package fusion

import (
	"context"
	"io"
	"testing"
	"time"
)

// This exercises the policy router and its SOCKS5 transport together. The
// local test proxies stand in for separately managed engine SOCKS listeners;
// they do not emulate an OS VPN or prove Internet reachability.
func TestRouterUsesSOCKS5PathAndUpdatesAfterPathFailure(t *testing.T) {
	fastProxy, closeFast := startTestSOCKS5Proxy(t, 0, 0, []byte("fast"), nil)
	defer closeFast()
	slowProxy, closeSlow := startTestSOCKS5Proxy(t, 0, 0, []byte("slow"), nil)
	defer closeSlow()

	router, err := NewRouter([]Route{
		{
			Path: Path{
				ID: "fast", Healthy: true, RTT: 10 * time.Millisecond,
				Weight: 1, MaxConcurrent: 2,
			},
			Dialer: SOCKS5Dialer{ProxyAddress: fastProxy},
		},
		{
			Path: Path{
				ID: "slow", Healthy: true, RTT: 50 * time.Millisecond,
				Weight: 1, MaxConcurrent: 2,
			},
			Dialer: SOCKS5Dialer{ProxyAddress: slowProxy},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	readPayload := func(want string) {
		t.Helper()
		conn, err := router.DialContext(context.Background(), "tcp", "example.com:443")
		if err != nil {
			t.Fatalf("router dial: %v", err)
		}
		defer conn.Close()
		got := make([]byte, len(want))
		if _, err := io.ReadFull(conn, got); err != nil {
			t.Fatalf("read proxy payload: %v", err)
		}
		if string(got) != want {
			t.Fatalf("selected route payload = %q, want %q", got, want)
		}
	}

	readPayload("fast")
	if got := activeFlowsByID(router.Snapshot(), "fast"); got != 0 {
		t.Fatalf("fast path active flows after close = %d, want 0", got)
	}

	if err := router.Update([]Route{
		{
			Path:   Path{ID: "fast", Healthy: false, RTT: 10 * time.Millisecond, Weight: 1, MaxConcurrent: 2},
			Dialer: SOCKS5Dialer{ProxyAddress: fastProxy},
		},
		{
			Path:   Path{ID: "slow", Healthy: true, RTT: 50 * time.Millisecond, Weight: 1, MaxConcurrent: 2},
			Dialer: SOCKS5Dialer{ProxyAddress: slowProxy},
		},
	}); err != nil {
		t.Fatalf("update route health: %v", err)
	}
	readPayload("slow")
}

func activeFlowsByID(paths []Path, id string) int {
	for _, path := range paths {
		if path.ID == id {
			return path.ActiveFlows
		}
	}
	return -1
}
