# Network diagnostics

The diagnostics package adds bounded domain checks to the core:

- hostname syntax validation (URL schemes, paths, and ports are rejected)
- DNS resolution and measured lookup latency
- HTTPS HEAD reachability, elapsed time, and HTTP status
- batch execution with input-order results and a worker cap of 32
- per-check deadlines and redirect limits

The API checks only hostnames explicitly supplied by the caller. It does not scan IP ranges, enumerate hosts, generate CDN configurations, modify proxy credentials, or imitate third-party app traffic. A failed check is a point-in-time observation, not proof that a domain or provider is universally blocked.

Example:

```go
checker := diagnostics.Checker{Timeout: 4 * time.Second, Workers: 4}
results := checker.CheckMany(ctx, []string{"example.com", "example.org"})
```

DNS uses the system resolver. HTTPS checks use the standard library HTTP client and verify TLS certificates by default. Custom clients are injectable for controlled tests.
