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
- sing-box adapter: upstream config check, private temporary profile, process start/health/stop
- Bounded HTTPS health probes and serialized failover service with switch verification/rollback hooks
- Thread-safe JSON API in `mobile` for exposing failover policy through gomobile
- Dart adapter for the upstream `flutter_v2ray` plugin
- Java, Python, and Kotlin bridge-client scaffolds (host transport still required)
- Unit tests and GitHub Actions workflow

## Foundation (Phase A)

Schema versioning, defensive structured-log redaction, lifecycle event bus, CLI diagnostics/status and capability negotiation are implemented. The CLI engine list reports known IDs; it does not mean every engine is available in the CLI.

## sing-box integration (Phase B)

See [docs/SING_BOX_ADAPTER.md](docs/SING_BOX_ADAPTER.md). The adapter wraps an already installed sing-box executable. It validates a complete JSON profile with `sing-box check`, writes it with restrictive permissions, starts the child process, checks process liveness, and stops/cleans up the process.

The adapter does **not** install or pin sing-box, configure OS routes, install system DNS settings, create a system TUN device itself, or guarantee end-to-end connectivity. A live process is not proof that routing or DNS works. Platform-specific route/DNS ownership, rollback, privilege design, and real-engine integration tests remain before production use.

## Quick start

Requires Go 1.22 or newer.

    go test ./...
    go vet ./...
    go run ./cmd/chimera
    go run ./cmd/chimera --config config.example.json

## Network diagnostics

See [docs/NETWORK_DIAGNOSTICS.md](docs/NETWORK_DIAGNOSTICS.md). Checks use system DNS and certificate-verified HTTPS, and are limited to explicitly supplied hostnames. They do not scan IP ranges or impersonate third-party services.

## Health probes and executable failover

The bounded probe implementation in [internal/health](internal/health) samples only explicitly configured HTTPS endpoints, uses verified TLS, limits concurrency and request time, and treats failed probes as latency -1. [internal/failover](internal/failover) can run the policy periodically through an application-provided switcher, verify the replacement, and request rollback if verification fails. It does not itself own OS routing or TUN, and the caller must provide the correct health endpoint and current active-candidate state.

## Flutter / Android integration

- [Flutter V2Ray adapter](integrations/flutter_v2ray/README.md) wraps the upstream plugin for profile parsing, latency measurement, and connection lifecycle.
- [Android mobile bridge API](docs/ANDROID_MOBILE_BRIDGE.md) documents the gomobile-friendly JSON API and host-side switch/verify/confirm sequence.

The Go API now exposes failover decisions to a native Android host, but this repository still does **not** include a complete Flutter application, generated AAR, production Kotlin `MethodChannel`, or Android `VpnService` implementation. Device-level verification of routes, DNS, rollback, and end-to-end traffic is still required. The Flutter plugin must be the single VPN/TUN owner for that integration; do not run it alongside the sing-box adapter for the same traffic path.

## Multi-language clients

See [docs/LANGUAGE_BINDINGS.md](docs/LANGUAGE_BINDINGS.md) for Java, Python, and Kotlin client scaffolds. These clients do not embed the Go runtime or create a VPN; each host application must provide and test a transport to the policy API.

## Engine roadmap

| Engine | Status |
|---|---|
| sing-box | Initial subprocess adapter implemented; platform networking validation remains |
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


## Website / application integration API

A small authenticated HTTP API is available for server-side integration. It currently validates connector manifests; it does not execute plugins, perform automatic code generation, start a VPN, or guarantee DPI circumvention.

See [docs/INTEGRATE_API.md](docs/INTEGRATE_API.md) for local startup, curl, Node.js, and Python examples.

Quick local run:

    export CHIMERA_API_KEY="$(openssl rand -hex 32)"
    go run ./cmd/chimera-api

The API binds to `127.0.0.1:8080` by default and refuses to start without a key. Keep the key on the server; never embed it in browser JavaScript or a mobile application bundle. For public deployments, use HTTPS via a trusted reverse proxy and configure access controls, rate limiting, and exact CORS origins.

- `GET /healthz` — public process-health check.
- `POST /v1/manifests/validate` — bearer-authenticated, size-bounded manifest validation.

This is the first integration surface, not yet the completed adaptive runtime. Connector discovery, permission-scoped execution, sandboxing, signed package distribution, device enrollment, and a tested DPI-resilient transport remain future implementation work.
