# Chimera Core (pg-core)

**Status: early scaffold / not production-ready.**

Chimera Core is a modular network-engine coordinator. It includes strict configuration validation, an engine adapter contract, a serialized lifecycle manager, deterministic health-aware candidate selection, bounded network diagnostics, unit tests, and CI. The AI diagnostics CLI is an initial explainable diagnostics feature, not a trained AI model or a production VPN client.

## Current implementation

- Strict JSON parsing with unknown-field and trailing-value rejection
- Config validation for supported engine IDs and active-engine selection
- Common engine lifecycle/health interface and adapter registry
- Start validation, health checks, failure cleanup, and serialized lifecycle operations
- Deterministic route-candidate selection based on health and latency
- Bounded DNS/HTTPS diagnostics for explicitly supplied hostnames
- AI diagnostics report: repeated HTTPS checks, latency min/average/max, failed-sample ratio, jitter estimate, evidence-backed rule findings, and opt-in bounded HTTP throughput measurements
- sing-box adapter: upstream config check, private temporary profile, process start/health/stop
- Unit tests and GitHub Actions workflow

## Quick start

Requires Go 1.22 or newer.

    go test ./...
    go vet ./...
    go run ./cmd/chimera
    go run ./cmd/chimera --config config.example.json

Run one DNS/HTTPS check:

    go run ./cmd/chimera --diagnose example.com

Run the bounded AI diagnostics report (contacts only the hostname you specify):

    go run ./cmd/chimera --ai-scan example.com
    go run ./cmd/chimera --ai-scan example.com --samples 10

The scan accepts 1–20 samples (default 5), uses bounded timeouts, emits JSON, and returns a non-zero exit code if no HTTPS samples succeed. HTTPS request timing is not ICMP ping.

To opt into actual throughput measurements, supply your own trusted test endpoints:

    go run ./cmd/chimera --ai-scan example.com --download-url https://speed.example.test/download --upload-url https://speed.example.test/upload

The download endpoint should return a successful 2xx response with bytes to download. The upload endpoint should accept a POST with `application/octet-stream` and return 2xx. By default, at most 10 MiB is read for download and 1 MiB is sent for upload; hard caps are 50 MiB and 10 MiB. Endpoints must use HTTPS (HTTP is allowed only for loopback development tests), and upload redirects are not followed. These are generic HTTP endpoints, not an automatic LibreSpeed/OpenSpeedTest protocol adapter. Throughput tests can consume mobile data and should only be enabled intentionally.

## Network diagnostics and speed tests

See [docs/NETWORK_DIAGNOSTICS.md](docs/NETWORK_DIAGNOSTICS.md) and [docs/AI_NETWORK_DIAGNOSTICS.md](docs/AI_NETWORK_DIAGNOSTICS.md). DNS/HTTPS checks use the system resolver and certificate-verified HTTPS, and are limited to explicitly supplied hostnames. They do not scan IP ranges or impersonate third-party services.

For a full browser-style speed-test protocol, deploy a trusted backend such as [LibreSpeed](https://github.com/librespeed/speedtest) or [OpenSpeedTest](https://github.com/openspeedtest/Speed-Test). The CLI's generic throughput hooks do not yet implement either project's native protocol and never contact those services automatically.

## sing-box integration

See [docs/SING_BOX_ADAPTER.md](docs/SING_BOX_ADAPTER.md). The adapter wraps an already installed sing-box executable. It validates a complete JSON profile with `sing-box check`, writes it with restrictive permissions, starts the child process, checks process liveness, and stops/cleans up the process.

The adapter does **not** install or pin sing-box, configure OS routes, install system DNS settings, create a system TUN device itself, or guarantee end-to-end connectivity. A live process is not proof that routing or DNS works. Platform-specific route/DNS ownership, rollback, privilege design, and real-engine integration tests remain before production use.

## Engine roadmap

| Engine | Status |
|---|---|
| sing-box | Initial subprocess adapter implemented; platform networking validation remains |
| Xray-core | Planned |
| Mihomo | Planned |
| Aether | Research only |

Do not run multiple engines as competing owners of the same traffic path without an explicit, tested design.

## Security and limitations

The AI findings are deterministic heuristics based on observed measurements; no LLM or trained ML model is called. Carrier/SIM information is not available in this cross-platform Go CLI and is not guessed. No implementation can guarantee connectivity on every network or defeat every DPI system. TLS fingerprint rotation and imitation of named third-party apps are not implemented.

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md), [docs/ROADMAP.md](docs/ROADMAP.md), [docs/NETWORK_DIAGNOSTICS.md](docs/NETWORK_DIAGNOSTICS.md), and [SECURITY.md](SECURITY.md).

## Development

    gofmt -w .
    go vet ./...
    go test ./...

Contributions should include tests and document supported platforms and engine versions.
