# Android MethodChannel integration

**Implementation status:** Go policy, bounded learning store, Dart bridge, and Kotlin channel source are present; the consuming app's gomobile adapter and Android device validation are not complete. See the [production checklist](PRODUCTION_CHECKLIST.md).

This directory contains the native Flutter channel host. The Dart bridge lives in `integrations/flutter_v2ray/lib/chimera_core_bridge.dart` and the execution coordinator is `chimera_auto_failover.dart`.

## Current integration boundary

- Failover methods: `initialize`, `evaluate`, `confirmSwitch`, and `cancelSwitch`.
- Learning methods: `recordOutcome`, `learningSnapshot`, `exportLearning`, and `importLearning`.
- The Android application must supply `ChimeraGoCoreFactory` against the exact generated gomobile AAR. Map `NewCore`, `EvaluateJSON`, `ConfirmSwitch`, and `CancelSwitch` to `ChimeraGoCore`. Implement `createLearningStore()` using gomobile's `NewLearningStore`, mapping `ObserveJSON`, `RankedJSON`, `ExportJSON`, and `ImportJSON` to `ChimeraGoLearningStore`.
- Register the handler with the application's FlutterEngine, e.g. `engine.plugins.add(ChimeraCoreMethodChannel(factory))`.
- Add the Kotlin file to the Android host app's source set and include Flutter embedding dependencies. This repository has no standalone Android/Flutter application module, Gradle wrapper, generated AAR, or device-test environment; therefore the source has not been compiled against a concrete generated gomobile artifact.
- On Dart, initialize `ChimeraCoreBridge`, then create `ChimeraAutoFailover` with the existing `ChimeraFlutterV2Ray` adapter. The coordinator switches only explicitly supplied profiles and confirms only after the app's independent connectivity verifier succeeds.

## Adaptive learning

Call `recordOutcome` only after independently verifying the outcome. It accepts connector ID, connector version, success, and latency—not credentials, share links, or raw profile data. Failed observations must use latency `-1`. `learningSnapshot` returns sanitized statistics sorted by evidence-based score; `exportLearning` / `importLearning` allow the app to persist the versioned store in platform-protected storage. Persistence is not automatic, and learning scores do not grant permissions.

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
    // Use a bounded HTTPS check through the active tunnel.
    return await appOwnedConnectivityCheck();
  },
);
await failover.runOnce(explicitCandidates);
```

The example's `appOwnedConnectivityCheck` and `explicitCandidates` are application-provided, not APIs supplied by this repository.

## Security and lifecycle

- Never send share links, passwords, full profile JSON, or credentials over this channel.
- Use one VPN/TUN owner: `flutter_v2ray`. Do not run the sing-box subprocess adapter as a competing owner of the same traffic path.
- On failed verification, restore the prior candidate best-effort and cancel the Go proposal.
- Switching stops the prior plugin session before starting the next one; it is not an atomic or seamless handover.
- Before unattended use, build against a pinned gomobile version and test permission denial/revocation, process death, failed start, rollback, DNS, IPv4/IPv6, network changes, and route restoration.
