# Android mobile bridge status

## Implemented and CI-verified on the work branch

The Go package `mobile` exposes the native TLS/WebSocket engine lifecycle, and the pinned gomobile workflow successfully builds and uploads an Android AAR.

- `Engine.start(profileJSON, protector)` refuses to start without a socket-protection callback.
- Outbound edge sockets pass through Go's `net.Dialer.Control`; the callback receives the raw descriptor and calls Android `VpnService.protect(fd)` before connect proceeds.
- If protection fails, the outbound connection fails closed.
- `health(timeoutMillis)` requires the configured native-engine health result; a TLS/WebSocket handshake alone is not proof that user traffic traverses the TUN data plane.
- `stop(timeoutMillis)` preserves ownership if shutdown has not completed.
- Unit tests cover the protection callback and fail-closed behavior.
- The generated Java API was inspected in CI: `Engine.start(String, SocketProtector)`, `addr()`, `health(long)`, `stop(long)`, and `SocketProtector.protect(long)`.
- The AAR contains JNI libraries for ARMv7, ARM64, x86, and x86_64.

Latest successful bridge build at commit `e780dc26344e70c2887d6f6defb704f62e75e6fa`:
https://github.com/AMINOPY/pg-core/actions/runs/40369604917

Generated artifact (expires 2026-11-09):
https://github.com/AMINOPY/pg-core/actions/runs/40369604917/artifacts/12672236521

At that commit, Android Kotlin CI, Go CI, the Android Go mobile bridge build, Hev AAR validation, and Dependency Security all report success.

## Not yet completed — must not be represented as production-ready

The generated Go AAR is a CI artifact and is **not yet wired into `PgCoreVpnService`**. The service intentionally remains on `UnavailableVpnBackend`; the Hev backend is not registered automatically. No physical Android device was available to this workflow, so real TCP/UDP/DNS forwarding and leak testing have not been demonstrated.

Required gates before enabling the backend:

1. Reproducibly package the generated Go AAR and pinned Hev AAR into the consuming Android app/library, with ABI and license/provenance checks.
2. Implement and wire a Kotlin adapter for the generated `dev.pgcore.mobile.mobile.Engine` / `SocketProtector` API.
3. Connect Android TUN FD → Hev tun2socks → local SOCKS endpoint → Go engine, and ensure every upstream socket is protected before connect.
4. Prove TCP, UDP, DNS, IPv4/IPv6, app filtering, cancellation, stop/start, network-loss recovery, and resource cleanup with automated end-to-end tests.
5. Run physical-device leak/soak tests and verify traffic cannot escape outside the VPN when connecting, disconnecting, or when the tunnel fails.

Do not register the backend or show a connected state until those gates pass. Passing repository CI proves source tests and AAR generation, not that real traffic is correctly tunneled or leak-free.
