package transportprofile

import (
	"context"
	"errors"
	"testing"
	"time"
)

func validConnectionConfig() ConnectionConfig {
	return ConnectionConfig{
		Mode:       ModeDirect,
		Endpoint:   "https://tunnel.example.com",
		ServerName: "tunnel.example.com",
	}
}

func TestConnectionModesValidate(t *testing.T) {
	tests := []struct {
		name string
		edit func(*ConnectionConfig)
		want bool
	}{
		{"direct", func(*ConnectionConfig) {}, true},
		{"auto", func(c *ConnectionConfig) { c.Mode = ModeAuto }, true},
		{"auto-ir requires candidates", func(c *ConnectionConfig) { c.Mode = ModeAutoIR }, false},
		{"auto-ir with candidates", func(c *ConnectionConfig) { c.Mode = ModeAutoIR; c.CDNIPs = []string{"192.0.2.10"} }, true},
		{"auto-us with candidates", func(c *ConnectionConfig) { c.Mode = ModeAutoUS; c.CDNIPs = []string{"192.0.2.10"} }, true},
		{"fronting requires IP", func(c *ConnectionConfig) {
			c.Mode = ModeCDNFronting
			c.ServerName = "edge.example.net"
		}, false},
		{"fronting explicit IP", func(c *ConnectionConfig) {
			c.Mode = ModeCDNFronting
			c.ServerName = "edge.example.net"
			c.CDNIPs = []string{"192.0.2.10"}
		}, true},
		{"direct mismatch rejected", func(c *ConnectionConfig) {
			c.ServerName = "edge.example.net"
		}, false},
		{"auto mismatch needs IP", func(c *ConnectionConfig) {
			c.Mode = ModeAuto
			c.ServerName = "edge.example.net"
		}, false},
		{"auto mismatch with IP", func(c *ConnectionConfig) {
			c.Mode = ModeAuto
			c.ServerName = "edge.example.net"
			c.CDNIPs = []string{"192.0.2.10"}
		}, true},
		{"invalid IP", func(c *ConnectionConfig) { c.CDNIPs = []string{"not-an-ip"} }, false},
		{"duplicate IP", func(c *ConnectionConfig) { c.CDNIPs = []string{"192.0.2.1", "192.0.2.1"} }, false},
		{"unknown mode", func(c *ConnectionConfig) { c.Mode = "magic" }, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConnectionConfig()
			tt.edit(&cfg)
			err := cfg.Validate()
			if tt.want && err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
			if !tt.want && !errors.Is(err, ErrInvalidConnectionConfig) {
				t.Fatalf("Validate() error = %v, want ErrInvalidConnectionConfig", err)
			}
		})
	}
}

func TestConnectionModesRejectNonPublicCDNIPs(t *testing.T) {
	for _, ip := range []string{"127.0.0.1", "10.0.0.1", "169.254.1.1", "224.0.0.1", "::", "fc00::1", "fe80::1"} {
		t.Run(ip, func(t *testing.T) {
			cfg := validConnectionConfig()
			cfg.Mode = ModeCDNFronting
			cfg.ServerName = "edge.example.net"
			cfg.CDNIPs = []string{ip}
			if err := cfg.Validate(); !errors.Is(err, ErrInvalidConnectionConfig) {
				t.Fatalf("Validate() error = %v, want invalid config for %q", err, ip)
			}
		})
	}
}

func TestRejectsUnsafeEndpointAndProbeBounds(t *testing.T) {
	cfg := validConnectionConfig()
	cfg.Endpoint = "http://tunnel.example.com"
	if !errors.Is(cfg.Validate(), ErrInvalidConnectionConfig) {
		t.Fatal("expected non-HTTPS endpoint to be rejected")
	}

	cfg = validConnectionConfig()
	cfg.Mode = ModeCDNFronting
	cfg.ServerName = "edge.example.net"
	cfg.CDNIPs = make([]string, MaxCDNIPs+1)
	for i := range cfg.CDNIPs {
		cfg.CDNIPs[i] = "192.0.2.1"
	}
	if !errors.Is(cfg.Validate(), ErrInvalidConnectionConfig) {
		t.Fatal("expected oversized IP list to be rejected")
	}
	if got := ProbeCDNIPs(context.Background(), cfg); got != nil {
		t.Fatalf("invalid config must not probe, got %d results", len(got))
	}
}

func TestProbeCDNIPsDoesNotProbeInvalidConfig(t *testing.T) {
	cfg := validConnectionConfig()
	cfg.Mode = ModeCDNFronting
	cfg.ServerName = "edge.example.net"
	cfg.CDNIPs = []string{"192.0.2.1"}
	// Documentation-only TEST-NET address: this asserts result shape and bounded
	// completion, not a real network connectivity guarantee.
	got := ProbeCDNIPs(context.Background(), cfg)
	if len(got) != 1 {
		t.Fatalf("got %d results, want 1", len(got))
	}
	if got[0].IP != "192.0.2.1" {
		t.Fatalf("got IP %q", got[0].IP)
	}
}

func TestPlanConnectionsModesAndOrdering(t *testing.T) {
	cfg := validConnectionConfig()
	plan, err := PlanConnections(cfg)
	if err != nil || len(plan) != 1 || plan[0].Mode != ModeDirect ||
		plan[0].Address != "tunnel.example.com" || plan[0].ServerName != "tunnel.example.com" {
		t.Fatalf("direct plan = %#v, err = %v", plan, err)
	}

	cfg.Mode = ModeAuto
	cfg.ServerName = "edge.example.net"
	cfg.CDNIPs = []string{"192.0.2.10", "2001:db8::10"}
	plan, err = PlanConnections(cfg)
	if err != nil || len(plan) != 3 {
		t.Fatalf("auto plan = %#v, err = %v", plan, err)
	}
	if plan[0].Mode != ModeDirect || plan[1].Mode != ModeCDNFronting ||
		plan[1].Address != "192.0.2.10" || plan[1].ServerName != "edge.example.net" ||
		plan[1].HTTPHost != "tunnel.example.com" || plan[2].Address != "2001:db8::10" {
		t.Fatalf("unexpected auto candidate order/content: %#v", plan)
	}

	cfg.Mode = ModeCDNFronting
	plan, err = PlanConnections(cfg)
	if err != nil || len(plan) != 2 || plan[0].Mode != ModeCDNFronting {
		t.Fatalf("fronting plan = %#v, err = %v", plan, err)
	}
}

func TestRegionalModesPlanDeterministically(t *testing.T) {
	for _, mode := range []ConnectionMode{ModeAutoIR, ModeAutoUS} {
		cfg := validConnectionConfig()
		cfg.Mode = mode
		cfg.ServerName = "edge.example.net"
		cfg.CDNIPs = []string{"192.0.2.10", "192.0.2.11"}
		plan, err := PlanConnections(cfg)
		if err != nil {
			t.Fatalf("%s PlanConnections() error = %v", mode, err)
		}
		if len(plan) != 3 || plan[0].Mode != ModeDirect || plan[1].Address != "192.0.2.10" || plan[2].Address != "192.0.2.11" {
			t.Fatalf("%s plan = %#v", mode, plan)
		}
	}
}

func TestRankProbeResultsHealthyFastFirstAndStable(t *testing.T) {
	input := []ProbeResult{
		{IP: "192.0.2.1", Reachable: true, Latency: 50 * time.Millisecond},
		{IP: "192.0.2.2", Reachable: false, Latency: time.Millisecond},
		{IP: "192.0.2.3", Reachable: true, Latency: 10 * time.Millisecond},
		{IP: "192.0.2.4", Reachable: true, Latency: 10 * time.Millisecond},
	}
	got := RankProbeResults(input)
	want := []string{"192.0.2.3", "192.0.2.4", "192.0.2.1", "192.0.2.2"}
	for i := range want {
		if got[i].IP != want[i] {
			t.Fatalf("ranked[%d].IP = %q, want %q", i, got[i].IP, want[i])
		}
	}
	if input[0].IP != "192.0.2.1" {
		t.Fatal("RankProbeResults mutated input")
	}
}

func TestProbeCDNIPsAcceptsNilContext(t *testing.T) {
	cfg := validConnectionConfig()
	cfg.Mode = ModeCDNFronting
	cfg.ServerName = "edge.example.net"
	cfg.CDNIPs = []string{"192.0.2.1"}
	cfg.TimeoutMS = 250
	got := ProbeCDNIPs(nil, cfg)
	if len(got) != 1 || got[0].IP != "192.0.2.1" {
		t.Fatalf("ProbeCDNIPs(nil) = %#v", got)
	}
}
