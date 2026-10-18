# Android sing-box integration boundary (Phase B)

## Status

The current `internal/engine/singbox` adapter is a **desktop/server subprocess adapter**, not an Android VPN implementation. Do not wire it directly into an Android app and assume it owns the VPN. Android applications normally obtain packet capture through Android's `VpnService`; a native sing-box integration commonly uses the upstream Android/libbox integration surface rather than launching an arbitrary executable.

This repository currently has no Android application module, Kotlin service, JNI/libbox binding, Android manifest, or device instrumentation harness. Therefore Android TUN/DNS integration cannot honestly be marked complete from Go unit tests alone.

## Required Android ownership model

- The Android app's `VpnService` is the sole owner of the VPN TUN file descriptor and Android VPN consent/lifecycle.
- The Android integration layer must pass the TUN descriptor and validated runtime options to the supported sing-box Android/libbox API for the exact pinned upstream version. Do not create a competing TUN owner in this Go subprocess adapter.
- DNS behavior must be explicit in the profile and verified on-device. A healthy native process is not evidence that DNS queries use the intended resolver or that there are no leaks.
- Route inclusion/exclusion and IPv4/IPv6 behavior must be generated from the Android VPN builder and the sing-box profile as one coordinated policy. Test split-tunnel and default-route behavior separately.
- On stop, service revocation, process failure, and configuration replacement, close the TUN descriptor and stop the engine in a deterministic order. Android's VPN service lifecycle should release the VPN interface; the app must not attempt to restore arbitrary global routes by shelling out or requiring root.
- Never report a full-tunnel state until the service is established, the engine is alive, and device-level connectivity/DNS checks pass.

## Safe implementation sequence

1. Pin a specific upstream sing-box release and use its matching Android/libbox API and documented ABI. Keep the CLI adapter for platforms that support the executable model.
2. Add a separate Android application/integration module; keep Android framework types out of the portable Go adapter package.
3. Add instrumented tests on an emulator/device for consent denial, VPN establishment, IPv4 and IPv6 routes, DNS resolution, service revocation, app/process restart, and clean stop.
4. Test rollback by confirming the Android VPN interface is released and ordinary connectivity returns after stop/revocation. Use a controlled test endpoint and never change production device routes from CI.
5. Expose distinct health states for engine-process/native-runtime liveness, VPN interface established, DNS check passed, and end-to-end check passed. Do not collapse these into one process-liveness bit.

## Current limitations

- The Go subprocess adapter cannot request Android VPN consent, own `VpnService`, pass a TUN file descriptor, or validate Android route/DNS state.
- The current `Health` method reports child-process liveness only.
- Fake-executable Go tests validate process lifecycle contracts, not Android networking.
- No root access, shell-based route mutation, kill-switch guarantee, or DNS leak-prevention guarantee is implemented.

Reference: upstream sing-box documentation and Android integration sources must be checked against the exact version pinned by the Android module before implementation: https://sing-box.sagernet.org/ and https://github.com/SagerNet/sing-box-for-android
