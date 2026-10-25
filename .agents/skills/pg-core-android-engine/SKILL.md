---
name: pg-core-android-engine
description: Apply PG Core-specific Android engine lifecycle, VPN ownership, security, and validation rules when changing the Kotlin boundary or Android integration.
---

# PG Core Android engine integration

Use this skill only for Android/Kotlin/platform integration work in PG Core. Combine it with relevant upstream Android skills installed under `.agents/skills/android-upstream/`.

## Required workflow

1. Inspect `android/`, the Android CI workflow, engine interfaces, architecture docs, and existing tests before editing.
2. Keep the Kotlin boundary thin and explicit. Do not silently move routing, DNS, TUN, or process ownership into an adapter.
3. Preserve cancellation and serialized start/stop semantics. A failed stop or cleanup must not be reported as a clean disconnect while resources may still be active.
4. Keep secrets and full connection profiles out of logs. Validate inputs at trust boundaries and bound all external input, response sizes, and deadlines.
5. Do not introduce new Android Gradle/plugin dependencies unless the repository actually has and needs that build setup. Prefer the existing CI toolchain.

## VPN and networking invariants

- One host-owned Android `VpnService` should own the system TUN and OS-level routes/DNS configuration.
- Engine adapters may manage their own process and local listeners but must not claim that this creates a complete system VPN.
- Process liveness is not data-plane health. Separate process status, listener status, and a real end-to-end probe in names and reports.
- Handle permission denial/revocation, network changes, IPv4/IPv6, DNS failures, cancellation, reconnect, stop failure, and cleanup rollback.
- Do not add silent traffic collection or unbounded speed tests. Diagnostics must be user-visible, bounded, cancellable, and use trusted endpoints.

## Verification

- Run relevant Go unit tests and `go vet ./...` for shared-core changes.
- Run the repository's Kotlin/Android CI checks for Android changes.
- Add tests for lifecycle state transitions and failure cleanup.
- Clearly label emulator/device-only checks that were not run. Do not claim production readiness without real-device validation of VPN lifecycle, routes, DNS, network changes, and restoration after disconnect.
