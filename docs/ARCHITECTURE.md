# Architecture

## Design principles

1. One owner per system resource. A profile chooses one active data-plane engine. TUN and DNS ownership are explicit.
2. Separate control plane from data plane. The coordinator validates profiles and observes lifecycle/health; an adapter owns engine integration.
3. Version adapters. Each integration declares supported versions, capabilities, config validation, startup/stop behavior, and tests.
4. Fail closed when policy requires it. If a profile requires a tunnel, do not silently fall back to direct networking after engine failure.
5. Do not confuse process-alive with end-to-end connectivity. Health checks must be bounded, cancellable, and documented.
6. Never include credentials, private keys, tokens, or raw secret-bearing configurations in normal logs.

## Components

- `internal/config`: strict JSON loading, rejection of unknown fields, and structural validation.
- `internal/engine`: adapter contract, capability declarations, and common health model.
- `internal/orchestrator`: serialized engine lifecycle, health refresh, bounded cleanup, resource claims, and ordered failover.
- `internal/policy`: deterministic candidate selection; when the caller supplies verified metrics it prefers success rate, packet loss, latency, jitter, and throughput in that order.
- `internal/diagnostics`: bounded DNS/HTTPS checks and opt-in, capped HTTPS transfer samples.
- `internal/engine/native`: native TLS/WebSocket tunnel primitives and tests; these are not by themselves an operating-system VPN data plane.
- `android/`: Kotlin Android `VpnService` lifecycle and TUN setup, with a backend interface. The default backend deliberately fails closed.
- `native/`: C ABI validation and fusion-frame primitives; not a complete C VPN implementation.
- `integrations/python/`: dependency-free client SDK for the tenant-scoped control API.

## Engine roles

- Xray-core: optional protocol engine where the selected configuration is supported.
- sing-box: optional all-in-one engine when its TUN, DNS, and routing features fit.
- Mihomo: optional policy/group engine when its proxy-group and rules model fits.
- Aether: evaluate exact repository version, license, API, and whether path-management features can be reused outside its client.

These are alternative integrations, not four mandatory stages in one packet pipeline. Routing semantics and configuration formats differ; the coordinator must not assume direct compatibility.

## TLS and traffic testing

Use maintained TLS libraries, correct certificate validation, and documented configuration. No fingerprint guarantees acceptance by every server or network. Traffic generation belongs in an isolated test environment and should use synthetic workloads rather than impersonating named third-party apps.

## Lifecycle state machine

stopped -> starting -> healthy / degraded / failed -> stopping -> stopped

- Start: validate configuration and capabilities before changing system networking.
- Failure: report a stable error code and cancel dependent work.
- Stop: release resources and restore system state. Fleet resource claims stay reserved after a failed health check until an explicit `Fleet.Stop` succeeds; a failed probe alone is not proof that OS-level TUN, DNS, or route state was released.
- Recovery: bounded retries with backoff; never create unbounded reconnect loops.

## Implemented vs. incomplete

Implemented in this repository: strict profile/config validation, adapter and lifecycle contracts, failover and exclusive resource-claim coordination, deterministic selection with optional caller-verified quality metrics, native TLS/WebSocket tunnel primitives, bounded network diagnostics, a Kotlin Android VPN-service lifecycle, and CI/unit tests for these components.

Still incomplete and not to be represented as production-ready:
- The Android service defaults to `UnavailableVpnBackend`; a complete, bundled TUN-to-engine packet-forwarding backend is not present.
- The Go fleet coordinates engine lifecycles and resource claims; it does not merge traffic from multiple engines or provide true packet-level multipath bonding.
- DNS/route behavior and packet forwarding must be validated end-to-end on each target OS. Windows/Linux service integration and Android host-app wiring remain platform-specific integration work.
- The repository does not yet establish real-world throughput gains. In-memory benchmarks and capped HTTPS samples are not substitutes for repeatable tests over a real tunnel on representative networks/devices.
- A complete release build, rollback verification, and physical-device tests for DNS, IPv4/IPv6, network changes, disconnect cleanup, and route restoration remain required.
