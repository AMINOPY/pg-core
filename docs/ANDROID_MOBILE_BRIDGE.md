# Android mobile bridge status

## Implemented in this branch

The Go package `mobile` exposes the existing native TLS/WebSocket engine lifecycle to a future gomobile/JNI host:

- `Engine.Start(profileJSON, protector)` refuses to start without a socket-protection callback.
- Every outbound edge socket passes through Go's `net.Dialer.Control`; the callback obtains the raw file descriptor and calls `SocketProtector.Protect(fd)` before connect proceeds.
- If protection fails, the dial fails closed.
- `Health(timeoutMillis)` only accepts the engine's real data-plane health result; a TLS/WebSocket-only probe is not enough.
- `Stop(timeoutMillis)` preserves ownership if shutdown has not completed.
- Unit tests cover the protection callback and fail-closed behavior.

## Not yet complete

This package is **not yet a compiled Android AAR and is not wired to `PgCoreVpnService`**. The service still defaults to `UnavailableVpnBackend`. The following integration remains mandatory:

1. Bind `mobile` into an Android AAR using a pinned Go/mobile toolchain and expose the generated `SocketProtector` interface to Kotlin.
2. Build/pin an Android tun2socks backend for supported ABIs and connect its local SOCKS endpoint to the mobile engine.
3. Ensure the tun2socks implementation's own sockets cannot loop through TUN; if it creates any non-loopback upstream sockets, protect those before connect too.
4. Connect the Kotlin backend to the generated bridge, with bounded start/stop, cancellation and worker joining.
5. Run automated TCP/UDP/DNS/IPv4/IPv6 tests, network-loss recovery, leak/soak tests, and physical-device tests.

Do not register a production backend or report VPN connected until those gates pass. Repository CI can validate source-level tests but cannot substitute for a real Android device/network leak test.
