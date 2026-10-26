# Android Hev tun2socks adapter integration

## What this change provides

This branch adds a Kotlin adapter for the public JNI contract exposed by
[heiher/hev-socks5-tunnel](https://github.com/heiher/hev-socks5-tunnel), plus
unit tests for lifecycle ordering, loopback endpoint validation, and cleanup.
The adapter starts the host-provided protected SOCKS engine first, then starts
Hev against the service-owned Android TUN descriptor. It stops Hev before the
SOCKS engine and never closes the TUN descriptor.

The adapter is deliberately **not registered by default**. The existing
`UnavailableVpnBackend` remains the safe default until an app supplies both
a working engine bridge and the native AAR.

## Verified Hev release pin

The integration is pinned to upstream release **2.19.3** (released 2026-10-25),
not a moving `latest` URL:

- Release: https://github.com/heiher/hev-socks5-tunnel/releases/tag/2.19.3
- AAR asset: https://github.com/heiher/hev-socks5-tunnel/releases/download/2.19.3/hev-socks5-tunnel.aar
- AAR SHA-256: `ab6c581e6157680cfe5687ee7bbdcf95405c006c2db3701879ec44b9e5f83836`
- Upstream source and MIT license: https://github.com/heiher/hev-socks5-tunnel/tree/2.19.3

The upstream release API reports that checksum for the published AAR. The
`hev-artifact-validation.yml` workflow downloads the exact version, verifies
the digest, and checks that the expected JNI class and method signatures exist.
This validation does **not** mean the AAR has been bundled into the Android
library or that device traffic has been tested.

Preserve the upstream MIT license and copyright notices in any distributable
build. Do not download a moving release during production builds.

## Host wiring

The host application must implement `PgCoreSocksEngine` using a pinned,
tested binding of this repository's `mobile` Go package. That bridge must:
- start the native SOCKS listener using the supplied profile JSON;
- synchronously call Android `VpnService.protect(fd)` for every upstream socket
  before connecting;
- return a loopback SOCKS endpoint only;
- stop idempotently and retain ownership if shutdown fails;
- provide `endToEndHealthCheck()` that proves packets traverse the complete TUN
  data plane, not merely an upstream TLS/WebSocket check.

Before calling `PgCoreVpnController.start`, the host must register the backend
and pass its engine profile in `VpnServiceOptions.engineProfileJson`:

```kotlin
PgCoreVpnBackendRegistry.factory = {
    HevSocks5TunnelBackend(
        cacheDir = applicationContext.cacheDir,
        engine = registeredPgCoreSocksEngine,
    )
}

PgCoreVpnController.start(
    context = applicationContext,
    options = VpnServiceOptions(engineProfileJson = profileJson),
)
```

The sample is intentionally incomplete until the host has a concrete
`registeredPgCoreSocksEngine` backed by the compiled native bridge. Do not
substitute a fake or no-op implementation.

## Remaining release gates

This adapter does not itself bundle the AAR, generate the gomobile/JNI
binding, or prove that packets traverse the complete path. Before this is
production-ready, CI and device tests must verify:

- the pinned AAR digest and JNI API contract pass artifact validation;
- the Go mobile engine bridge is built and registered by a real host app;
- TCP, UDP, DNS, IPv4, IPv6, and configured app filtering work end-to-end;
- every upstream socket is protected before connect and no routing loop occurs;
- stop/start, cancellation, failed native startup, network loss/recovery, and
  cleanup retries do not leak descriptors or leave workers running;
- a real-device leak/soak test passes before the UI can report connected.

Until these gates pass, keep `UnavailableVpnBackend` as the default and do not
claim that a production full-tunnel VPN is complete.
