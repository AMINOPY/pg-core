# CDN connection modes

The core now defines three validated connection modes in `internal/transportprofile`:

- `auto`: configuration for a caller-managed fallback strategy; this package validates the inputs but does not itself perform automatic fallback.
- `direct`: the HTTPS endpoint hostname and TLS `server_name` must match; CDN IP overrides are rejected.
- `cdn_fronting`: requires an explicit IP list and permits a separate TLS SNI name from the HTTPS endpoint's HTTP authority. Use only a CDN and origin you control or are authorized to use, and only where the provider permits this routing pattern.

The connection configuration supports a bounded `cdn_ips` list (up to 32 literal IPv4/IPv6 addresses) and a configurable timeout (250–30,000 ms). `ProbeCDNIPs` checks only those listed IPs, with certificate-verified TLS using `server_name`, and a maximum of eight concurrent probes. It does not scan ranges or discover IPs.

A successful IP probe means only that TCP and a verified TLS handshake succeeded for the configured SNI. It does not prove that the HTTP authority will be routed to the desired origin, that a WebSocket/gRPC tunnel will work, or that end-to-end traffic is flowing. CDN behavior is provider-specific; SNI and an IP list alone do not create a tunnel.

This package is configuration and diagnostics groundwork, not a traffic dialer. The existing engine adapter still needs to consume these options, perform the selected transport, and integrate with OS routing/TUN/DNS before the feature can be considered an operational VPN connection.
