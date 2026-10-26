# VPN Core Fusion Architecture

## Goal

Build one VPN data plane that can coordinate multiple engine adapters instead of treating each engine as an unrelated toggle. “Fusion” means explicit orchestration and policy-controlled routing first; it does **not** imply that arbitrary engine protocols can be combined transparently or that packets are automatically bonded across engines.

This document describes the target architecture and acceptance gates. It is not a claim that the full data plane is already implemented.

## Fusion layers

1. **Engine adapters** — each adapter exposes a stable lifecycle, capabilities, health, and a local data-plane endpoint. Adapters must own only their own process/session and must stop deterministically.
2. **Capability registry** — records supported protocols and features (TCP, UDP, IPv4, IPv6, DNS, multiplexing, packet forwarding), plus whether each capability is tested. Unknown capabilities are unavailable, not assumed.
3. **Policy and scheduler** — selects a usable path per flow using explicit policy (priority/failover, latency-aware selection, or weighted distribution). Policy decisions must be observable and reproducible. A path is eligible only when its health and capability requirements pass.
4. **Unified packet boundary** — Android TUN packets enter one backend. The backend routes TCP/UDP and DNS according to policy, preserves flow affinity where needed, and prevents upstream sockets from looping back through the VPN.
5. **Health and recovery** — end-to-end probes, bounded timeouts, cancellation, network-change handling, and explicit states (starting, connected, degraded, failed). Process liveness or a TLS handshake alone must never mean the VPN is connected.
6. **Telemetry and safety** — per-path errors/latency, selected path and reason, counters, and leak-test results without logging secrets or full user traffic.

## Modes

- **Single path / failover:** one active engine path, another takes over after a failed health check. This is the first safe fusion mode.
- **Per-flow distribution:** different flows may use different healthy paths. Flow affinity must be preserved for the lifetime of a TCP flow.
- **Packet-level multipath bonding:** split one flow across multiple paths. This requires a sequence/reassembly protocol, congestion and retransmission strategy, MTU handling, anti-replay protection, and coordinated server support. It is a separate protocol feature and must not be implied by weighted per-flow routing.

## Required adapter contract

Each adapter must provide:
- stable engine ID and version;
- declared capabilities and configuration schema;
- start, stop, and bounded healthCheck operations;
- structured errors and cancellation;
- a local endpoint or packet I/O interface;
- upstream socket protection on Android before connecting;
- deterministic cleanup and no process-fatal errors;
- a tested compatibility matrix for client, server, transport, and Android ABI.

Adapters must not silently mutate shared configuration or start duplicate VPN services. One coordinator owns connection state and the TUN descriptor.

## Current repository reality

The repository currently has a native TLS/WebSocket SOCKS transport, a Go-side UDP relay prototype, Android service/backend contracts, and a fail-closed unavailable backend. Android TUN-to-engine packet forwarding and a bundled JNI/native implementation are not present. The current UDP transport opens a WebSocket per datagram and is not a production multiplexed transport. Existing path configuration is not proof of real multipath bonding.

## Implementation order

1. Define capability and lifecycle contracts with unit tests.
2. Implement one real end-to-end adapter and TUN packet path; keep Android fail-closed until it passes.
3. Add DNS handling, IPv4/IPv6 and TCP/UDP integration tests, and upstream socket protection.
4. Add single-path health-based failover and network-change recovery.
5. Add per-flow distribution only after path identity and flow-affinity behavior are tested.
6. Consider packet-level bonding only with a specified interoperable wire protocol and a cooperating server implementation.
7. Run leak tests, repeated start/stop tests, network handover tests, physical-device tests, soak tests, and declared-device performance measurements.

## Acceptance gates

- No UI or API reports connected until a bounded end-to-end probe through the TUN succeeds.
- Failure of one adapter cannot silently route traffic outside the VPN; behavior is fail-closed.
- DNS and IPv4/IPv6 traffic follow documented policy; leak tests pass on real devices.
- Network loss invalidates stale flows and recovery does not reuse dead sockets.
- Each selected path is observable with a reason; unsupported capabilities are rejected.
- Start/stop is idempotent, cancellation closes blocked I/O, and TUN FD ownership is explicit.
- CI, race tests, Android tests, physical-device validation, and security review pass before a production claim.

## Non-goals

- No claim that Xray, sing-box, WireGuard, and the repository's native engine can be merged internally into one binary.
- No arbitrary protocol translation without an adapter.
- No packet-level bonding claim from simple weighted path selection.
- No disabling TLS certificate validation to improve compatibility.
