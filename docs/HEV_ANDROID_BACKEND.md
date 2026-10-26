# Android Hev tun2socks adapter integration

## Implemented on this branch

- Kotlin lifecycle adapter for the pinned Hev JNI API; the TUN FD remains owned by VpnService.
- Reflective adapter for generated gomobile API dev.pgcore.mobile.mobile.Engine and SocketProtector, avoiding compile-time coupling to a generated AAR.
- Socket protection callback maps generated protect(long) to the Android VpnService.protect(fd) callback. A false result rejects the outbound connection.
- CI builds the generated Go Android AAR for ARMv7, ARM64, x86, and x86_64 and inspects its generated Java API.
- CI validates the pinned Hev 2.19.3 AAR and checksum.

This is NOT yet a production-ready VPN. The adapter is intentionally not registered as the default backend. The generated AAR is a CI artifact, not yet reproducibly bundled by this Android library/host app.

## Artifact and source pins

- Hev release: https://github.com/heiher/hev-socks5-tunnel/releases/tag/2.19.3
- Hev AAR: https://github.com/heiher/hev-socks5-tunnel/releases/download/2.19.3/hev-socks5-tunnel.aar
- Hev AAR SHA-256: ab6c581e6157680cfe5687ee7bbdcf95405c006c2db3701879ec44b9e5f83836
- Hev source and MIT license: https://github.com/heiher/hev-socks5-tunnel/tree/2.19.3
- Latest Go bridge build: https://github.com/AMINOPY/pg-core/actions/runs/40369604917
- Branch CI: https://github.com/AMINOPY/pg-core/actions?query=branch%3Awork%2Fandroid-hev-tun-backend

Preserve upstream copyright/license notices in redistributable builds. Do not use a moving latest URL.

## Host integration

The consuming Android host must bundle the generated Go AAR and pinned Hev AAR, then instantiate the adapter with a real end-to-end TUN traffic probe:

```kotlin
val nativeEngine = ReflectiveGoMobileSocksEngine(
    endToEndProbe = {
        // Run a bounded TCP + UDP/DNS probe through this VPN's TUN path.
        // Return success only after the expected test endpoint confirms traffic.
        runConfiguredTunTrafficProbe()
    },
)
val backend = HevSocks5TunnelBackend(
    cacheDir = applicationContext.cacheDir,
    engine = nativeEngine,
)
```

runConfiguredTunTrafficProbe() is an application-specific integration point, not a helper supplied by this library. Do not replace it with a successful stub or an upstream TLS/WebSocket handshake. Engine.health() validates native engine/upstream health; it cannot prove that packets traversed Android TUN.

Before invoking PgCoreVpnController.start, the host must register the backend and pass VpnServiceOptions.engineProfileJson. Register the backend only after the AARs are bundled and the probe is implemented. Until then UnavailableVpnBackend remains the safe default.

## Release gates requiring a consuming app and physical Android device

- Bundle and verify both pinned AARs for each supported ABI and preserve license/provenance.
- Run a real end-to-end probe through TUN for TCP, UDP, DNS, IPv4, and IPv6.
- Verify every upstream socket is protected before connect and fails closed when VpnService.protect(fd) returns false.
- Verify no traffic escapes outside the VPN during connect, reconnect, stop, network loss, or engine failure.
- Test app filtering, cancellation, repeated start/stop, descriptor/thread cleanup, network recovery, and soak stability.
- Capture test logs/results from at least one physical device per supported ABI family before calling this production-ready.

Repository CI can verify compilation, unit tests, native AAR generation, and artifact signatures. It cannot substitute for a real Android device or prove leak-free network behavior. Do not report the VPN as connected or production-complete until these gates pass.