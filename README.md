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
- AI diagnostics report: repeated HTTPS checks, latency min/average/max, failed-sample ratio, jitter estimate, and evidence-backed rule findings
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

The AI scan accepts 1–20 samples (default 5), uses bounded per-request timeouts, emits JSON, and returns a non-zero exit code if no HTTPS samples succeed. It reports HTTPS request timing, not ICMP ping or download/upload throughput.

## Network diagnostics and speed tests

See [docs/NETWORK_DIAGNOSTICS.md](docs/NETWORK_DIAGNOSTICS.md) and [docs/AI_NETWORK_DIAGNOSTICS.md](docs/AI_NETWORK_DIAGNOSTICS.md). DNS/HTTPS checks use the system resolver and certificate-verified HTTPS, and are limited to explicitly supplied hostnames. They do not scan IP ranges or impersonate third-party services.

For actual download/upload speed, deploy a trusted speed-test backend such as [LibreSpeed](https://github.com/librespeed/speedtest) or [OpenSpeedTest](https://github.com/openspeedtest/Speed-Test). Those services are not bundled or automatically contacted by the current AI scan. Run throughput tests only when you understand the data usage and have configured a trusted endpoint.

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
