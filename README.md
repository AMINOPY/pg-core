# Chimera Core (pg-core)

**Status: early scaffold / not production-ready.**

Chimera Core is a proposed modular network-engine coordinator. This repository provides the foundation: strict configuration validation, engine capability contracts, health-report types, documentation, and CI. It does **not** yet embed or merge Xray, sing-box, Mihomo, or Aether, and it is not a working VPN client.

## Goals

- Keep engine integrations behind explicit adapters.
- Give each profile one clearly designated owner for system TUN and DNS.
- Validate configuration before starting an engine.
- Expose health reports and predictable lifecycle operations.
- Test routing policy and recovery logic independently.
- Keep secrets out of logs and diagnostic output.

## Quick start

Requires Go 1.22 or newer.

    go test ./...
    go run ./cmd/chimera
    go run ./cmd/chimera --config config.example.json

The CLI currently validates configuration and reports scaffold status. It does not start network tunnels.

## Planned adapters

| Engine | Intended role | Status |
|---|---|---|
| Xray-core | Protocol engine through a maintained integration boundary | Planned; not connected |
| sing-box | Optional engine for profiles needing its TUN/DNS/routing capabilities | Planned; not connected |
| Mihomo | Optional policy/group engine where its configuration model fits | Planned; not connected |
| Aether | Evaluate client-side path-management behavior and licensing/API constraints | Research only |

These projects overlap in routing, DNS, and tunnel ownership. They must not all control the same traffic path simultaneously without an explicit, tested design.

## Scope and limitations

The project focuses on reliability, interoperability, secure configuration, and testability. No implementation can guarantee that a connection will work on every network or defeat every DPI system. TLS fingerprint rotation and imitation of named third-party apps are not implemented. Synthetic traffic may be used in controlled performance tests, not to impersonate real services.

## Development

    gofmt -w .
    go vet ./...
    go test ./...

Contributions should include tests and document supported platforms and engine versions.