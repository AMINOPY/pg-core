# Flutter V2Ray integration boundary

Reference project: [blueboy-tm/flutter_v2ray](https://github.com/blueboy-tm/flutter_v2ray).

The referenced project is a Flutter plugin, not a Go package. Its README describes V2Ray/Xray proxy and VPN mode, share-link parsing, server-delay checks, and platform-specific VPN handling; it reports Xray 25.3.6 for Android. See the upstream [README](https://github.com/blueboy-tm/flutter_v2ray) and [pub.dev package page](https://pub.dev/packages/flutter_v2ray). Verify the upstream version, license, supported ABIs, and Android requirements at integration time.

## Recommended responsibility split

- **Flutter application:** screens, user consent, subscription/profile UI, and platform permissions.
- **`flutter_v2ray` plugin:** its supported Flutter API and its bundled Xray/VPN lifecycle on platforms where the plugin actually supports it.
- **Chimera Core (Go):** engine-neutral configuration validation, candidate/health policy, failover decisions, and structured events. It does not directly import Dart packages.
- **Exactly one VPN/TUN owner:** when using `flutter_v2ray` in Android VPN mode, do not also start Chimera's sing-box adapter or another TUN owner on the same device traffic path.

## Integration options

1. **Flutter app uses the plugin directly:** quickest path for a Flutter UI, but the plugin remains responsible for its own Xray connection lifecycle. Chimera's Go failover policy is not automatically wired into that plugin.
2. **A platform bridge:** define a narrow Dart-facing API for importing a profile, checking a candidate, starting/stopping a connection, and receiving state/health events. Implement and test the bridge in the Flutter/Android application. Do not call a Go subprocess to manipulate Android `VpnService` or TUN descriptors.
3. **Native core ownership:** if Chimera is to own Android VPN lifecycle, integrate an appropriate native Xray/libbox binding behind a single `VpnService`; do not treat the Flutter plugin as a reusable Go engine. This is a separate implementation and needs device-level tests.

## Mapping to the current core

| Flutter/plugin capability | Chimera status |
|---|---|
| Parse VMess/VLESS/etc. share links | Not implemented in Go core |
| Import and normalize subscriptions | Not implemented |
| Measure candidate latency | Policy accepts measurements, but no recurring probe scheduler is wired |
| Decide cross-subscription failover | Policy available in `internal/policy/failover.go`; execution must be confirmed by the caller |
| Start/stop Xray through `flutter_v2ray` | Not integrated |
| Own Android `VpnService`, TUN, DNS and routes | Not implemented by Chimera |
| Verify connectivity and roll back failed switches | Platform integration remains |

## Integration requirements

- Keep credentials and share links out of logs; use redaction and secure storage.
- Treat missing or negative latency (including `-1`) as unavailable, not as zero latency.
- Apply bounded timeouts and concurrency to health checks.
- Confirm a failover only after the replacement is running and DNS/end-to-end connectivity is verified; cancel the proposal if startup fails.
- Serialize start/stop and permission changes. Test VPN permission revocation, process death, IPv4/IPv6, DNS behavior, and route restoration on physical Android devices.
- Check upstream license and dependency/native-binary compatibility before redistributing. Do not assume that the plugin's platform support or commercial platform packages have identical terms.

This document records an integration design, not a claim that Flutter or Android support is already implemented in Chimera.
