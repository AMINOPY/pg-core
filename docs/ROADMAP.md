# Roadmap

## Phase 0 — Repository foundation (implemented)
- [x] Go module and CLI entry point
- [x] Strict configuration parsing and validation
- [x] Engine adapter and health-report contracts
- [x] Unit tests for configuration validation
- [x] CI workflow and security notes
- [x] Serialized lifecycle coordinator with startup validation and cleanup
- [x] Unit tests for coordinator startup, health, and stop behavior

## Phase A — Coordinator hardening (implemented)
- [x] Adapter registry and capability negotiation
- [x] Profile schema versioning and migration
- [x] Redacted structured logging
- [x] Bounded lifecycle event bus and CLI diagnostics/status

## Phase B — First engine adapter (initial implementation)
- [x] Implement sing-box CLI adapter behind the common lifecycle contract
- [x] Validate profiles using the upstream `sing-box check` command
- [x] Add private temporary config handling and size limits
- [x] Add process lifecycle and fake-binary unit tests
- [x] Document process-level health semantics and limitations
- [ ] Pin supported sing-box versions and verify against a real installed binary
- [ ] Select and document one supported OS/privilege model
- [ ] Add platform-specific integration tests for DNS/routes and rollback

## Phase C — Network correctness
- [x] Add bounded HTTPS transfer diagnostics with timing breakdown and byte caps
- [ ] Define DNS ownership and leak-test strategy
- [ ] Define IPv4/IPv6 policy
- [ ] Verify system routes are restored after crash/stop
- [ ] Add controlled end-to-end health checks through the active tunnel
- [ ] Define fail-closed behavior and test it on supported platforms
- [ ] Measure reconnect, packet loss, latency, and throughput on controlled endpoints

## Phase D — Additional engines
- [ ] Add Xray adapter behind the common contract
- [ ] Add Mihomo adapter where its rules/groups add value
- [ ] Evaluate Aether integration surface and license
- [ ] Reject incompatible combinations during validation

## Phase E — Quality
- [x] Add a fuzz target for strict config JSON decoding and schema validation
- [ ] Fuzz policy normalization
- [ ] Add benchmarks and long-running tests
- [ ] Add dependency and license scanning
- [ ] Publish support matrix by OS and engine version
- [ ] Compile-check Linux, Windows, and Android targets in CI

## DPI resistance and performance
- [ ] Define a documented, user-configurable transport strategy supported by the selected engine
- [ ] Add controlled tests for transport compatibility, latency, packet loss, and reconnect behavior
- [ ] Test IPv4/IPv6, DNS behavior, MTU, and mobile-network transitions
- [ ] Document that no single obfuscation or transport strategy defeats all DPI systems
- [ ] Use benchmark data to inform endpoint choice; do not claim automatic speed improvements without measurements

## Non-goals
- Guaranteeing resistance to every DPI or filtering system
- Impersonating real third-party services
- Treating randomized TLS fingerprints as a universal solution
- Allowing multiple competing TUN/DNS owners in one profile
