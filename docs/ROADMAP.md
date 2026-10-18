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

## Phase B — First engine adapter (portable subprocess adapter implemented; Android integration boundary documented)
- [x] Implement sing-box CLI adapter behind the common lifecycle contract
- [x] Validate profiles using the upstream `sing-box check` command
- [x] Add private temporary config handling and size limits
- [x] Add process lifecycle and fake-binary unit tests
- [x] Test unexpected process exit and context-aware health checks
- [x] Document process-level health semantics and limitations
- [x] Document Android `VpnService`/TUN ownership, DNS validation, and stop/revocation rollback requirements
- [ ] Pin supported sing-box versions and verify against a real installed binary
- [ ] Add real-engine integration test for the pinned release
- [ ] Add a separate Android app module using the matching upstream Android/libbox API
- [ ] Add Android emulator/device tests for VPN consent, TUN establishment, DNS, IPv4/IPv6 routes, service revocation, and connectivity restoration

## Phase C — Network correctness
- [ ] Define DNS ownership and leak-test strategy
- [ ] Define IPv4/IPv6 policy
- [ ] Verify system routes are restored after crash/stop on each supported platform
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

## Non-goals
- Guaranteeing resistance to every DPI or filtering system
- Impersonating real third-party services
- Treating randomized TLS fingerprints as a universal solution
- Allowing multiple competing TUN/DNS owners in one profile
