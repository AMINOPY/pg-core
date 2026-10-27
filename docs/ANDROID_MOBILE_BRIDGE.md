# Android mobile bridge status

## Implemented on the work branch

The Go package `mobile` exposes the native TLS/WebSocket engine lifecycle, and CI builds an Android AAR for ARMv7, ARM64, x86, and x86_64. A Kotlin reflection adapter and the Hev tun2socks backend are present on this branch.

- `Engine.Start(profileJSON, protector)` refuses to start without a socket-protection callback.
- Outbound edge sockets use Go's `net.Dialer.Control`; Android must call `VpnService.protect(fd)` before each upstream connection proceeds. A failed protection callback aborts the connection.
- The generated binding API is checked in CI, including `Engine.start(String, SocketProtector)`, `addr()`, `health(long)`, `stop(long)`, and `SocketProtector.protect(long)`.
- Hev AAR 2.19.3 is checksum-pinned and its JNI class/method signatures are checked in CI.
- Kotlin lifecycle/fail-closed tests, Go CI, dependency security, Hev artifact validation, and Android Go mobile bridge build all passed for branch HEAD `7fb0929dbcc2f1d9bbcb122d3db81a6922e8087c`.

Latest verified workflow runs for that commit:
- Android Kotlin CI: https://github.com/AMINOPY/pg-core/actions/runs/40375780500
- Go CI: https://github.com/AMINOPY/pg-core/actions/runs/40375780542
- Android Go mobile bridge (AAR build): https://github.com/AMINOPY/pg-core/actions/runs/40375780422
- Hev Android artifact validation: https://github.com/AMINOPY/pg-core/actions/runs/40375780490
- Dependency Security: https://github.com/AMINOPY/pg-core/actions/runs/40375780369

The generated AAR artifact is available for a limited time from:
https://github.com/AMINOPY/pg-core/actions/runs/40375780422/artifacts/12674832491

## Not yet completed — do not call this a usable/production VPN

The repository currently contains an Android **library** module, not a consuming application that packages and installs both native AARs. The backend registry intentionally defaults to `UnavailableVpnBackend`; the Hev backend is not automatically registered. The Kotlin adapter also requires a host-provided end-to-end TUN probe. Therefore, the current branch is not a ready-to-install VPN app and should not be reported as fully executable.

Remaining release gates:

1. Integrate/package the generated Go AAR and checksum-pinned Hev AAR in the actual consuming Android app, including license notices and ABI checks.
2. Supply a real end-to-end probe that sends bounded TCP, UDP, and DNS test traffic through the TUN data plane; a TLS/WebSocket health check alone is insufficient.
3. Verify every upstream socket is protected with `VpnService.protect(fd)` before connect and fail closed if protection fails.
4. Run automated end-to-end tests for TCP/UDP/DNS, IPv4/IPv6, app filtering, cancellation, stop/start, network loss/recovery, and resource cleanup.
5. Run leak/soak tests on a physical Android device and retain device/ABI/log evidence.

No physical-device test has been performed by these CI workflows. Do not register the backend by default, merge it as production-complete, or show a connected state until these gates pass. Passing CI proves source checks and AAR generation, not leak-free real traffic forwarding.
