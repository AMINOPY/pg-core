// Package networkpolicy defines validated, platform-neutral network safety
// policy. It does not install routes, configure DNS, or create a TUN device.
package networkpolicy

import (
	"errors"
	"fmt"
	"time"
)

type RoutingMode string

const (
	RoutingDirect RoutingMode = "direct"
	RoutingTunnel RoutingMode = "tunnel"
)

type DNSMode string

const (
	DNSSystem DNSMode = "system"
	DNSTunnel DNSMode = "tunnel"
)

type IPFamily string

const (
	IPDualStack IPFamily = "dual"
	IPv4Only    IPFamily = "ipv4"
	IPv6Only    IPFamily = "ipv6"
)

var ErrInvalidPolicy = errors.New("invalid network policy")

// Policy is a platform-neutral declaration consumed by future OS adapters.
// FailClosed means traffic must not fall back to direct routing if tunnel
// enforcement is available; it is not a guarantee until an adapter enforces it.
type Policy struct {
	RoutingMode         RoutingMode   `json:"routing_mode"`
	DNSMode             DNSMode       `json:"dns_mode"`
	IPFamily            IPFamily      `json:"ip_family"`
	FailClosed          bool          `json:"fail_closed"`
	TUNMTU              int           `json:"tun_mtu"`
	ConnectTimeout      time.Duration `json:"connect_timeout"`
	HealthCheckInterval time.Duration `json:"health_check_interval"`
}

// Default returns a conservative tunnel-oriented policy. It is a declaration,
// not an applied OS configuration.
func Default() Policy {
	return Policy{
		RoutingMode:         RoutingTunnel,
		DNSMode:             DNSTunnel,
		IPFamily:            IPDualStack,
		FailClosed:          true,
		TUNMTU:              1500,
		ConnectTimeout:      10 * time.Second,
		HealthCheckInterval: 30 * time.Second,
	}
}

func (p Policy) Validate() error {
	if p.RoutingMode != RoutingDirect && p.RoutingMode != RoutingTunnel {
		return fmt.Errorf("%w: routing_mode %q is unsupported", ErrInvalidPolicy, p.RoutingMode)
	}
	if p.DNSMode != DNSSystem && p.DNSMode != DNSTunnel {
		return fmt.Errorf("%w: dns_mode %q is unsupported", ErrInvalidPolicy, p.DNSMode)
	}
	if p.IPFamily != IPDualStack && p.IPFamily != IPv4Only && p.IPFamily != IPv6Only {
		return fmt.Errorf("%w: ip_family %q is unsupported", ErrInvalidPolicy, p.IPFamily)
	}
	if p.DNSMode == DNSTunnel && p.RoutingMode != RoutingTunnel {
		return fmt.Errorf("%w: tunnel DNS requires tunnel routing", ErrInvalidPolicy)
	}
	if p.FailClosed && p.RoutingMode != RoutingTunnel {
		return fmt.Errorf("%w: fail_closed requires tunnel routing", ErrInvalidPolicy)
	}
	if p.TUNMTU < 576 || p.TUNMTU > 9000 {
		return fmt.Errorf("%w: tun_mtu must be between 576 and 9000", ErrInvalidPolicy)
	}
	if p.ConnectTimeout < time.Second || p.ConnectTimeout > 2*time.Minute {
		return fmt.Errorf("%w: connect_timeout must be between 1s and 2m", ErrInvalidPolicy)
	}
	if p.HealthCheckInterval < time.Second || p.HealthCheckInterval > 10*time.Minute {
		return fmt.Errorf("%w: health_check_interval must be between 1s and 10m", ErrInvalidPolicy)
	}
	return nil
}
