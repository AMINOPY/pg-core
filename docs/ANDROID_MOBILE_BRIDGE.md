# Android / Flutter bridge API

This package exposes the failover policy and bounded connector-outcome evidence as JSON-only Go APIs that can be exported with `gomobile`. It is the native boundary for an Android host; it does not create a VPN, manipulate Android routes, or replace `flutter_v2ray`'s connection lifecycle.

## Build the Android library

Use a Go toolchain supported by the pinned gomobile release, Android SDK/NDK, and the gomobile setup documented by the Go mobile project. From the repository root:

```sh
go test ./...
go install golang.org/x/mobile/cmd/gomobile@latest
gomobile init
mkdir -p build
gomobile bind -target=android -androidapi 23 -o build/chimera-core.aar ./mobile
```

Pin the gomobile version in CI/release automation rather than using `@latest` for reproducible release builds. The generated AAR must be integrated into the Android host app and exposed to Dart through a native `MethodChannel`. Generated Java/Kotlin signatures should be checked against the exact gomobile version used by the app.

## Failover API contract

`NewCore(configJSON)` accepts:

```json
{
  "failure_threshold": 2,
  "max_latency_ms": 1500,
  "cooldown_ms": 10000,
  "min_improvement_ms": 100
}
```

`EvaluateJSON(sampleJSON)` accepts sanitized samples from the app. Keep credentials, raw share links, full profile JSON, and secrets out of this bridge payload:

```json
{
  "current_id": "profile-1",
  "candidates": [
    {"id": "profile-1", "subscription_id": "sub-a", "status": "failed", "latency_ms": -1},
    {"id": "profile-2", "subscription_id": "sub-a", "status": "healthy", "latency_ms": 90}
  ]
}
```

It returns a proposal such as:

```json
{"current_id":"profile-1","switch":true,"target_id":"profile-2","reason":"current_unavailable","target_latency_ms":90}
```

Supported status values are `healthy`, `degraded`, `failed`, and `stopped`. Latency is milliseconds; `-1` means unavailable. Inputs are strictly decoded, capped at 64 candidates, and duplicate IDs/unknown JSON fields are rejected.

## AI network analysis API

The mobile package exposes `AnalyzeNetworkJSON(reportJSON)`. It accepts a bounded JSON report with measured throughput, latency, jitter, packet loss, DNS timing, transport type, and optional coarse cellular metadata. It returns a score, grade, findings, confidence estimates, and recommended next steps. The analyzer is deterministic and local, not a trained model.

The Dart client is `integrations/flutter_v2ray/lib/chimera_ai.dart`. The Android host should implement `ChimeraGoAIAnalyzer` by forwarding the JSON to `mobile.AnalyzeNetworkJSON`, and provide it from `ChimeraGoCoreFactory.createAIAnalyzer()`. The speed-test engine itself must be provided separately by the app (for example a user-approved self-hosted LibreSpeed/OpenSpeedTest server). The core does not start large transfers or silently consume mobile data.

Only pass coarse, user-approved metadata. Do not include subscriber identifiers, phone numbers, precise location, credentials, or raw connection profiles. See [AI Network Diagnostics](AI_NETWORK_SCANNER.md) for the full contract and validation checklist.

## Adaptive learning API

`NewLearningStore()` creates a bounded in-memory evidence store. The exported gomobile facade includes:

- `ObserveJSON(observationJSON)`: record a verified result.
- `SnapshotJSON()`: sanitized records in stable ID/version order.
- `RankedJSON()`: records sorted by evidence-based score (highest first), with a `score` field in the range 0–100.
- `ExportJSON()` and `ImportJSON(snapshotJSON)`: versioned persistence interchange.

Observation example:

```json
{"connector_id":"profile-2","version":"1","success":true,"latency_ms":90}
```

Failed observations must use `"success":false,"latency_ms":-1`. Record an observation only after the host independently verifies end-to-end connectivity; do not trust a connector's self-reported success. The store tracks success/failure counts, consecutive failures, and an exponentially weighted latency estimate. Scores are ranking hints only and do not grant permissions or automatically override failover policy. The store is bounded to 2,048 connector/version records and imports are limited to 1 MiB.

The Android adapter should implement both `ChimeraGoCore` and `ChimeraGoLearningStore`; `ChimeraGoCoreFactory.createLearningStore()` should wrap gomobile's `NewLearningStore`. Map the Kotlin `snapshot()` method to Go `RankedJSON()`. Keep exported snapshots in platform-protected storage if persistence is desired. Learning is not automatically persisted, and a fresh store is created after engine detachment.

## Required host-side sequence

1. Let `flutter_v2ray` own the single Android VPN session for this integration. Do not start another TUN owner for the same traffic path.
2. Probe only app-supplied profiles. Send IDs, status, and measured latency to `EvaluateJSON`; do not send credentials or raw profiles.
3. If `switch` is false, keep the active profile and continue monitoring.
4. If true, ask the plugin to switch to `target_id`. Independently verify that the new session is connected and a user-approved connectivity check succeeds.
5. Only then call `ConfirmSwitch(targetID)`. On any failed, cancelled, or unverified switch, restore the previous profile if possible and call `CancelSwitch()`.
6. Record the verified result through `ObserveJSON`; serialize all switch operations in the host. A successful plugin start callback alone is not proof that routes, DNS, and end-to-end traffic work.

The policy facade is thread-safe. It does not implement Android `VpnService`, route/DNS ownership, a kill switch, persistent profile storage, or atomic handover. Device tests must cover permission denial, process death, failed replacement, rollback, IPv4/IPv6, DNS leaks, and network changes before describing the integration as production-ready.
