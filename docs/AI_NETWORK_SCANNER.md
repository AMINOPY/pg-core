# AI Network Diagnostics

The `AI` module is a local, explainable diagnostic analyzer. It is deliberately not marketed as a trained model: current findings come from deterministic rules and include confidence estimates, reasons, and safe next steps. It does not upload reports or grant connector permissions.

## What the feature can analyze

The Go analyzer accepts bounded measurements supplied by the host app:

- Download/upload throughput in Mbps, latency, jitter, packet-loss percentage, and DNS latency.
- Current transport (Wi-Fi, cellular, Ethernet, or unknown).
- Optional cellular generation, carrier label, roaming state, SIM availability state, and signal strength where the platform exposes them with appropriate consent.
- Up to 64 endpoint-check summaries for endpoints the user or app explicitly configured.

It returns a 0–100 score, grade, findings, confidence estimates, recommendations, and limitations. It rejects unknown JSON fields, invalid ranges, NaN/infinity, and oversized input.

## Speed-test engine integration

Use one of these projects as a separately deployed or separately packaged measurement engine; pass its numeric results into `ChimeraNetworkReport`:

- [LibreSpeed](https://github.com/librespeed/speedtest) — self-hosted speed test with HTML5 client/server options; upstream is LGPL-3.0-or-later.
- [OpenSpeedTest](https://github.com/openspeedtest/Speed-Test) — self-hosted web speed test; upstream repository states MIT licensing.
- [Network Checker](https://github.com/mirarr-app/network-checker) — a separate Flutter application with network diagnostic tools, licensed GPL-3.0. This project does not copy its code or incorporate its DPI-evasion/edge-scanning features. Review GPL obligations before linking or redistributing any of its code.

For production, deploy a trusted test endpoint you control or explicitly approve. Do not run repeated tests automatically in the background: speed tests can consume mobile data, battery, and server bandwidth. Show the user estimated data usage and let them start/stop tests. Run download/upload tests only on demand, with bounded duration and payload size.

The Go core does **not** download large files, generate traffic, or automatically choose a third-party test server. A speed-test engine must provide the measurements. This avoids hidden data usage and makes server choice visible to the user.

## Flutter / Android bridge

The Dart API is `integrations/flutter_v2ray/lib/chimera_ai.dart`:

```dart
final ai = ChimeraAI();
final result = await ai.analyze(const ChimeraNetworkReport(
  networkType: 'cellular',
  cellularGeneration: '4g',
  downloadMbps: 24.3,
  uploadMbps: 4.8,
  latencyMs: 48,
  jitterMs: 7,
  packetLossPct: 0,
  dnsLatencyMs: 35,
));
```

The native MethodChannel exposes `analyzeNetwork`. The host app must implement `ChimeraGoAIAnalyzer` against the generated gomobile AAR, mapping it to `mobile.AnalyzeNetworkJSON`. This repository still does not include a complete Flutter host application or generated AAR, so the native-to-Go adapter must be completed and built in the consuming app.

## SIM and privacy boundaries

- Collect cellular metadata only when the user understands why it is needed and grants any required platform permission.
- The core accepts only a carrier display label, broad radio generation, roaming flag, optional signal strength, and a coarse SIM state.
- Never send IMSI, ICCID, SIM serial number, phone number, device identifiers, precise location, credentials, or raw connection profiles to the analyzer.
- Do not infer the user's exact plan, account, location, or real-world identity from carrier metadata.
- Reports are analyzed locally by this package; persistence and upload are not implemented.

## Scope and limitations

This is an explainable diagnostic baseline, not a trained AI model and not proof that an ISP is blocking a service. A failed endpoint can reflect server outage, DNS, routing, TLS, captive portals, or transient congestion. It performs no IP-range scan, traffic impersonation, SNI spoofing, or DPI bypass. The app should report only what its measurements support.

## Validation checklist

- Unit tests for valid reports, invalid metrics, unknown fields, oversized input, and missing optional SIM data.
- Run `gofmt -w internal/ai/*.go mobile/ai*.go`, `go test ./...`, and `go test -race ./...`.
- Add Flutter unit tests for JSON encoding/decoding.
- Build the Kotlin host against the exact generated AAR and Flutter embedding.
- On real devices, test Wi-Fi, cellular, airplane mode, permission denied/revoked, roaming metadata, and network changes.
- Verify the UI explains test data usage and never silently starts a speed test.
