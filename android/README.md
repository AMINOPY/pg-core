# Android Kotlin integration

This directory contains the Kotlin boundary, Android VpnService lifecycle integration,
and `PgCoreVpnController` helper for embedding in a host Android application. It is
an Android library module, not a standalone end-user VPN application. See
[Android host-app integration](../docs/ANDROID_APP_INTEGRATION.md).

## VPN service

PgCoreVpnService owns the Android VPN lifecycle: foreground notification,
TUN establishment, permission-revocation handling, health-gated startup, and
cleanup. The host must first call VpnService.prepare(context) and obtain
consent, then start the service with ACTION_START. Host apps can use `PgCoreVpnController`
to check permission, launch the consent flow, start with validated options, and
request a stop. To disconnect manually, send ACTION_STOP.

Android-specific options can be passed as intent extras defined in VpnServiceOptions:
MTU, numeric DNS server addresses, an allowed-app list, or a disallowed-app
list. App allowlisting and denylisting are mutually exclusive and bounded;
an empty list means Android's default all-app behavior. Unknown package names
fail startup rather than silently changing the requested policy.

The service deliberately fails closed by default. UnavailableVpnBackend is
registered unless the host explicitly installs a real PgCoreVpnBackend. This
prevents claiming success while silently blackholing traffic. A backend must
forward IP packets between the TUN descriptor and a real engine and pass an
end-to-end data-plane health check. Process liveness alone is insufficient.

The manifest fragment declares the service and VPN binding permission. Merge
it into the host application's manifest and validate foreground-service
requirements against the target SDK and Android versions.

## Multi-path status

VpnBackendConfig.paths provides a bounded, validated path configuration
contract for a future/host-supplied packet engine. It is not itself traffic
bonding. True multipath requires a packet/session protocol at both ends
(sequence numbers, authentication, replay protection, bounded reordering,
congestion control, path health and MTU handling), plus a server endpoint.
The existing Go router selects one path per outbound connection; it does not
bond packets from one flow over multiple links. Neither feature is complete
until a real backend and server implementation are integrated and tested.

## Current limitations

- No Gradle wrapper or standalone Android app module is provided here.
- No JNI/native TUN packet-forwarding library is bundled by this change.
- No end-to-end multi-path server protocol is bundled.
- This code has not been built with an Android SDK or tested on a physical
  Android device in this environment.

Do not register a backend factory until its start/stop semantics, packet
forwarding, DNS, IPv4/IPv6, network-change recovery, and data-plane health
checks have been tested.
