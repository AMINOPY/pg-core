# Roadmap

This roadmap distinguishes implemented contracts from platform behavior that has not yet been verified. A green unit-test workflow is not evidence of a working system VPN on a real device.

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
- [x] Independent engine lifecycle with exclusive resource claims and regression tests

## Phase B — Engine adapters (initial implementation)
- [x] Implement sing-box CLI adapter behind the common lifecycle contract
- [x] Validate profiles using the upstream `sing-box check` command
- [x] Add private temporary config handling and size limits
- [x] Add process lifecycle and fake-binary unit tests
- [x] Implement an initial Xray subprocess adapter
- [ ] Pin supported sing-box and Xray versions and verify against real installed binaries
- [ ] Select and document supported OS/privilege models
- [ ] Add platform-specific integration tests for DNS/routes and rollback

## Phase C — Network correctness and platform enforcement
- [x] Define a validated, platform-neutral IPv4/IPv6, routing and DNS ownership policy contract
- [ ] Implement OS-specific policy enforcement on Android, Linux and Windows
- [ ] Define DNS ownership and leak-test strategy for each supported platform
- [ ] Verify system routes are restored after normal stop, crash and forced termination
- [ ] Add controlled end-to-end health checks through the active tunnel (not merely process liveness)
- [ ] Define fail-closed behavior and test it on supported platforms
- [ ] Add platform-specific TUN/VpnService integration and permission lifecycle tests
- [ ] Verify UDP behavior and DNS/IPv4/IPv6 leak prevention on real devices

## Phase D — Secure transport profile discovery
- [x] Validate explicit HTTPS/WebSocket/gRPC transport-profile fields and reject unsafe endpoint/path combinations
- [x] Add unit and fuzz tests for transport-profile validation
- [x] Verify Ed25519-signed manifests with host binding, expiry and monotonic sequence checks
- [x] Fetch well-known manifests over bounded, certificate-verified HTTPS and reject redirects
- [x] Add a persistent sequence store for rollback protection when the host uses it
- [x] Document customer-controlled CDN/TLS transport requirements and discovery design
- [ ] Provision and rotate trusted manifest keys safely on each host platform
- [ ] Add operator-side signed manifest publishing and rollback/recovery procedures
- [ ] Implement a transport dialer with bounded reconnect, cancellation and end-to-end health checks
- [ ] Test provider compatibility and end-to-end TLS identity on controlled endpoints

## Phase E — Health, performance and resilience
- [x] Add bounded HTTPS transfer diagnostics with timing breakdown, cancellation, redirect rejection and byte caps
- [x] Add a reproducible WebSocket framing/masking benchmark
- [ ] Add tunnel-level benchmarks on controlled endpoints and representative devices
- [ ] Measure throughput, latency, jitter, loss, reconnect time, CPU and memory before claiming speed improvements
- [ ] Add long-running soak tests, constrained-network tests and recovery tests
- [ ] Implement real data-plane health checks and controlled automatic failover
- [ ] Keep health diagnostics separate from endpoint throughput samples and ISP line-speed claims

## Phase F — Engine coexistence and host integrations
- [x] Reject duplicate declared exclusive resource claims in the fleet coordinator
- [ ] Define host-level ownership for TUN, routes, DNS, listeners and firewall state
- [ ] Add traffic-path isolation or a tested single-owner multiplexing design; lifecycle coordination alone is insufficient
- [ ] Validate every engine combination against capabilities and resource claims
- [ ] Complete Android Kotlin/JNI/VpnService integration and instrumentation tests
- [ ] Complete Linux and Windows adapters and end-to-end integration tests
- [ ] Publish a host-app integration example for Settings and Add Connection entry points

## Phase G — Quality and release engineering
- [x] Add fuzz targets for strict config decoding and deterministic policy selection
- [x] Add fuzz targets for network-policy and transport-profile validation
- [ ] Add dependency, license and supply-chain scanning
- [ ] Publish a support matrix by OS, architecture and engine version
- [ ] Define release signing, artifact provenance and upgrade/rollback behavior
- [ ] Protect the main branch with required checks and prevent force-push/deletion
- [ ] Define acceptance criteria and a release checklist for every supported platform

## Explicit non-goals

- Guaranteeing resistance to every DPI or filtering system
- Impersonating real third-party applications or services
- Treating randomized TLS fingerprints as a universal solution
- Allowing multiple competing TUN/DNS owners without an explicit, tested design
- Claiming a process is healthy merely because it remains alive
- Claiming a larger WebSocket frame or one HTTPS sample proves faster real-world tunnel throughput
