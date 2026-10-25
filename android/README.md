# Android Kotlin integration

This directory contains the Kotlin boundary and Android VpnService lifecycle
integration. It is source code for inclusion in a host Android application,
not yet a standalone Gradle application.

## VPN service

PgCoreVpnService owns the Android VPN lifecycle: foreground notification,
TUN establishment, permission-revocation handling, health-gated startup, and
cleanup. The host must first call VpnService.prepare(context) and obtain
consent, then start the service with ACTION_START. To disconnect, send
ACTION_STOP.

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
