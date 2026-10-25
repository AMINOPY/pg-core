package fusion

import (
	"context"
	"errors"
	"net"
	"testing"
)

type routeTestDialer struct {
	conn net.Conn
	err  error
	got  string
}

func (d *routeTestDialer) DialContext(_ context.Context, network, address string) (net.Conn, error) {
	d.got = network + "|" + address
	if d.err != nil {
		return nil, d.err
	}
	return d.conn, nil
}

func TestRoutesFromSOCKS5BindsPathsToEngineListeners(t *testing.T) {
	routes, err := RoutesFromSOCKS5([]SOCKS5Path{
		{Path: Path{ID: "singbox", Healthy: true, Weight: 1}, ProxyAddress: "127.0.0.1:1080"},
		{Path: Path{ID: "xray", Healthy: true, Weight: 2}, ProxyAddress: "127.0.0.1:1081"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 2 {
		t.Fatalf("got %d routes, want 2", len(routes))
	}
	router, err := NewRouter(routes)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(router.Snapshot()); got != 2 {
		t.Fatalf("router snapshot has %d paths, want 2", got)
	}
	if _, err := RoutesFromSOCKS5([]SOCKS5Path{{Path: Path{ID: "bad", Weight: 1}, ProxyAddress: "not-an-address"}}); err == nil {
		t.Fatal("expected malformed engine listener to be rejected")
	}
	for _, address := range []string{
		"0.0.0.0:1080",
		"192.0.2.10:1080",
		"localhost:1080",
		"127.0.0.1:abc",
		"127.0.0.1:65536",
	} {
		if _, err := RoutesFromSOCKS5([]SOCKS5Path{{
			Path: Path{ID: "invalid-listener", Weight: 1}, ProxyAddress: address,
		}}); err == nil {
			t.Errorf("expected unsafe or invalid listener %q to be rejected", address)
		}
	}
	if _, err := RoutesFromSOCKS5([]SOCKS5Path{
		{Path: Path{ID: "same", Weight: 1}, ProxyAddress: "127.0.0.1:1080"},
		{Path: Path{ID: "same", Weight: 1}, ProxyAddress: "127.0.0.1:1081"},
	}); err == nil {
		t.Fatal("expected duplicate path ID to be rejected")
	}
}

func TestProbeSOCKS5PathUsesRouteDialerAndClosesConnection(t *testing.T) {
	client, peer := net.Pipe()
	defer peer.Close()
	dialer := &routeTestDialer{conn: client}
	route := Route{Path: Path{ID: "engine-a", Healthy: true, Weight: 1}, Dialer: dialer}
	rtt, err := ProbeSOCKS5Path(context.Background(), route, "example.com:443")
	if err != nil {
		t.Fatal(err)
	}
	if rtt < 0 {
		t.Fatalf("RTT must be non-negative, got %s", rtt)
	}
	if dialer.got != "tcp|example.com:443" {
		t.Fatalf("dial target = %q", dialer.got)
	}
	if _, err := client.Write([]byte("x")); err == nil {
		t.Fatal("probe connection should be closed")
	}
}

func TestProbeSOCKS5PathRejectsInvalidInputsAndSurfacesDialFailure(t *testing.T) {
	if _, err := ProbeSOCKS5Path(nil, Route{}, "example.com:443"); err == nil {
		t.Fatal("expected nil context error")
	}
	if _, err := ProbeSOCKS5Path(context.Background(), Route{}, "example.com:443"); err == nil {
		t.Fatal("expected missing dialer error")
	}
	dialErr := errors.New("proxy listener unavailable")
	dialer := &routeTestDialer{err: dialErr}
	route := Route{Path: Path{ID: "engine-a", Weight: 1}, Dialer: dialer}
	if _, err := ProbeSOCKS5Path(context.Background(), route, "example.com:443"); !errors.Is(err, dialErr) {
		t.Fatalf("probe error = %v, want dial error", err)
	}
	if _, err := ProbeSOCKS5Path(context.Background(), route, " "); err == nil {
		t.Fatal("expected empty target error")
	}
}
