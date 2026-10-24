# Auto CDN candidate catalog

Auto mode reads an operator-supplied JSON catalog selected by
`CHIMERA_AUTO_CANDIDATES`. The format is:

```json
{
  "sni": ["sni.example.com"],
  "cdn_ips": ["203.0.113.10"]
}
```

The example address above is documentation-only and will not be accepted as a
usable route. Supply only domains and CDN edge IPs that you own, administer, or
are authorized to test. Network Checker is a Flutter desktop/mobile application,
not a Go library or a documented command-line/API service; use its Domain
Checker and Edge IP Checker/CDN Xray Scanner results as an operator-controlled
input rather than copying its GPL-3.0 application code into this repository.

The catalog accepts at most 1,000 SNI hostnames and 1,000 public CDN IPs. Auto
performs at most 10,000 bounded TLS probes across candidate pairs, verifies TLS
certificates normally, and chooses the lowest-latency successful pair. It does
not call a route “clean” permanently: results are network- and time-specific.
After selection, Chimera must still run its tunnel-specific health check before
reporting Connected. TLS handshake success alone does not prove the CDN is
configured to forward traffic to the intended origin.

Set `CHIMERA_AUTO_CANDIDATES=/path/to/auto-candidates.json` before starting
quick connect. An absent or invalid catalog must be reported as not configured;
the app must not invent candidate hosts or scan arbitrary public ranges.
