# Ghost network analysis

Ghost is a bounded, evidence-based analysis module for connectivity symptoms that can sometimes accompany network filtering or DPI. It reports observable DNS/HTTPS outcomes and cautious explanations; it does not claim that an individual failure proves DPI.

## CLI

```sh
go run ./cmd/chimera --ghost example.com --samples 5
```

- The hostname is explicitly supplied by the user; Ghost does not enumerate hosts or scan address ranges.
- Samples are capped at 20, with bounded per-check timeouts.
- Reports include DNS/HTTPS success counts, mean successful-check latency, findings, evidence, and recommendations.
- A 403/451, timeout, TLS error, reset, or DNS failure is a symptom, not a definitive DPI signature.
- The analyzer does not intercept traffic, impersonate third-party apps, alter system DNS/routes, or silently switch profiles.

## Recovery workflow

1. Compare the same endpoint with a second known-good host and, when possible, a trusted alternate network.
2. Check system time, captive portal, resolver behavior, and server availability.
3. If the operator has already configured supported alternate transports, test one at a time and verify actual end-to-end reachability before selecting it.
4. Keep the last known working profile available and require explicit user consent before changing network ownership or route/DNS state.

Ghost currently produces diagnosis and recovery recommendations. It is not a DPI detector with ground-truth labels, and does not guarantee circumvention. Production adaptive fallback still needs integration with engine health verification and platform-specific route/TUN ownership.
