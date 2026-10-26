# Ghost, Ghost Plus, and Ghost Ultra Plus

## Ghost (Go)

Ghost performs a bounded set of DNS/HTTPS observations for one hostname explicitly supplied by the user. It summarizes symptoms and recommendations; it does not establish ground truth for DPI.

## Ghost Plus (C)

The C API in native/ghost_plus.h consumes caller-supplied observations and returns a conservative classification. It does not open sockets, capture packets, scan hosts, modify routes, or perform a bypass. Samples are capped at 20. Its confidence field is a heuristic classification score, not a calibrated probability that DPI exists.

## Ghost Ultra Plus (Kotlin + Python)

- android/src/main/kotlin/dev/pgcore/GhostUltraPlus.kt provides a typed Kotlin analysis facade with English/Persian output.
- tools/ghost_ultra_plus/ghost_report.py formats a Ghost JSON report in English (en) or Persian (fa), without network access.
- Example: python tools/ghost_ultra_plus/ghost_report.py fa < report.json

## Validation

The C unit test uses strict C11 compiler flags. Python tests use unittest; Kotlin tests run in the Android Kotlin CI workflow.

## Boundaries

A timeout, TLS error, reset, DNS failure, or HTTP 403/451 does not by itself prove DPI. These modules are diagnostic and reporting layers. They do not promise to defeat filtering, imitate third-party services, intercept unrelated traffic, or change system networking. A real transport fallback requires a separately reviewed implementation, user consent, and end-to-end verification on each supported platform.
