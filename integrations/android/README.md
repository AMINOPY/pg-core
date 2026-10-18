# Android MethodChannel integration

**Implementation status:** policy and bridge source are present; the consuming app's gomobile adapter and Android device validation are not complete. See the [production checklist](PRODUCTION_CHECKLIST.md).

This directory contains the native Flutter channel host for the Go failover policy. The Dart side lives in `integrations/flutter_v2ray/lib/chimera_core_bridge.dart` and the execution coordinator is `chimera_auto_failover.dart`.

## Current integration boundary

- `ChimeraCoreMethodChannel` exposes `initialize`, `evaluate`, `confirmSwitch`, and `cancelSwitch` on channel `com.aminopy.chimera/core`.
- The Android application must supply `ChimeraGoCoreFactory`, implemented against the exact generated gomobile AAR version. Its adapter must map the generated `NewCore`, `EvaluateJSON`, `ConfirmSwitch`, and `CancelSwitch` methods to `ChimeraGoCore`.
- Register the handler with the application's FlutterEngine, passing that factory, e.g. `engine.plugins.add(ChimeraCoreMethodChannel(factory))`.
- Add the Kotlin file to the Android host app's source set and include Flutter embedding dependencies. This repository has no standalone Android/Flutter application module, Gradle wrapper, generated AAR, or device-test environment; therefore this source has not yet been compiled against a concrete generated gomobile artifact.
- On Dart, initialize `ChimeraCoreBridge`, then create `ChimeraAutoFailover` with the existing `ChimeraFlutterV2Ray` adapter. The coordinator probes only the explicitly supplied profiles, sends sanitized samples to Go, switches the plugin session, invokes the app-provided connectivity verifier, and confirms only after successful verification.

## Example Dart setup

```dart
final core = ChimeraCoreBridge();
await core.initialize();

final engine = ChimeraFlutterV2Ray(
  onStatusChanged: (status) {
    // Update app state; do not log profile links or secrets.
  },
);
await engine.initialize();

final failover = ChimeraAutoFailover(
  core: core,
  engine: engine,
  verifyConnectivity: (candidate) async {
    // Implement a bounded HTTPS check through the active tunnel. Do not
    // treat plugin connected status or a pre-connect latency probe as proof.
    return await appOwnedConnectivityCheck();
  },
);

// Call from an app-owned timer/network-state observer, and serialize with
// all manual connect/disconnect actions.
await failover.runOnce(explicitCandidates);
```

The example's `appOwnedConnectivityCheck` and `explicitCandidates` are application-provided values, not APIs supplied by this repository.

## Security and lifecycle

- Never send share links, passwords, full profile JSON, or credentials over this channel.
- Use one VPN/TUN owner: `flutter_v2ray` for this integration. Do not run the sing-box subprocess adapter as a competing owner of the same traffic path.
- Confirm only after the new session passes an independent end-to-end check. On failed verification, restore the prior candidate best-effort and cancel the Go proposal.
- Switching stops the prior plugin session before starting the next one; it is not an atomic or seamless handover.
- Before unattended use, build against a pinned gomobile version and test on supported Android devices: permission denial/revocation, app/process death, failed start, rollback, DNS, IPv4/IPv6, network changes, and route restoration.
