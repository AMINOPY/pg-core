# Chimera Core (pg-core)

**Status: early scaffold / not production-ready.**

Chimera Core is a modular network-engine coordinator. It includes strict configuration validation, an engine adapter contract, a serialized lifecycle manager, deterministic health-aware candidate selection, bounded network diagnostics, unit tests, and CI. A first sing-box subprocess adapter is available, but Chimera is not yet a production-ready VPN client.

## Current implementation

- Strict JSON parsing with unknown-field and trailing-value rejection
- Config validation for supported engine IDs and active-engine selection
- Common engine lifecycle/health interface and adapter registry
- Start validation, health checks, failure cleanup, and serialized lifecycle operations
- Deterministic route-candidate selection based on health and latency
- Bounded DNS/HTTPS diagnostics for explicitly supplied hostnames
- sing-box subprocess adapter: upstream config check, private temporary profile, process start/health/stop
- Process-exit and health-context tests, plus GitHub Actions CI

## Foundation (Phase A)

- Schema versioning, defensive structured-log redaction, lifecycle event bus, CLI diagnostics/status and capability negotiation
- The CLI engine list reports known IDs; it does not mean every engine is available in the CLI.

## sing-box integration (Phase B)

See [docs/SING_BOX_ADAPTER.md](docs/SING_BOX_ADAPTER.md). The portable adapter wraps an already installed sing-box executable. It validates a complete JSON profile with `sing-box check`, writes it with restrictive permissions, starts the child process, checks process liveness, and stops/cleans up the process.

**Android status:** this subprocess adapter is not an Android VPN implementation. See [docs/ANDROID_SING_BOX.md](docs/ANDROID_SING_BOX.md) for the required `VpnService`/TUN ownership model, DNS and route verification, service-revocation handling, and connectivity rollback. This repository does not yet include an Android app module or device-level tests.

The adapter does **not** install or pin sing-box, configure OS routes, install system DNS settings, create a system TUN device itself, or guarantee end-to-end connectivity. A live process is not proof that routing or DNS works. No kill-switch or DNS leak-prevention guarantee is made.

## Quick start

Requires Go 1.22 or newer.

    go test ./...
    go vet ./...
    go run ./cmd/chimera
    go run ./cmd/chimera --config config.example.json

## Network diagnostics

See [docs/NETWORK_DIAGNOSTICS.md](docs/NETWORK_DIAGNOSTICS.md). Checks use system DNS and certificate-verified HTTPS, and are limited to explicitly supplied hostnames. They do not scan IP ranges or impersonate third-party services.

## Engine roadmap

| Engine | Status |
|---|---|
| sing-box | Portable subprocess adapter implemented; Android requires separate native integration |
| Xray-core | Planned |
| Mihomo | Planned |
| Aether | Research only |

Do not run multiple engines as competing owners of the same traffic path without an explicit, tested design.

## Security and limitations

No implementation can guarantee connectivity on every network or defeat every DPI system. TLS fingerprint rotation and imitation of named third-party apps are not implemented. Synthetic traffic may be used in controlled performance tests, not to impersonate real services.

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md), [docs/ROADMAP.md](docs/ROADMAP.md), [docs/NETWORK_DIAGNOSTICS.md](docs/NETWORK_DIAGNOSTICS.md), and [SECURITY.md](SECURITY.md).

## Development

    gofmt -w .
    go vet ./...
    go test ./...

Contributions should include tests and document supported platforms and engine versions.
