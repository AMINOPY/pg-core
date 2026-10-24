package networkpolicy

import (
	"errors"
	"testing"
	"time"
)

func TestDefaultPolicyIsValid(t *testing.T) {
	if err := Default().Validate(); err != nil {
		t.Fatalf("Default().Validate() error = %v", err)
	}
}

func TestRejectsDNSLeakProneCombination(t *testing.T) {
	p := Default()
	p.RoutingMode = RoutingDirect
	if err := p.Validate(); !errors.Is(err, ErrInvalidPolicy) {
		t.Fatalf("Validate() error = %v, want ErrInvalidPolicy", err)
	}
}

func TestRejectsFailClosedWithoutTunnel(t *testing.T) {
	p := Default()
	p.RoutingMode = RoutingDirect
	p.DNSMode = DNSSystem
	if err := p.Validate(); !errors.Is(err, ErrInvalidPolicy) {
		t.Fatalf("Validate() error = %v, want ErrInvalidPolicy", err)
	}
}

func TestRejectsInvalidBounds(t *testing.T) {
	tests := []struct {
		name string
		edit func(*Policy)
	}{
		{"mtu below minimum", func(p *Policy) { p.TUNMTU = 575 }},
		{"mtu above maximum", func(p *Policy) { p.TUNMTU = 9001 }},
		{"zero connect timeout", func(p *Policy) { p.ConnectTimeout = 0 }},
		{"too long health interval", func(p *Policy) { p.HealthCheckInterval = 10*time.Minute + time.Nanosecond }},
		{"unknown IP family", func(p *Policy) { p.IPFamily = "automatic-magic" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := Default()
			tt.edit(&p)
			if err := p.Validate(); !errors.Is(err, ErrInvalidPolicy) {
				t.Fatalf("Validate() error = %v, want ErrInvalidPolicy", err)
			}
		})
	}
}

func TestDirectModeAllowsSystemDNSAndNoFailClosed(t *testing.T) {
	p := Default()
	p.RoutingMode = RoutingDirect
	p.DNSMode = DNSSystem
	p.FailClosed = false
	if err := p.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}
