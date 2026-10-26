# Android host-app integration

This repository's `android` directory is an Android library module that a host
application can include. It is not a complete installable VPN app and it does
not bundle a working TUN-to-engine packet-forwarding backend.

## 1. Include the library

Include the repository's `:android` module in the host Gradle build (or copy
the module and keep its package namespace). Use a compatible Android Gradle
Plugin, Kotlin plugin, Android SDK, and Java 17 toolchain. Merge the service
declaration from `android/src/main/AndroidManifest.xml` into the app manifest
and verify foreground-service requirements against the app's target SDK.

## 2. Request VPN consent

Call `PgCoreVpnController.prepareIntent(context)`. If it returns an Intent,
launch that Intent using the host app's Activity Result API and wait for the
result. Only after the user grants permission should the host call
`PgCoreVpnController.start(context, options)`. If it returns null, permission
is already granted.

Example from an Activity after the consent result is successful:

```kotlin
PgCoreVpnController.start(
    context = this,
    options = VpnServiceOptions(
        mtu = 1500,
        dnsServers = listOf("1.1.1.1"),
    ),
)
```

To request disconnection from a foreground UI, call
`PgCoreVpnController.stop(context)`. Android may restrict service starts from
background states; the host must handle platform exceptions and can call
`stopService(Intent(context, PgCoreVpnService::class.java))` as a fallback.
The service's destruction path attempts backend cleanup and closes its TUN
descriptor.

## 3. Install a real backend before connecting

The default `PgCoreVpnBackendRegistry` uses `UnavailableVpnBackend`. It returns
`vpn_backend_unavailable` and the service closes the newly established TUN
rather than reporting a false connection. The host must register a tested
implementation before calling start:

```kotlin
PgCoreVpnBackendRegistry.factory = { HostSuppliedVpnBackend(/* dependencies */) }
```

`HostSuppliedVpnBackend` above is a placeholder name, not a class included in
this repository. The implementation must forward IP packets from the borrowed
TUN file descriptor through a real engine, protect upstream sockets with the
provided callback to prevent VPN loops, stop all workers and sockets, and
perform a bounded end-to-end data-plane health check. It must handle IPv4,
IPv6, DNS, MTU, network changes, cancellation, and cleanup failure. Do not
register a process-liveness-only adapter as a packet backend.

## 4. Status and release gate

Implemented in this repository:

- Android `VpnService` lifecycle and foreground notification
- Android consent boundary and validated MTU/DNS/app filtering options
- Host-app controller for permission checks, start, and stop requests
- Backend contract and safe fail-closed default
- Kotlin unit tests and CI build/test workflow

Not yet complete or verified:

- A bundled JNI/native TUN-to-engine data plane
- An end-to-end adapter that forwards packets to the selected native, sing-box,
  or Xray engine
- Transactional device-level DNS/route restoration beyond Android's own TUN
  lifecycle
- Real packet-level multi-path bonding and a cooperating server
- Instrumentation tests on supported Android versions and physical devices
- Validation of UDP, DNS leaks, IPv4/IPv6 behavior, network handover, forced
  process death, and recovery after backend failure

A successful Gradle build only proves that the library compiles and its unit
tests pass. It does not prove that device traffic traverses a tunnel. Do not
ship this as a production VPN until the backend and device-level release gates
are completed.
