# Android mobile bridge status

## Implemented and CI-verified on the work branch

The Go package `mobile` exposes the existing native TLS/WebSocket engine lifecycle, and the pinned gomobile workflow now successfully builds and uploads an Android AAR.

- `Engine.Start(profileJSON, protector)` refuses to start without a socket-protection callback.
- Outbound edge sockets pass through Go's `net.Dialer.Control`; the callback obtains the raw file descriptor and calls `SocketProtector.Protect(fd)` before connect proceeds.
- If protection fails, the outbound connection fails closed.
- `Health(timeoutMillis)` requires the native engine's configured health result; a TLS/WebSocket-only handshake is not proof that user traffic traverses the TUN data plane.
- `Stop(timeoutMillis)` preserves ownership if shutdown has not completed.
- Unit tests cover the protection callback and fail-closed behavior.
- The Android Go mobile bridge workflow builds `mobile-engine.aar`, inspects the generated Java API, and uploads the artifact with ARMv7, ARM64, x86, and x86_64 JNI libraries.

Latest successful bridge build (artifact retained by GitHub Actions for 14 days): https://github.com/AMINOPY/pg-core/actions/runs/40368247460

## Integration still required before production use

The generated AAR is a build artifact; it is **not yet wired into `PgCoreVpnService`**. The service deliberately remains on `UnavailableVpnBackend` until the complete data plane is connected and validated.

1. Integrate the generated AAR into the consuming Android app/library build reproducibly, including its ABI packaging and licensing/provenance.
2. Implement and wire the Kotlin adapter for the generated `dev.pgcore.mobile.mobile.Engine` / `SocketProtector` API.
3. Build/pin an Android tun2socks backend for supported ABIs and connect its local SOCKS endpoint to the mobile engine.
4. Ensure tun2socks sockets cannot loop through TUN; protect any upstream sockets before connect.
5. Add end-to-end data-plane tests for TCP, UDP, DNS, IPv4/IPv6, app filtering, cancellation, network loss/recovery, and stop/start cleanup.
6. Run leak/soak and physical-device tests, including confirmation that traffic does not escape outside the VPN.

Do not register a production backend or report VPN connected until those gates pass. Passing repository CI proves that source tests and AAR generation succeed; it does not prove that real device traffic is correctly tunneled or leak-free.
