# AI Network Diagnostics (proposal)

This document defines the first integration boundary for an `AI` diagnostics capability in pg-core. It is a design/specification, not a claim that platform-specific collection or an ML model is already implemented.

## Goals

Provide one opt-in diagnostic report combining:
- Connectivity checks: DNS resolution and certificate-verified HTTPS reachability for explicitly configured targets.
- Latency quality: HTTP request latency, jitter estimate, failed-request ratio, and stability over a bounded test window.
- Throughput: download/upload measurements using a self-hosted test endpoint.
- Connection context: platform-reported connection type when available (Wi-Fi/cellular/unknown), with the source and confidence recorded.
- Recommendations: deterministic, explainable rules first; an optional AI explanation layer may summarize the measurements but must not invent measurements.

## Suggested package boundary

Keep the diagnostics independent of the VPN engine lifecycle:

```text
internal/ai/
  report.go       # report and finding types
  runner.go       # bounded orchestration and cancellation
  connectivity.go # DNS + verified HTTPS probes
  stability.go    # latency samples, jitter, failure ratio
  speedtest.go    # adapter for a configured speed-test endpoint
  recommendations.go # deterministic findings
```

The runner should accept a context, explicit target allowlist, deadlines, and a maximum test-data budget. It must never scan arbitrary IP ranges or silently run continuous tests.

## Speed-test integrations

- [LibreSpeed](https://github.com/librespeed/speedtest): preferred first adapter. Self-host the test backend and use its supported download/upload/ping interfaces. Keep test duration and payload limits configurable; ask the user before upload/download tests that may consume mobile data.
- [OpenSpeedTest](https://github.com/openspeedtest/Speed-Test): optional alternative backend behind the same adapter contract. Verify the upstream project's current license and deployment requirements before distributing or embedding assets.
- [Network Checker](https://github.com/mirarr-app/network-checker): evaluate as a reference for diagnostics and UX; do not copy code until its license and compatibility are verified.

Do not bundle or redistribute upstream assets until their licenses and notices have been reviewed. Prefer configuring a user-controlled/self-hosted endpoint over sending network test traffic to an unknown third party.

## SIM and cellular details

SIM/operator data is platform-specific and is not available to a generic Go process or ordinary web page. Implement separate native providers only where the OS exposes the data and the user grants required permissions. Return `unknown` when unavailable. Do not collect phone numbers, IMSI, ICCID, precise location, or other subscriber identifiers for this feature. If the platform only exposes a carrier label or radio technology, collect only that coarse value with explicit consent.

## Privacy and safety requirements

- Diagnostics are user-triggered and clearly show expected data use before throughput tests.
- No telemetry by default; saving or sharing reports requires a separate opt-in.
- Redact IP addresses and hostnames from logs by default.
- Use HTTPS with certificate verification; never disable TLS verification to make a test pass.
- Bound concurrency, request count, payload size, and total duration.
- Cancel promptly when the user stops a test or the context expires.
- Keep VPN engine start/stop and routing ownership separate from diagnostics.
- Report measured facts separately from inferred recommendations and include uncertainty.

## Initial implementation sequence

1. Add report types and unit-tested deterministic recommendation rules.
2. Add bounded DNS/HTTPS and stability probes using existing diagnostics conventions.
3. Add an opt-in throughput adapter for a configured LibreSpeed endpoint.
4. Add platform-specific connection-context providers behind capability interfaces.
5. Add a UI/API surface named **AI** only after the core API and tests are in place.

## Acceptance criteria

- Tests cover cancellation, timeouts, malformed targets, partial failures, and redaction.
- No external requests occur until a user starts diagnostics.
- Throughput tests require a configured endpoint and an explicit data-use confirmation.
- Unsupported SIM/carrier fields return `unknown`, never fabricated values.
- Recommendations cite the measurements that triggered them.
