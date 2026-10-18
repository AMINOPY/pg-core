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
- [ ] Define DNS ownership and leak-test strategy
- [ ] Define IPv4/IPv6 policy
- [ ] Verify system routes are restored after crash/stop
- [ ] Add controlled end-to-end health checks
- [ ] Define fail-closed behavior and test it on supported platforms

## Phase D — Additional engines
- [ ] Add Xray adapter behind the common contract
- [ ] Add Mihomo adapter where its rules/groups add value
- [ ] Evaluate Aether integration surface and license
- [ ] Reject incompatible combinations during validation

## Phase E — Quality
- [ ] Fuzz config parsing and policy normalization
- [ ] Add benchmarks and long-running tests
- [ ] Add dependency and license scanning
- [ ] Publish support matrix by OS and engine version

## Phase F — Mobile integration (API boundary implemented)
- [x] Add bounded, strict JSON API for failover policy in the `mobile` package
- [x] Document gomobile export command and host-side contract
- [x] Unit-test input validation, deterministic proposals, and confirm/cancel lifecycle
- [ ] Pin gomobile version and build the AAR in CI
- [ ] Add Android host `MethodChannel` implementation wired to the generated AAR
- [ ] Connect Dart adapter to the native bridge
- [ ] Add Android device tests for permission handling, failed switch, rollback, DNS/routes, and network changes

## Non-goals
- Guaranteeing resistance to every DPI or filtering system
- Impersonating real third-party services
- Treating randomized TLS fingerprints as a universal solution
- Allowing multiple competing TUN/DNS owners in one profile
