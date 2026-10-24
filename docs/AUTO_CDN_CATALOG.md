# Auto CDN candidate catalog

Auto, CDN IR, and CDN US read separate operator-supplied JSON catalogs selected by
`CHIMERA_AUTO_CANDIDATES`, `CHIMERA_CDN_IR_CANDIDATES`, and
`CHIMERA_CDN_US_CANDIDATES`. Each catalog uses this format:

```json
{
  "sni": ["sni.example.com"],
  "cdn_ips": ["203.0.113.10"]
}
```

Replace the placeholders with domains and CDN edge IPs that you own, administer,
or are authorized to test. Network Checker is a Flutter desktop/mobile application,
not a Go library or a documented command-line/API service; use its Domain
Checker and Edge IP Checker/CDN Xray Scanner results as an operator-controlled
input rather than copying its GPL-3.0 application code into this repository.

Each catalog accepts at most 1,000 SNI hostnames and 1,000 public CDN IPs. Each scan performs at most 1,000 bounded TLS probes across candidate pairs, verifies TLS certificates normally, ranks successful routes by measured latency, and returns up to the top 20 routes. The UI fills its two multi-line fields with up to 20 distinct SNI and CDN IP values derived from those ranked routes. It does not call a route “clean” permanently: results are network- and time-specific. Because route quality depends on an SNI/IP pair, the list of distinct values may contain fewer than 20 items.
After selection, Chimera must still run its tunnel-specific health check before
reporting Connected. TLS handshake success alone does not prove the CDN is
configured to forward traffic to the intended origin.

Set `CHIMERA_AUTO_CANDIDATES=/path/to/auto-candidates.json` before starting
quick connect. For the regional modes, set `CHIMERA_CDN_IR_CANDIDATES` and
`CHIMERA_CDN_US_CANDIDATES` to their own curated catalog files. The IR/US label
means the operator has curated that catalog for the intended region; it is not
proof of an IP's geolocation. Network Checker has no documented CLI/API that
this Go app can call directly, so its results must be exported or copied into
these catalogs by the operator. Do not claim live Network Checker integration.
An absent or invalid catalog must be reported as not configured; the app must
not invent candidate hosts or scan arbitrary public ranges.
