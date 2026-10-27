# Android mobile bridge status

## Implemented on the work branch

- Go package `mobile` exposes the native TLS/WebSocket engine lifecycle. GitHub Actions builds `mobile-engine.aar` for ARMv7, ARM64, x86 and x86_64.
- `GoMobilePgCoreSocksEngine` is a concrete Kotlin reflection adapter for the generated gomobile API. It passes Android's `VpnService.protect(fd)` callback through the generated `SocketProtector` interface.
- `HevSocks5TunnelBackend` starts the Go engine before Hev's TUN-to-SOCKS data plane, keeps ownership of the borrowed TUN descriptor in `PgCoreVpnService`, and rolls back on startup/health failure.
- `PgCoreHevMobileIntegration.install(...)` provides explicit opt-in registry wiring. The safe default remains `UnavailableVpnBackend`.
- A `VpnTrafficProbe` is mandatory for successful health checks. It must prove actual traffic traverses the VPN; the Go engine's TLS/WebSocket health check alone is not accepted as end-to-end proof.
- The Go bridge enforces a non-null socket protector and aborts upstream connection setup if `VpnService.protect(fd)` fails.
- Hev AAR 2.19.3 is checksum-pinned and its JNI class/method signatures are checked by CI.

Latest branch commit: `e7835d4fe0b5f5377d4c4c0d6fa9c7e8555ec1ca`.

Workflow results for the latest commit can be inspected here:
- Android Kotlin CI: https://github.com/AMINOPY/pg-core/actions/runs/40444768293
- Go CI: https://github.com/AMINOPY/pg-core/actions/runs/40444768281
- Android Go mobile bridge (AAR build): https://github.com/AMINOPY/pg-core/actions/runs/40444768237
- Hev Android artifact validation: https://github.com/AMINOPY/pg-core/actions/runs/40444768144
- Dependency Security: https://github.com/AMINOPY/pg-core/actions/runs/40444768138

## Release blockers — not yet a verified usable VPN

This repository contains an Android **library**, not a consuming/installable Android application. CI builds and checks artifacts, but it does not install the app on a physical phone or prove traffic is leak-free. Do not label this production-ready until all gates below are evidenced.

1. **Host app packaging:** the consuming app must package both the generated Go AAR and the checksum-pinned Hev AAR, validate native ABIs, and include license notices. This repository does not currently supply a finished consuming app/APK.
2. **Real traffic probe:** the host must implement `VpnTrafficProbe` to issue bounded TCP, UDP and DNS tests through the device VPN and verify the results independently. It must fail closed on any missing/failed protocol test.
3. **Leak testing:** verify upstream sockets are protected before connect; test DNS routing, IPv4/IPv6, app filtering and external-IP behavior while connected and after disconnect.
4. **Lifecycle/recovery:** test cancellation, stop/start, permission revocation, network loss/recovery, failed cleanup, and resource/descriptor leaks.
5. **Physical device:** run repeated connect/disconnect and a sustained traffic soak on real Android hardware; retain device model, Android version, ABI, test logs and results.

No physical-device test has been performed by these workflows. The backend must remain opt-in and must not report CONNECTED until the host's actual TCP/UDP/DNS-through-TUN probe passes. CI success is necessary but is not evidence of real-phone traffic forwarding or zero leaks.
