# AI Network Diagnostics

This feature is an explainable diagnostics module in `internal/ai`. It is a CLI report with repeated DNS/HTTPS checks and optional bounded HTTP throughput measurements; it is not a trained ML model, mobile app UI, SIM inspector, or native LibreSpeed/OpenSpeedTest protocol client.

## Run it

```sh
go run ./cmd/chimera --ai-scan example.com
go run ./cmd/chimera --ai-scan example.com --samples 10
# Optional, only with endpoints you trust and have configured:
go run ./cmd/chimera --ai-scan example.com --download-url https://your-server.test/download --upload-url https://your-server.test/upload
```

The command prints a JSON report. Samples are clamped to 1–20 (default 5), each DNS/HTTPS check has a bounded timeout, and the overall CLI operation has a two-minute deadline. Exit status is non-zero when no HTTPS sample succeeds.

## Current measurements

- DNS lookup success/failure
- HTTPS reachability using the Go standard library with TLS certificate validation enabled
- HTTPS request latency min/average/max (this is not pure ICMP ping; it includes server response behavior)
- Failed HTTPS sample percentage
- Jitter estimate: mean absolute change between consecutive successful HTTPS timings
- Optional HTTP download/upload throughput to operator-configured endpoints
- Rule-based findings with severity, explanation, and measured evidence

Findings are deterministic heuristics. No network measurement is invented and no external AI provider is contacted.

## Throughput endpoint contract and limits

The current adapter is a generic HTTP transfer measurement, not the native LibreSpeed or OpenSpeedTest protocol. The download endpoint must return a 2xx response and a body. The upload endpoint must accept a POST with `application/octet-stream` and return 2xx. Default limits are 10 MiB download and 1 MiB upload; hard limits are 50 MiB and 10 MiB. Request timeouts are bounded. Remote endpoints must use HTTPS; HTTP is permitted only for loopback development tests. Redirects are not followed to prevent forwarding upload data to another host. Measurements include endpoint/server overhead and are not a substitute for a standardized speed-test suite.

## Potential native adapters

- [LibreSpeed](https://github.com/librespeed/speedtest): integrate with a trusted, preferably self-hosted backend for standardized download/upload/ping tests.
- [OpenSpeedTest](https://github.com/openspeedtest/Speed-Test): possible native protocol backend.
- [Network Checker](https://github.com/mirarr-app/network-checker): reference for UX and diagnostics; review its license and compatibility before reusing code.

These projects are not bundled and are not automatically contacted by `--ai-scan`. Throughput tests run only when an endpoint URL is supplied. They can consume mobile data; use a trusted server and review the configured transfer limits.

## SIM and carrier details

A generic Go CLI cannot reliably inspect SIM identity or cellular radio metadata across platforms. This command does not claim to detect SIM type, carrier, signal strength, or mobile radio technology. Future native providers must be platform-specific, permission-gated, and return `unknown` when the OS does not expose a value. Subscriber identifiers such as IMSI/ICCID and phone numbers are out of scope.

## Safety and privacy requirements

- Only probes the hostname explicitly supplied by the user; no IP-range scanning.
- No automatic telemetry or result sharing.
- TLS verification remains enabled.
- No OS routes, DNS settings, or VPN engine state are changed.
- Results are point-in-time observations and do not prove that an ISP or service is universally blocked.
- Do not treat a live engine process as proof of end-to-end connectivity.

## Validation

```sh
gofmt -w .
go vet ./...
go test ./...
```
