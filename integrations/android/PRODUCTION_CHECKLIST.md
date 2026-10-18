# Android integration completion checklist

This repository provides the Go policy API, a Flutter MethodChannel host, and Dart failover orchestration. It is **not yet a standalone Android app or a production-ready VPN client**. Complete this checklist in the consuming Flutter application.

## 1. Build the Go policy AAR reproducibly

1. Install the Android SDK/NDK and a Go version compatible with the selected gomobile release.
2. Pin exact Go, gomobile, SDK and NDK versions in CI; do not use `@latest` for release builds.
3. Run `go test ./...` and `go test -race ./...`.
4. Build `gomobile bind -target=android -androidapi 23 -o build/chimera-core.aar ./mobile`.
5. Inspect the generated Java API and implement `ChimeraGoCoreFactory` against those actual signatures. Do not guess generated class names; keep generated types behind the adapter interface.
6. Add the AAR to the host application's local Maven repository or `app/libs` and verify ABI packaging for every supported architecture.

## 2. Register the native bridge

Register `ChimeraCoreMethodChannel(factory)` with the FlutterEngine used by the app. Confirm the native channel name matches the Dart client: `com.aminopy.chimera/core`.

The factory should create a fresh Go policy instance from the bounded JSON configuration. Never put API keys, subscription URLs, profile credentials, or raw profile contents into the policy JSON.

## 3. Keep one VPN/TUN owner

For the current integration, `flutter_v2ray` owns the Android VPN session. Do not concurrently run another `VpnService`, sing-box TUN, or other route/DNS owner for the same traffic path. The policy bridge only proposes a profile switch; it does not establish a tunnel or bypass network filtering.

Android VPN permission must be requested through the selected VPN plugin's supported flow. Handle denial, revocation, app process death, and Android foreground-service requirements according to the plugin and target SDK documentation.

## 4. Required failover semantics

- Candidate profiles must be explicitly provided by the app; do not silently discover or execute arbitrary profiles.
- Send only sanitized candidate IDs, status, and measured latency to the Go policy.
- Serialize manual and automatic connect/disconnect operations through one app-owned coordinator.
- A switch proposal is not success. Confirm only after an independent bounded HTTPS request succeeds through the new session.
- On failed verification, restore the prior profile best-effort and cancel the policy proposal.
- Avoid logging profile URIs, authentication material, or complete plugin configuration.

## 5. Device test matrix before production

Test on real supported Android versions and devices:

- VPN permission denied, revoked, and granted.
- Invalid configuration and native core initialization failure.
- Process killed during start/switch and device network transition Wi-Fi ↔ cellular.
- Candidate start failure, verification timeout, rollback failure, and user cancellation.
- DNS resolution and leak checks; IPv4 and IPv6 routing; route restoration after disconnect.
- Background/foreground transitions, battery restrictions, and app upgrade.
- Multiple simultaneous triggers; prove no overlapping switch operation occurs.

Record Android version, device model, plugin version, Go/gomobile versions, and the exact test result. Do not describe DPI resistance, leak prevention, seamless handover, or production readiness unless the relevant test has actually passed.

## Current limitations

- The repo does not contain a complete Flutter host application, generated AAR, Android SDK/NDK build pipeline, or device-test lab.
- Kotlin MethodChannel source therefore has not been compiled against the consuming app's Flutter embedding and generated gomobile API here.
- Switches stop/start the selected plugin session and are not atomic or guaranteed seamless.
- The Go package makes a policy decision only. It does not own routes, DNS, Android `VpnService`, a kill switch, or an adaptive DPI-circumvention transport.
- CI passing on the Go/Kotlin/Python/Java repository jobs is not evidence of a successful Android device build or network-leak test.
