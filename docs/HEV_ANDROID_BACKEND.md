# Android Hev tun2socks adapter integration

## What this change provides

This branch adds a Kotlin adapter for the public JNI contract exposed by
[heiher/hev-socks5-tunnel](https://github.com/heiher/hev-socks5-tunnel), plus
unit tests for lifecycle ordering, loopback endpoint validation, and cleanup.
The adapter starts the host-provided protected SOCKS engine first, then starts
Hev against the service-owned Android TUN descriptor. It stops Hev before the
SOCKS engine and never closes the TUN descriptor.

The adapter is deliberately **not registered by default**. The existing
`UnavailableVpnBackend` remains the safe default until an app supplies both a
working engine bridge and the native AAR.

## Pinned native dependency

The upstream repository documents a self-contained Android AAR and the
`hev.htproxy.TProxyService` JNI class. The upstream release list currently
shows version `2.18.0` as latest:
- https://github.com/heiher/hev-socks5-tunnel/releases/tag/2.18.0
- https://github.com/heiher/hev-socks5-tunnel/blob/2.18.0/README.md

Before shipping, the host build must fetch and pin the exact AAR artifact from
the selected release, verify its checksum against a trusted release record,
retain the upstream MIT license/copyright notices, and verify that the class
and method signatures match this adapter. Do not use an unpinned `latest`
artifact or silently download a moving release during production builds.

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

This adapter does not by itself bundle the AAR, generate the gomobile/JNI
binding, or prove that packets traverse the complete path. Before this is
production-ready, CI and device tests must verify:

- the selected AAR is pinned, checksummed, included for supported ABIs, and
  loads on each ABI;
- the Go mobile engine bridge is built and registered by a real host app;
- TCP, UDP, DNS, IPv4, IPv6, and configured app filtering work end-to-end;
- every upstream socket is protected before connect and no routing loop occurs;
- stop/start, cancellation, failed native startup, network loss/recovery, and
  cleanup retries do not leak descriptors or leave workers running;
- a real-device leak/soak test passes before the UI can report connected.

Until these gates pass, keep `UnavailableVpnBackend` as the default and do not
claim that a production full-tunnel VPN is complete.
