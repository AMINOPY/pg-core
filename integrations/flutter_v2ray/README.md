# Flutter V2Ray adapter for Chimera

This folder is an app-side integration adapter for the upstream
blueboy-tm/flutter_v2ray plugin. It is intentionally separate from the Go module:
Flutter/Dart packages cannot be imported directly into Go.

## Add to a Flutter application

The upstream README currently documents version 1.0.10 and requires Dart >=3.1
and Flutter >=3.3. Check the current package release, supported platforms, and
license before shipping.

Run:

    flutter pub add flutter_v2ray

Copy lib/chimera_flutter_v2ray.dart into the Flutter application's lib tree,
then create the adapter with the app's status callback:

    final engine = ChimeraFlutterV2Ray(
      onStatusChanged: (status) {
        // Publish status to app state. Never log raw profiles or links.
      },
    );
    await engine.initialize();

    final probes = await engine.probeCandidates(candidates);
    final available = probes.where((p) => p.isAvailable).toList();
    if (available.isNotEmpty) {
      await engine.connect(available.first.candidate, proxyOnly: false);
    }

## Three-part integration

1. Health sampling: Go internal/health probes explicitly configured HTTPS health
   URLs with timeout and bounded concurrency, returning -1 on failure. The
   Flutter adapter exposes the plugin's own server-delay probe for supplied share
   links. These are different probe types and should not be conflated.
2. Failover execution: Go internal/failover.Service runs the policy against
   sampled candidates and calls a platform Switcher. It verifies the new
   candidate and invokes rollback if verification fails. The app must implement
   that Switcher against the actual active engine and own the connection state.
3. Flutter/VPN lifecycle: ChimeraFlutterV2Ray wraps plugin initialization,
   profile parsing, delay checks, connect/disconnect, and best-effort replacement
   rollback. The upstream API does not promise atomic handover.

## Important boundary

The Go service and this Dart adapter are not connected through Go FFI or a
MethodChannel in this repository yet. A production app must add a native bridge
that passes sanitized candidate IDs, subscription IDs, health samples, and
switch results between Go policy and Flutter. Do not send share links,
credentials, or raw configuration through logs. The plugin must be the single
VPN/TUN owner while active; do not run the sing-box adapter as a second VPN
owner on the same traffic path.

Test Android permission denial/revocation, process death, failed replacement,
rollback, DNS, IPv4/IPv6, route restoration, and real connectivity on physical
devices before enabling unattended automatic switching.
