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
- [x] Define platform-neutral IPv4/IPv6 and routing policy contract (enforcement still pending)
- [ ] Verify system routes are restored after crash/stop
- [ ] Add controlled end-to-end health checks
- [ ] Define fail-closed behavior and test it on supported platforms

## Phase D — Additional engines
- [ ] Add Xray adapter behind the common contract
- [ ] Add Mihomo adapter where its rules/groups add value
- [ ] Evaluate Aether integration surface and license
- [ ] Reject incompatible combinations during validation

## Phase E — Transport resilience
- [x] Add strict validation contract for explicit HTTPS/WebSocket/gRPC transport profiles
- [x] Document customer-controlled CDN/TLS transport requirements and discovery design
- [ ] Implement signed `.well-known/pg-core/transport.json` discovery with expiry and rollback protection
- [ ] Implement transport dialer and bounded reconnect/health checks
- [ ] Test CDN provider compatibility and end-to-end TLS identity validation
- [ ] Add transport benchmarks before selecting a performance-sensitive implementation language

## Phase F — Quality
- [x] Add a fuzz target for strict config JSON decoding and schema validation
- [ ] Fuzz policy normalization and transport-profile validation
- [ ] Add benchmarks and long-running tests
- [ ] Add dependency and license scanning
- [ ] Publish support matrix by OS and engine version

## Non-goals
- Guaranteeing resistance to every DPI or filtering system
- Impersonating real third-party services
- Treating randomized TLS fingerprints as a universal solution
- Allowing multiple competing TUN/DNS owners in one profile
