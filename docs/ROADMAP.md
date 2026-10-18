# Roadmap

## Phase 0 — Repository foundation (implemented)
- [x] Go module and CLI entry point
- [x] Strict configuration parsing and validation
- [x] Engine adapter and health-report contracts
- [x] Unit tests for configuration validation
- [x] CI workflow and security notes
- [x] Serialized lifecycle coordinator with startup validation and cleanup
- [x] Unit tests for coordinator startup, health, and stop behavior

## Phase 1 — Coordinator hardening
- [ ] Add adapter registry and capability negotiation
- [ ] Add profile schema versioning and migration
- [ ] Add redacted structured logging
- [ ] Add fuzz tests for config parsing
- [ ] Improve cancellation and cleanup behavior for adapters that ignore context

## Phase 2 — First real engine integration
- [ ] Select one platform and one engine as sole data-plane owner
- [ ] Check upstream license, supported API, and version pinning
- [ ] Implement startup, health, shutdown, and rollback
- [ ] Add integration tests

## Phase 3 — Network correctness
- [ ] Define DNS ownership and leak-test strategy
- [ ] Define IPv4/IPv6 policy
- [ ] Verify system routes are restored after crash/stop
- [ ] Add controlled end-to-end health checks

## Phase 4 — Additional engines
- [ ] Add Xray adapter behind the common contract
- [ ] Add sing-box adapter or document it as the selected all-in-one engine
- [ ] Add Mihomo adapter where its rules/groups add value
- [ ] Evaluate Aether integration surface and license
- [ ] Reject incompatible combinations during validation

## Phase 5 — Quality
- [ ] Fuzz config parsing and policy normalization
- [ ] Add benchmarks and long-running tests
- [ ] Add dependency and license scanning
- [ ] Publish support matrix by OS and engine version

## Non-goals
- Guaranteeing resistance to every DPI or filtering system
- Impersonating real third-party services
- Treating randomized TLS fingerprints as a universal solution
- Allowing multiple competing TUN/DNS owners in one profile
