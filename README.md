# Chimera Core (pg-core)

**Status: early scaffold / not production-ready.**

Chimera Core is a proposed modular network-engine coordinator. The current Go code includes strict configuration validation, an engine adapter contract, a serialized lifecycle manager, deterministic health-aware candidate selection, unit tests, and CI. It does **not** yet embed Xray, sing-box, Mihomo, or Aether, and it is not a working VPN client.

## Current implementation

- Strict JSON parsing with unknown-field and trailing-value rejection
- Config validation for supported engine IDs and active-engine selection
- Common engine lifecycle/health interface
- Start validation, health check, failure cleanup, and serialized stop/start operations
- Deterministic selection: healthy candidates first, lowest measured latency next, degraded candidates only as fallback
- Go unit tests and GitHub Actions workflow

## Quick start

Requires Go 1.22 or newer.

    go test ./...
    go vet ./...
    go run ./cmd/chimera
    go run ./cmd/chimera --config config.example.json

The CLI validates configuration only. It does not start a network engine, configure TUN, modify system routes, or provide a tunnel.

## Planned adapters

| Engine | Intended role | Status |
|---|---|---|
| Xray-core | Protocol engine behind an adapter | Planned; not connected |
| sing-box | Optional engine for profiles needing its TUN/DNS/routing capabilities | Planned; not connected |
| Mihomo | Optional policy/group engine where its configuration model fits | Planned; not connected |
| Aether | Evaluate client-side path-management behavior and licensing/API constraints | Research only |

These projects overlap in routing, DNS, and tunnel ownership. They must not all control the same traffic path simultaneously without an explicit, tested design.

## Architecture and security

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md), [docs/ROADMAP.md](docs/ROADMAP.md), and [SECURITY.md](SECURITY.md).

The project focuses on reliability, interoperability, secure configuration, and testability. No implementation can guarantee connectivity on every network or defeat every DPI system. TLS fingerprint rotation and imitation of named third-party apps are not implemented. Synthetic traffic may be used in controlled performance tests, not to impersonate real services.

## Development

    gofmt -w .
    go vet ./...
    go test ./...

Contributions should include tests and document supported platforms and engine versions.