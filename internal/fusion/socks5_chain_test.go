package fusion

import (
	"context"
	"errors"
	"testing"
)

func TestRoutesFromSOCKS5ChainsBuildsOrderedDialer(t *testing.T) {
	routes, err := RoutesFromSOCKS5Chains([]SOCKS5ChainPath{{
		Path: Path{ID: "two-hop", Healthy: true, Weight: 1},
		Hops: []string{"127.0.0.1:1081", "127.0.0.1:1082"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 1 {
		t.Fatalf("route count = %d, want 1", len(routes))
	}
	chain, ok := routes[0].Dialer.(SOCKS5ChainDialer)
	if !ok {
		t.Fatalf("dialer type = %T, want SOCKS5ChainDialer", routes[0].Dialer)
	}
	if len(chain.Hops) != 2 || chain.Hops[0] != "127.0.0.1:1081" || chain.Hops[1] != "127.0.0.1:1082" {
		t.Fatalf("chain hops = %v, want ordered two-hop chain", chain.Hops)
	}
}

func TestRoutesFromSOCKS5ChainsRejectsInvalidConfiguration(t *testing.T) {
	cases := []struct {
		name string
		path SOCKS5ChainPath
	}{
		{name: "missing ID", path: SOCKS5ChainPath{Path: Path{Healthy: true}, Hops: []string{"127.0.0.1:1080"}}},
		{name: "missing hops", path: SOCKS5ChainPath{Path: Path{ID: "empty", Healthy: true}}},
		{name: "invalid hop", path: SOCKS5ChainPath{Path: Path{ID: "bad", Healthy: true}, Hops: []string{"not-a-proxy"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := RoutesFromSOCKS5Chains([]SOCKS5ChainPath{tc.path}); err == nil {
				t.Fatal("expected invalid chain configuration to fail")
			}
		})
	}
}

func TestSOCKS5ChainDialerRejectsMissingHopsAndUDP(t *testing.T) {
	d := SOCKS5ChainDialer{}
	if _, err := d.DialContext(context.Background(), "tcp", "example.com:443"); err == nil {
		t.Fatal("expected missing hops to fail")
	}
	d.Hops = []string{"127.0.0.1:1080"}
	if _, err := d.DialContext(context.Background(), "udp", "example.com:53"); !errors.Is(err, ErrUnsupportedNetwork) {
		t.Fatalf("UDP error = %v, want ErrUnsupportedNetwork", err)
	}
}
