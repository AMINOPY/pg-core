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

## Phase B — First engine adapter (portable subprocess adapter implemented)
- [x] Implement sing-box CLI adapter behind the common lifecycle contract
- [x] Validate profiles using the upstream `sing-box check` command
- [x] Add private temporary config handling and size limits
- [x] Add process lifecycle and fake-binary unit tests
- [x] Document process-level health semantics and limitations
- [ ] Pin supported sing-box versions and verify against a real installed binary
- [ ] Select and document a supported OS/privilege model
- [ ] Add real-engine integration test for the pinned release

## Phase C — Android and network correctness
- [x] Document Android `VpnService`/TUN ownership, DNS validation, and stop/revocation rollback requirements
- [ ] Add a separate Android app module using the matching upstream Android/libbox API
- [ ] Add Android emulator/device tests for VPN consent, TUN, DNS, IPv4/IPv6 routes, service revocation, and restored connectivity
- [ ] Define DNS ownership and leak-test strategy
- [ ] Define IPv4/IPv6 policy
- [ ] Verify routes are restored after crash/stop on each supported platform
- [ ] Add controlled end-to-end health checks
- [ ] Define fail-closed behavior and test it on supported platforms

## Phase D — Smart auto-failover (policy implemented; lifecycle integration pending)
- [x] Deterministic candidate ranking across supplied subscriptions
- [x] Treat negative/sentinel `-1` latency as invalid, not fastest
- [x] Consecutive-failure threshold, latency threshold, cooldown, and minimum improvement policy
- [x] Unit tests for cross-subscription selection, no fallback, high latency, and cooldown
- [ ] Wire probe collection and active-engine switching into the app/service lifecycle
- [ ] Verify replacement engine before releasing active VPN ownership
- [ ] Add Android device tests for failover, failed replacement rollback, DNS, IPv4/IPv6, and recovery

## Phase E — Additional engines and quality
- [ ] Add Xray adapter behind the common contract
- [ ] Add Mihomo adapter where its rules/groups add value
- [ ] Evaluate Aether integration surface and license
- [ ] Fuzz config parsing and policy normalization
- [ ] Add benchmarks and long-running tests
- [ ] Add dependency and license scanning
- [ ] Publish support matrix by OS and engine version

## Non-goals
- Guaranteeing resistance to every DPI or filtering system
- Impersonating real third-party services
- Treating randomized TLS fingerprints as a universal solution
- Allowing multiple competing TUN/DNS owners in one profile
