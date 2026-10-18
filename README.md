# Chimera Core (pg-core)

**Status: early scaffold / not production-ready.**

Chimera Core is a proposed modular network-engine coordinator. The current Go code includes strict configuration validation, an engine adapter contract, a serialized lifecycle manager, deterministic health-aware candidate selection, bounded network diagnostics, unit tests, and CI. It does **not** yet embed Xray, sing-box, Mihomo, or Aether, and it is not a working VPN client.

## Current implementation

- Strict JSON parsing with unknown-field and trailing-value rejection
- Config validation for supported engine IDs and active-engine selection
- Common engine lifecycle/health interface
- Start validation, health check, failure cleanup, and serialized stop/start operations
- Deterministic selection: healthy candidates first, lowest measured latency next, degraded candidates only as fallback
- Network diagnostics package: hostname validation, DNS lookup latency, HTTPS reachability/status, bounded concurrent batch checks, per-check timeouts
- Go unit tests and GitHub Actions workflow

## Core foundation additions (Phase A)

- Adapter registry with deterministic listing and capability negotiation before start
- Configuration schema version 1; legacy unversioned files normalize to v1 in memory, unknown future versions are rejected
- Structured JSON logging with defensive redaction for common secret-bearing attribute names
- Bounded in-process lifecycle event bus; optional orchestrator integration via `NewWithEvents`
- CLI commands for config validation, bounded hostname diagnostics, known engine IDs, and local status

Examples:

    go run ./cmd/chimera --list-engines
    go run ./cmd/chimera --status
    go run ./cmd/chimera --diagnose example.com

The engine list reports known IDs, not available adapters. The status command reports local CLI state only; there is no daemon or active tunnel. Diagnostics perform DNS and HTTPS checks for the hostname explicitly supplied by the operator.

## Quick start

Requires Go 1.22 or newer.

    go test ./...
    go vet ./...
    go run ./cmd/chimera
    go run ./cmd/chimera --config config.example.json

The CLI can validate configuration and run bounded DNS/HTTPS diagnostics, but it does not start a network engine, configure TUN, modify system routes, or provide a tunnel. No GUI or background status daemon is included.

## Network diagnostics API

See [docs/NETWORK_DIAGNOSTICS.md](docs/NETWORK_DIAGNOSTICS.md). A minimal example:

    checker := diagnostics.Checker{Timeout: 4 * time.Second, Workers: 4}
    results := checker.CheckMany(ctx, []string{"example.com", "example.org"})

Checks are limited to explicitly supplied hostnames and use the system DNS resolver plus certificate-verified HTTPS. They do not perform IP-range scanning or impersonate third-party services.

## Planned adapters

| Engine | Intended role | Status |
|---|---|---|
| Xray-core | Protocol engine behind an adapter | Planned; not connected |
| sing-box | Optional engine for profiles needing its TUN/DNS/routing capabilities | Planned; not connected |
| Mihomo | Optional policy/group engine where its configuration model fits | Planned; not connected |
| Aether | Evaluate client-side path-management behavior and licensing/API constraints | Research only |

These projects overlap in routing, DNS, and tunnel ownership. They must not all control the same traffic path simultaneously without an explicit, tested design.

## Architecture and security

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md), [docs/ROADMAP.md](docs/ROADMAP.md), [docs/NETWORK_DIAGNOSTICS.md](docs/NETWORK_DIAGNOSTICS.md), and [SECURITY.md](SECURITY.md).

The project focuses on reliability, interoperability, secure configuration, and testability. No implementation can guarantee connectivity on every network or defeat every DPI system. TLS fingerprint rotation and imitation of named third-party apps are not implemented. Synthetic traffic may be used in controlled performance tests, not to impersonate real services.

## Development

    gofmt -w .
    go vet ./...
    go test ./...

Contributions should include tests and document supported platforms and engine versions.
