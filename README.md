# Chimera Core (pg-core)

**Status: early scaffold / not production-ready.**

Chimera Core is a modular network-engine coordinator. It includes strict configuration validation, an engine adapter contract, a serialized lifecycle manager, deterministic health-aware candidate selection, bounded network diagnostics, unit tests, and CI. Initial sing-box and Xray subprocess adapters are available, but Chimera is not yet a production-ready VPN client.

## Current implementation

- Strict JSON parsing with unknown-field and trailing-value rejection
- Config validation for supported engine IDs and active-engine selection
- Common engine lifecycle/health interface and adapter registry
- Multi-engine fleet coordinator with independent per-engine lifecycle and explicit exclusive-resource claims
- Start validation, health checks, failure cleanup, and serialized lifecycle operations
- Deterministic route-candidate selection based on health and latency
- Bounded DNS/HTTPS diagnostics for explicitly supplied hostnames
- sing-box and Xray adapters: config validation, private temporary profiles, process start/health/stop
- Unit tests and GitHub Actions workflow

## Foundation (Phase A)

- Schema versioning, defensive structured-log redaction, lifecycle event bus, CLI diagnostics/status and capability negotiation
- The CLI engine list reports known IDs; it does not mean every engine is available in the CLI.

## sing-box integration (Phase B)

See [docs/SING_BOX_ADAPTER.md](docs/SING_BOX_ADAPTER.md). The adapter wraps an already installed sing-box executable. It validates a complete JSON profile with `sing-box check`, writes it with restrictive permissions, starts the child process, checks process liveness, and stops/cleans up the process.

The adapter does **not** install or pin sing-box, configure OS routes, install system DNS settings, create a system TUN device itself, or guarantee end-to-end connectivity. A live process is not proof that routing or DNS works. Platform-specific route/DNS ownership, rollback, privilege design, and real-engine integration tests remain before production use.

## Xray integration (initial)

See [docs/XRAY_ADAPTER.md](docs/XRAY_ADAPTER.md). The Xray adapter wraps an already-installed executable, validates and stores a private temporary profile, and manages its process lifecycle. It does not install Xray, create TUN, configure system routes/DNS, or prove end-to-end connectivity. Platform-specific validation remains outstanding.

## CDN/SNI

Optional `fronting` section with an SNI and a list of edge IPs, applied by the sing-box adapter. See [docs/CDN_SNI.md](docs/CDN_SNI.md).

For a two-field, no-local-config UI using the native TLS/WebSocket tunnel, see [Quick Connect](docs/QUICK_CONNECT.md). Set `CHIMERA_TUNNEL_KEY` once, then run `go run ./cmd/chimera --quick-connect`; the UI asks only for SNI and CDN edge IPs. This requires a server/Worker already deployed with the matching key and only exposes a local SOCKS5 proxy, not system-wide VPN routing.

## Multilanguage core

The Go/C/Python/Kotlin implementation is staged and documented in [docs/MULTILANGUAGE_CORE.md](docs/MULTILANGUAGE_CORE.md). Go remains the single lifecycle owner. A small C11 ABI primitive for bounded SNI syntax validation now lives in `native/` with standalone tests; it is not a tunnel engine and Go validation remains authoritative until parity tests are added. The dependency-free Python benchmark report validator lives in `tools/corecheck/` and runs in CI. `android/src/main/kotlin/dev/pgcore/EngineBoundary.kt` defines a UI-neutral Kotlin contract for a host app to forward lifecycle requests. It is not yet wired to a Gradle app, foreground service, JNI library, or Android instrumentation tests.

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
| native | Own engine: TLS+WebSocket tunnel, local SOCKS5, CDN/SNI. See [docs/NATIVE_ENGINE.md](docs/NATIVE_ENGINE.md) |
| sing-box | Initial subprocess adapter implemented; platform networking validation remains |
| Xray-core | Initial process adapter implemented; platform and real-engine validation remain |
| Mihomo | Planned |
| Aether | Research only |

The `internal/orchestrator.Fleet` can run independent engine processes together when the host app supplies accurate exclusive-resource claims; duplicate claims (for example `tun:default`) are rejected. This is process/lifecycle coordination only, not traffic multiplexing or automatic route/DNS isolation. Do not run engines as competing owners of the same traffic path without an explicit, tested host-network design.

## Security and limitations

No implementation can guarantee connectivity on every network or defeat every DPI system. TLS fingerprint rotation and imitation of named third-party apps are not implemented. Synthetic traffic may be used in controlled performance tests, not to impersonate real services.

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md), [docs/ROADMAP.md](docs/ROADMAP.md), [docs/NETWORK_DIAGNOSTICS.md](docs/NETWORK_DIAGNOSTICS.md), and [SECURITY.md](SECURITY.md).

## Development

    gofmt -w .
    go vet ./...
    go test ./...

Contributions should include tests and document supported platforms and engine versions.
