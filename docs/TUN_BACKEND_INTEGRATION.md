# TUN data-plane integration gate

This document is an implementation gate, not a claim that TUN forwarding is already available.

## Current hard boundary

The native engine supports SOCKS5 TCP CONNECT and a loopback SOCKS5 UDP ASSOCIATE path. UDP currently uses one bounded WebSocket tunnel per datagram through the Go Dialer and VPS-side Server. Local UDP echo and SOCKS5 UDP-association regression tests cover the path, including IPv4/domain/IPv6 packet parsing, malformed/fragmented packet rejection, and a no-response timeout. The UDP handshake now carries an explicit protocol version and rejects unknown versions. Server negotiation and UDP response reads have bounded deadlines.

This remains an initial transport, not a high-performance multiplexed UDP data plane. UDP mode requires the Go VPS-side server; `deploy/cloudflare-worker.js` remains TCP-only. Android TUN still has no packet-forwarding backend/JNI integration, so these SOCKS features do **not** mean the Android full-tunnel VPN is complete.

The Android service must keep its fail-closed default until a real backend is registered. Do not show “connected” based only on a live service, an established TUN descriptor, or a successful TLS/WebSocket handshake.

## Dependency evaluation

`github.com/xjasonlyu/tun2socks/v2` is a plausible user-space IP stack for IPv4/IPv6 and TCP/UDP, but the inspected v2.6.0 release declares Go 1.23.1 (this repository currently declares Go 1.22). The newer engine API also exposes a `Start()` wrapper that calls fatal logging on startup error, which is not an acceptable failure boundary for an embeddable library. Do not import the convenience engine API without replacing that process-fatal behavior and explicitly managing device/stack/tunnel ownership.

Before adopting it, make an explicit toolchain decision, pin a reviewed version, inspect its full transitive dependency and license set, and use lower-level APIs or a maintained fork that returns startup errors and supports deterministic shutdown. Do not silently upgrade the project's Go toolchain.

## Required implementation sequence

1. **Complete the versioned transport envelope.** UDP negotiation now includes a protocol version and rejects unsupported versions. Still needed: explicit message types, stream/session IDs, bounded framing, cancellation, backpressure, and compatibility policy. Preserve TLS certificate verification.
2. **Harden remote datagram handling.** Add datagram-mode authentication, per-peer rate limits, and a persistent association or multiplexing strategy. The current per-datagram WebSocket is not suitable for high-volume general-purpose UDP. The Worker implementation remains TCP-only unless its platform supports UDP forwarding. Verify DNS through the same controlled data plane or an explicitly configured, validated resolver path.
3. **Integrate the user-space IP stack.** Read packets from the Android TUN FD, forward IPv4/IPv6 TCP/UDP through the engine, protect upstream sockets with `VpnService.protect` before connect, and ensure the engine never routes its own upstream connections back into the TUN.
4. **Make lifecycle ownership explicit.** Start returns an error instead of exiting the process; stop is idempotent, cancels blocked I/O, waits for workers, and does not close the service-owned TUN FD. A failed stop must retain ownership and surface a retryable error.
5. **Fail closed during recovery.** On network loss, stop accepting new data-plane work, invalidate stale upstream sockets, and re-establish the upstream path on network availability. Never report healthy until a bounded end-to-end probe succeeds.
6. **Validate before enabling by default.** Build and load native artifacts for every supported Android ABI; run unit/integration tests; verify TCP, UDP, IPv4, IPv6, DNS, cancellation, network handover, and stop/start cycles; then perform physical-device leak and soak tests.

## Release acceptance checklist

- [ ] Backend startup fails with a structured error; no process-fatal logging.
- [ ] TUN FD ownership is documented and tested; no double-close or descriptor leak.
- [ ] TCP and UDP pass end-to-end through the configured remote server.
- [ ] DNS queries are observable only on the intended resolver path; leak tests pass.
- [ ] IPv4 and IPv6 routes behave as configured, including when one family is disabled.
- [ ] Upstream sockets are protected before connection; no VPN routing loop.
- [ ] Network loss/recovery and repeated start/stop pass stress tests.
- [ ] Native binaries build for every supported ABI and load on real devices.
- [ ] Latency/throughput results are measured on a declared device/network; local microbenchmarks are labeled separately.
- [ ] Android UI reports connected only after the backend's end-to-end health check passes.

Until these checks pass, the VPN service should continue to fail closed rather than imply full-tunnel protection.

## Dependency review update (2026-10-26)

The upstream `xjasonlyu/tun2socks` v2.6.0 release is MIT-licensed, but its `go.mod` declares Go 1.23.1 while this repository declares Go 1.22. Its public `engine.Start()` and `engine.Stop()` wrappers call fatal logging on errors, so they are not safe lifecycle boundaries for an Android library. The repository also does not currently contain an Android-specific `device.Device` adapter that accepts the borrowed Android `VpnService` TUN file descriptor and guarantees that the service retains descriptor ownership.

References reviewed:
- https://github.com/xjasonlyu/tun2socks/releases/tag/v2.6.0
- https://github.com/xjasonlyu/tun2socks/blob/v2.6.0/go.mod
- https://github.com/xjasonlyu/tun2socks/blob/v2.6.0/engine/engine.go
- https://github.com/xjasonlyu/tun2socks/blob/v2.6.0/LICENSE

### Decision

Do not add the upstream CLI/release binary or call the fatal engine wrapper from the Android service. The viable next implementation is a separately owned, error-returning Go adapter around the lower-level gVisor stack/device APIs (or a reviewed Android-compatible library that already provides this contract), exposed to Kotlin through a pinned gomobile/JNI build. It must consume the borrowed TUN FD without closing it, route all upstream sockets through a protect-before-connect callback, and stop workers deterministically. This requires an explicit Go toolchain/Android ABI build decision and an actual device integration test; a dependency declaration alone would not complete the data plane.

### Release gate status

- [x] Android service remains fail-closed when no data-plane backend is installed.
- [x] Backend interface defines borrowed TUN FD ownership, socket protection, start/stop, and health checks.
- [ ] Android TUN-to-IP-stack packet reader/writer implementation and packaged JNI/AAR artifacts.
- [ ] End-to-end TCP, UDP, DNS, IPv4, and IPv6 tests through the Android TUN.
- [ ] Network transition/recovery, leak checks, and sustained physical-device test.
- [ ] Promote PR from Draft only after the above gates pass.
