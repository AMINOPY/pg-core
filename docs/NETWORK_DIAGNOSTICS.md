# Network diagnostics

The diagnostics package provides bounded checks against explicitly supplied hosts and endpoints:

- hostname syntax validation (the DNS/HTTPS checker rejects URL schemes, paths, and ports)
- DNS resolution and measured lookup latency
- HTTPS HEAD reachability, elapsed time, and HTTP status
- batch execution with input-order results and a worker cap of 32
- bounded HTTPS transfer samples with DNS, TCP connect, TLS handshake, time-to-first-byte, total latency, bytes read, and observed payload throughput
- per-check deadlines and redirect disabling for the transfer benchmark

The API checks only hostnames or HTTPS endpoints explicitly supplied by the caller. It does not scan IP ranges, enumerate hosts, generate CDN configurations, modify proxy credentials, or imitate third-party app traffic. A failed check is a point-in-time observation, not proof that a domain or provider is universally blocked.

Example for hostname checks:

```go
checker := diagnostics.Checker{Timeout: 4 * time.Second, Workers: 4}
results := checker.CheckMany(ctx, []string{"example.com", "example.org"})
```

Example for a bounded transfer sample:

```go
sample := (diagnostics.Benchmarker{
    Timeout:  10 * time.Second,
    MaxBytes: 1 << 20, // at most 1 MiB
}).Benchmark(ctx, "https://your-controlled-endpoint.example/test.bin")
```

The benchmark accepts HTTPS URLs only, uses certificate verification by default, disables redirects, has a maximum 60-second timeout and an 8 MiB transfer cap. The result omits the URL path and query string so signed paths and query tokens are not copied into diagnostic output. Configure the endpoint you control and avoid using arbitrary large downloads.

DNS uses the system resolver. HTTPS checks use the standard library HTTP client and verify TLS certificates by default. Custom clients are injectable for controlled tests. Benchmark results are endpoint-specific observations, not a tunnel health guarantee or a measurement of the ISP's maximum line speed; connection reuse, caching, endpoint capacity, and server load affect results.
