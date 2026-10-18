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
- [x] Validate profiles using the upstream sing-box check command
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

## Phase F — Mobile integration (bridge source implemented; app wiring incomplete)
- [x] Add bounded, strict JSON API for failover policy in the mobile package
- [x] Document gomobile export command and host-side contract
- [x] Unit-test input validation, deterministic proposals, and confirm/cancel lifecycle
- [x] Add Kotlin MethodChannel host source for initialize/evaluate/confirm/cancel
- [x] Add Dart MethodChannel client and serialized plugin failover coordinator source
- [ ] Implement the app-specific ChimeraGoCoreFactory against a pinned generated gomobile AAR
- [ ] Add a Flutter/Android host app and build the AAR in CI
- [ ] Run Dart analysis/tests and compile Kotlin against the generated AAR
- [ ] Add Android device tests for permission handling, failed switch, rollback, DNS/routes, and network changes

## Phase G — Adaptive connector foundation (in progress)
- [x] Add a bounded, strict capability manifest contract
- [x] Reject unknown manifest fields, duplicate capability IDs, malformed schemas, and oversized input
- [x] Document connector API proposal and trust boundaries
- [x] Document automated learning pipeline and release gates
- [x] Add authenticated, size-bounded HTTP manifest-validation endpoint
- [x] Add exact-origin CORS allowlist, HTTP timeouts, security headers, and API tests
- [x] Add server-side Node.js/Python integration examples and a sample manifest
- [ ] Implement connector identity and scoped authorization
- [ ] Implement capability discovery over a real transport
- [ ] Implement policy-checked operation execution
- [ ] Build disposable, OS-isolated adapter generation and test workers
- [ ] Add signed package metadata, staged activation, revocation, and rollback
- [ ] Add server/device enrollment and independently enforced local policy
- [ ] Implement HTTP/MCP connector discovery and policy-checked operation APIs with contract tests

## Phase H — Transport resilience and DPI testing
- [x] Document DPI resilience goals and limitations
- [ ] Pin and integrate a maintained, supported transport adapter
- [ ] Define transport profiles and policy-controlled selection
- [ ] Add TLS validation, DNS/IPv4/IPv6 leak, fail-closed, and rollback tests
- [ ] Test supported configurations on real devices and controlled networks
- [ ] Publish tested compatibility matrix and known limitations

## Non-goals
- Guaranteeing resistance to every DPI or filtering system
- Impersonating real third-party services
- Treating randomized TLS fingerprints as a universal solution
- Allowing multiple competing TUN/DNS owners in one profile
- Executing generated or discovered code inside the trusted core process
