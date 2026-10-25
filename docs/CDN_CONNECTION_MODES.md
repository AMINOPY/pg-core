# CDN connection modes

The `internal/transportprofile` package validates explicit connection modes and produces a deterministic candidate plan. It does not silently discover arbitrary IP ranges or bypass CDN provider policy.

## Modes

- `direct`: connect to the configured HTTPS hostname; `server_name` must match the endpoint hostname and no CDN IP override is accepted.
- `auto`: try the direct hostname first, followed by the explicitly supplied CDN IP list.
- `cdn_fronting`: use only explicitly supplied edge IPs, with the configured TLS `server_name` and HTTP host set independently. The operator must control or be authorized to use the edge/origin mapping.
- `auto-ir` and `auto-us`: regional candidate-list modes. The caller supplies the region-specific IP list; the core then builds a direct-first plan and can rank verified TLS probes. The names do not mean that the library contains a live region database or can infer geography from an IP address.

The `CDNIPs` list is capped at 32 entries. TLS probes are bounded to eight concurrent workers and keep certificate verification enabled. A successful TLS handshake is only a reachability signal; it does not prove that HTTP routing, the origin mapping, the tunnel protocol, DNS privacy, or the system TUN works.

## Candidate ranking

`ProbeCDNIPs` returns measurements for the supplied IP list. `RankProbeResults` puts TLS-reachable candidates first, then sorts by observed handshake latency, preserving input order for ties. Callers must still perform a real data-path check before marking a route healthy and must avoid claiming that a one-time probe guarantees future performance.

## Explicit limitations

- No built-in SNI/CDN IP harvesting or unbounded scan is performed.
- Regional candidate lists must be provided by a trusted source and refreshed by the host application.
- These modes are validation/planning/probing contracts; the native tunnel and subprocess adapters do not yet automatically consume the plan.
- Certificate validation is not disabled to make an edge IP appear healthy.
- The feature does not impersonate unrelated third-party services or guarantee bypass of any filtering system.
