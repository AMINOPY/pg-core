package networkpolicy

import (
	"testing"
	"time"
)

func FuzzPolicyValidation(f *testing.F) {
	f.Add("tunnel", "tunnel", "dual", true, 1500, int64(10*time.Second), int64(30*time.Second))
	f.Add("direct", "system", "ipv4", false, 1280, int64(time.Second), int64(time.Minute))
	f.Add("direct", "tunnel", "ipv6", true, 0, int64(0), int64(0))
	f.Fuzz(func(t *testing.T, routing, dns, family string, failClosed bool, mtu int, connectNanos, healthNanos int64) {
		p := Policy{
			RoutingMode: RoutingMode(routing),
			DNSMode: DNSMode(dns),
			IPFamily: IPFamily(family),
			FailClosed: failClosed,
			TUNMTU: mtu,
			ConnectTimeout: time.Duration(connectNanos),
			HealthCheckInterval: time.Duration(healthNanos),
		}
		err := p.Validate()
		if err == nil {
			if p.RoutingMode != RoutingDirect && p.RoutingMode != RoutingTunnel {
				t.Fatal("accepted unsupported routing mode")
			}
			if p.DNSMode != DNSSystem && p.DNSMode != DNSTunnel {
				t.Fatal("accepted unsupported DNS mode")
			}
			if p.DNSMode == DNSTunnel && p.RoutingMode != RoutingTunnel {
				t.Fatal("accepted tunnel DNS without tunnel routing")
			}
			if p.FailClosed && p.RoutingMode != RoutingTunnel {
				t.Fatal("accepted fail-closed without tunnel routing")
			}
			if p.TUNMTU < 576 || p.TUNMTU > 9000 {
				t.Fatal("accepted out-of-range MTU")
			}
			if p.ConnectTimeout < time.Second || p.ConnectTimeout > 2*time.Minute {
				t.Fatal("accepted out-of-range connect timeout")
			}
			if p.HealthCheckInterval < time.Second || p.HealthCheckInterval > 10*time.Minute {
				t.Fatal("accepted out-of-range health interval")
			}
		}
	})
}
