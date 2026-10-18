# AI Network Diagnostics

This feature is an initial, explainable diagnostics module in `internal/ai`. It is operational as a CLI diagnostic report; it is not a trained ML model, mobile app UI, SIM inspector, or full throughput test yet.

## Run it

```sh
go run ./cmd/chimera --ai-scan example.com
go run ./cmd/chimera --ai-scan example.com --samples 10
```

The command prints a JSON report. Samples are clamped to 1–20 (default 5), each DNS/HTTPS check has a bounded timeout, and the overall CLI operation has a two-minute deadline. Exit status is non-zero when no HTTPS sample succeeds.

## Current measurements

- DNS lookup success/failure
- HTTPS reachability using the Go standard library with TLS certificate validation enabled
- HTTPS request latency min/average/max (this is not pure ICMP ping; it includes server response behavior)
- Failed HTTPS sample percentage
- Jitter estimate: mean absolute change between consecutive successful HTTPS timings
- Rule-based findings with severity, explanation, and measured evidence

Findings are deterministic heuristics. No network measurement is invented and no external AI provider is contacted.

## Planned adapters

- [LibreSpeed](https://github.com/librespeed/speedtest): integrate with a trusted, preferably self-hosted backend for real download/upload/ping tests.
- [OpenSpeedTest](https://github.com/openspeedtest/Speed-Test): possible alternative throughput backend.
- [Network Checker](https://github.com/mirarr-app/network-checker): reference for UX and diagnostics; review its license and compatibility before reusing code.

These projects are not bundled and are not automatically contacted by `--ai-scan`. A real throughput test can consume substantial mobile data, so any future implementation must require a configured trusted endpoint and explicit user consent.

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
