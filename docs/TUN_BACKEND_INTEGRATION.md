# TUN data-plane integration gate

This document is an implementation gate, not a claim that TUN forwarding is already available.

## Current hard boundary

The native engine exposes SOCKS5 TCP CONNECT over TLS/WebSocket. A first one-datagram UDP request/response protocol has now been added to the Go `Dialer` and VPS-side `Server`, with a local UDP echo regression test. It is deliberately **not yet wired to SOCKS5 UDP ASSOCIATE or Android TUN**, does not provide a persistent UDP association, and is not supported by `deploy/cloudflare-worker.js`. A UDP primitive is not full-tunnel support. Wiring Android `VpnService` to the current SOCKS endpoint still would not provide a complete VPN: UDP-based applications and DNS are not yet routed through the TUN backend.

The Android service must keep its fail-closed default until a real backend is registered. Do not show “connected” based only on a live service, an established TUN descriptor, or a successful TLS/WebSocket handshake.

## Dependency evaluation

`github.com/xjasonlyu/tun2socks/v2` is a plausible user-space IP stack for IPv4/IPv6 and TCP/UDP, but the inspected v2.6.0 release declares Go 1.23.1 (this repository currently declares Go 1.22). The newer engine API also exposes a `Start()` wrapper that calls fatal logging on startup error, which is not an acceptable failure boundary for an embeddable library. Do not import the convenience engine API without replacing that process-fatal behavior and explicitly managing device/stack/tunnel ownership.

Before adopting it, make an explicit toolchain decision, pin a reviewed version, inspect its full transitive dependency and license set, and use lower-level APIs or a maintained fork that returns startup errors and supports deterministic shutdown. Do not silently upgrade the project's Go toolchain.

## Required implementation sequence

1. **Define a versioned transport envelope.** Add message types, stream/session IDs, bounded payload lengths, deadlines, cancellation, and backpressure. Reject unknown versions/types and malformed lengths. Preserve TLS certificate verification.
2. **Finish remote datagram handling.** The Go client/server now support one bounded UDP request/response per WebSocket tunnel, but this is an initial protocol primitive only. Add protocol versioning/authentication for datagram mode, timeouts/rate limits, a persistent association or multiplexing strategy, and a compatible Worker-side implementation where the platform permits it. Wire the path into SOCKS5 UDP ASSOCIATE and verify DNS through the same controlled data plane or an explicitly configured, validated resolver path.
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
