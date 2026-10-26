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
- Bounded DNS/HTTPS diagnostics and HTTPS transfer samples with timing breakdown, cancellation, redirect rejection, URL redaction, and strict byte/time limits
- Ghost network symptom analysis: repeated bounded DNS/HTTPS observations, cautious evidence-based findings, and recovery recommendations without treating failures as proof of DPI
- sing-box and Xray adapters: config validation, private temporary profiles, process start/health/stop
- Native TLS+WebSocket tunnel, local SOCKS5, and CDN/SNI support
- Native tunnel WebSocket framing tuned to 64 KiB chunks, with a reproducible framing/masking benchmark
- Platform-neutral network-policy contract for routing mode, DNS ownership intent, IP-family policy, MTU and deadlines
- Explicit HTTPS transport-profile validation and bounded discovery of Ed25519-signed manifests
- Persistent monotonic manifest-sequence storage to reject rollback across restarts, when the host persists and uses the store
- Validated Auto/Direct/CDN Fronting and regional auto-ir/auto-us candidate modes with bounded TLS probes and stable latency ranking (planning/probing, not a complete tunnel)
- Unit tests, fuzz targets, and GitHub Actions workflow

## Ghost network analysis

See [docs/GHOST.md](docs/GHOST.md). Run `go run ./cmd/chimera --ghost example.com --samples 5` to collect bounded DNS/HTTPS observations and receive cautious findings. Ghost does not prove DPI from a single symptom, intercept traffic, alter system networking, or guarantee circumvention.

## Remote control API

See [docs/REMOTE_CONTROL_API.md](docs/REMOTE_CONTROL_API.md) for the tenant-scoped HTTP/JSON API foundation for app announcements, release metadata, and app-local user status. The `cmd/controlplane` service supports a durable single-instance file store and loopback-only serving. Multi-instance database storage, operator login/MFA, rate limiting, key rotation, audit persistence, and production deployment hardening remain before production.

## Traffic fusion

See [docs/TRAFFIC_FUSION.md](docs/TRAFFIC_FUSION.md). The `internal/fusion` package provides deterministic path selection, concurrency-safe active-flow reservations, bounded route-quality probes (success rate, packet loss, mean RTT and jitter), and a `Router` that invokes path-specific context-aware dialers for new outbound connections. It releases leases on dial failure or connection close and refuses to remove routes while they still have active connections.

This is flow-level routing: each connection uses one path. It does **not** bond multiple paths into one flow or implement packet-level aggregation. `fusion.RoutesFromSOCKS5` now binds route entries to already-running engine SOCKS5 listeners, and `fusion.ProbeSOCKS5Path` can test a TCP connection through a selected engine to a caller-chosen trusted target. `fusion.RefreshRoutes` refreshes supplied route snapshots with bounded, context-aware probes and protects against stale concurrent updates; the host still owns engine lifecycle, listener configuration, health snapshot collection, and probe scheduling. Only new TCP connections explicitly opened through `Router.DialContext` use this router. It is not a system-wide packet router and is not yet connected to OS TUN/routes/DNS or a live telemetry collector.

## Engine lifecycle and coexistence

The `internal/orchestrator.Fleet` can manage independent engine processes when the host app supplies accurate exclusive-resource claims. Duplicate claims (for example, `tun:default`) are rejected. This is process/lifecycle coordination, not traffic multiplexing or automatic route/DNS isolation. The host must define a single owner for each TUN device, route table, DNS configuration, and listener. Never start competing engines against the same system resources without an explicit, tested network design.

## sing-box integration

See [docs/SING_BOX_ADAPTER.md](docs/SING_BOX_ADAPTER.md). The adapter wraps an already installed sing-box executable. It validates a complete JSON profile with `sing-box check`, writes it with restrictive permissions, starts the child process, checks process liveness, and stops/cleans up the process.

The adapter does **not** install or pin sing-box, configure OS routes, install system DNS settings, create a system TUN device itself, or guarantee end-to-end connectivity. A live process is not proof that routing or DNS works. Platform-specific route/DNS ownership, rollback, privilege design, and real-engine integration tests remain before production use.

## Xray integration

See [docs/XRAY_ADAPTER.md](docs/XRAY_ADAPTER.md). The Xray adapter wraps an already-installed executable, validates and stores a private temporary profile, and manages its process lifecycle. It does not install Xray, create TUN, configure system routes/DNS, or prove end-to-end connectivity. Platform-specific validation remains outstanding.

## CDN/SNI and transport profiles

The optional `fronting` section accepts an SNI and edge IP list for supported profiles. See [docs/CDN_SNI.md](docs/CDN_SNI.md).

The `internal/transportprofile` package validates explicit HTTPS/WebSocket/gRPC endpoint profiles and verifies Ed25519-signed manifests with expiry, host binding, and sequence checks. Bounded manifest discovery uses certificate-verified HTTPS and rejects redirects. The file sequence store can persist monotonic manifest versions; the host must actually provision the trusted public key and use the store to obtain rollback protection across restarts.

See [docs/CDN_TRANSPORT.md](docs/CDN_TRANSPORT.md) and [docs/MANIFEST_ROLLBACK_PROTECTION.md](docs/MANIFEST_ROLLBACK_PROTECTION.md). This is profile validation and discovery, not a completed CDN transport dialer. It does not bypass CDN provider policy, install OS routes, or provide a universal zero-configuration service without a trusted bootstrap identity and reachable server endpoint.

For a two-field, no-local-config UI using the native TLS/WebSocket tunnel, see [Quick Connect](docs/QUICK_CONNECT.md). Set `CHIMERA_TUNNEL_KEY` once, then run `go run ./cmd/chimera --quick-connect`; the UI asks only for SNI and CDN edge IPs. This requires a server/Worker already deployed with the matching key and only exposes a local SOCKS5 proxy, not system-wide VPN routing.

## CDN connection modes

See [docs/CDN_CONNECTION_MODES.md](docs/CDN_CONNECTION_MODES.md). `auto` produces a deterministic direct-first candidate plan, `direct` requires endpoint/SNI identity agreement, and `cdn_fronting` requires explicit edge IPs. `auto-ir` and `auto-us` accept caller-supplied regional candidate lists; they do not ship a live regional IP database. TLS probes keep certificate verification enabled and are bounded, and results can be ranked by reachability and latency. These modes do not discover IP ranges, bypass provider policy, or automatically alter the active engine's transport. Candidate selection is not proof that the native tunnel can use the candidate.

## Network policy and platform support

See [docs/NETWORK_POLICY.md](docs/NETWORK_POLICY.md) and [docs/ENGINE_FUSION_RECOVERY.md](docs/ENGINE_FUSION_RECOVERY.md). The network-policy package is a validated, platform-neutral declaration; it does not itself modify routes, DNS, firewall rules, or TUN devices. The new `internal/netrecovery.Session` provides transactional capture/apply/restore/verify semantics and retains snapshots when restoration fails so cleanup can be retried. It is a safety mechanism, not an OS backend: Android `VpnService`, Linux TUN/routes/DNS, and Windows TUN/routes/DNS backends plus host privilege handling and real-device/host tests are still required before claiming system-wide routing, DNS leak protection, crash recovery, or fail-closed behavior.

## Multilanguage core

The Go/C/Python/Kotlin implementation is staged in [docs/MULTILANGUAGE_CORE.md](docs/MULTILANGUAGE_CORE.md). Go remains the single lifecycle owner. A small C11 ABI primitive validates SNI syntax; it is not a tunnel engine, and Go validation remains authoritative until parity tests are complete. Python is used for test tooling and benchmark-report validation. `android/src/main/kotlin/dev/pgcore/EngineBoundary.kt` defines a UI-neutral Kotlin contract, but is not yet wired to a complete Gradle app, foreground service, JNI library, or Android instrumentation tests.

See [docs/LANGUAGE_STRATEGY.md](docs/LANGUAGE_STRATEGY.md). Additional languages should be introduced only for a clear component-level need, not by duplicating the same core logic.

## Quick start

Requires Go 1.22 or newer.

    go test ./...
    go vet ./...
    go run ./cmd/chimera
    go run ./cmd/chimera --config config.example.json

## Tunnel performance

See [docs/TUNNEL_PERFORMANCE.md](docs/TUNNEL_PERFORMANCE.md) for the 64 KiB WebSocket frame default, benchmark commands, and limitations. No internet throughput improvement is claimed until measured on representative devices and network paths.

## Network diagnostics

See [docs/NETWORK_DIAGNOSTICS.md](docs/NETWORK_DIAGNOSTICS.md). Checks use system DNS and certificate-verified HTTPS, and are limited to explicitly supplied hostnames or HTTPS endpoints. The bounded transfer sample reports endpoint-specific timing and payload throughput, caps data at 8 MiB and duration at 60 seconds, disables redirects, and omits URL paths/query strings from returned diagnostics. It is not a tunnel health guarantee or ISP line-speed test. These tools do not scan IP ranges or impersonate third-party services.

## Engine roadmap

| Engine | Status |
|---|---|
| native | TLS+WebSocket tunnel, local SOCKS5, CDN/SNI. See [docs/NATIVE_ENGINE.md](docs/NATIVE_ENGINE.md) |
| sing-box | Initial subprocess adapter; platform networking validation remains |
| Xray-core | Initial process adapter; platform and real-engine validation remain |
| Mihomo | Planned |
| Aether | Research only |

## Security and limitations

No implementation can guarantee connectivity on every network or defeat every DPI system. TLS fingerprint rotation and imitation of named third-party apps are not implemented. Synthetic traffic may be used in controlled performance tests, not to impersonate real services. Signed manifests require trusted key provisioning and careful key rotation; storing a sequence is effective only when the host uses the persistent store.

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md), [docs/ROADMAP.md](docs/ROADMAP.md), [docs/NETWORK_DIAGNOSTICS.md](docs/NETWORK_DIAGNOSTICS.md), [docs/CDN_TRANSPORT.md](docs/CDN_TRANSPORT.md), [docs/CDN_CONNECTION_MODES.md](docs/CDN_CONNECTION_MODES.md), [docs/NETWORK_POLICY.md](docs/NETWORK_POLICY.md), [docs/LANGUAGE_STRATEGY.md](docs/LANGUAGE_STRATEGY.md), and [SECURITY.md](SECURITY.md).

## Development

    gofmt -w .
    go vet ./...
    go test ./...

Contributions should include tests and document supported platforms and engine versions.


## Explainable network diagnostics

The optional `--ai-scan` CLI runs a bounded series of DNS/HTTPS checks and reports latency summaries, failed-sample ratio, timing variation, and evidence-backed rule findings. It is a deterministic diagnostics module, **not a trained AI model** and not a system-VPN health check.

Example:

```sh
go run ./cmd/chimera --ai-scan example.com --samples 5
```

Throughput checks are opt-in and must use endpoints you control or trust:

```sh
go run ./cmd/chimera --ai-scan example.com --download-url https://your-test-host.example/download --upload-url https://your-test-host.example/upload
```

The downloader caps transfer size; the uploader sends a bounded payload. Use an endpoint that explicitly supports these request types. Diagnostics are point-in-time measurements and do not claim to measure pure ICMP ping, carrier/SIM identity, or the performance of an active tunnel. See [docs/AI_NETWORK_DIAGNOSTICS.md](docs/AI_NETWORK_DIAGNOSTICS.md).


## Multi-language fusion foundation

See [docs/MULTILANG_FUSION.md](docs/MULTILANG_FUSION.md). Go, C11, and Kotlin now share a bounded PGF1 frame envelope with a 16-byte header and a 64 KiB payload cap. This is framing only, not an authenticated tunnel or production packet-bonding data plane: no session authentication, replay protection, TUN integration, or cooperating multi-path server is included.
