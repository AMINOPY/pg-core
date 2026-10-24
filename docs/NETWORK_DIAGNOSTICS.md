# Network diagnostics

The diagnostics package provides bounded checks against explicitly supplied hosts and endpoints:

- hostname syntax validation (the DNS/HTTPS checker rejects URL schemes, paths, and ports)
- DNS resolution and measured lookup latency
- HTTPS HEAD reachability, elapsed time, and HTTP status
- batch execution with input-order results and a worker cap of 32
- a bounded HTTPS transfer sample with DNS, TCP connect, TLS handshake, time-to-first-byte, total latency, bytes read, and observed payload throughput
- per-check deadlines and redirect limits

The APIs check only targets explicitly supplied by the caller. They do not scan IP ranges, enumerate hosts, generate CDN configurations, modify proxy credentials, or imitate third-party app traffic. A failed check is a point-in-time observation, not proof that a domain or provider is universally blocked.

## Host reachability example

```go
checker := diagnostics.Checker{Timeout: 4 * time.Second, Workers: 4}
results := checker.CheckMany(ctx, []string{"example.com", "example.org"})
```

## Bounded HTTPS transfer sample

```go
benchmarker := diagnostics.Benchmarker{
    Timeout:  8 * time.Second,
    MaxBytes: 2 << 20, // read at most 2 MiB
}
sample := benchmarker.Benchmark(ctx, "https://your-controlled-test-endpoint.example/test.bin")
```

The benchmark accepts an explicit HTTPS URL, rejects userinfo and fragments, does not follow redirects, uses normal certificate verification by default, caps the read to 8 MiB maximum, and applies a deadline capped at one minute. Injected custom HTTP clients are intended for controlled tests; callers must not disable TLS verification in production.

Throughput is the amount of response payload read divided by elapsed request time. It is not a definitive ISP line-speed test, and a single sample must not trigger automatic network changes. Compare multiple samples against endpoints you control and record server/network conditions. This diagnostic measures performance; it does not itself increase bandwidth or bypass DPI.

DNS in the host checker uses the system resolver. HTTPS checks use the standard library HTTP client and verify TLS certificates by default. Custom clients are injectable for controlled tests.
