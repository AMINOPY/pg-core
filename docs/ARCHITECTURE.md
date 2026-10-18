# Architecture

## Design principles

1. One owner per system resource. A profile chooses one active data-plane engine. TUN and DNS ownership are explicit.
2. Separate control plane from data plane. The coordinator validates profiles and observes lifecycle/health; an adapter owns engine integration.
3. Version adapters. Each integration declares supported versions, capabilities, config validation, startup/stop behavior, and tests.
4. Fail closed when policy requires it. If a profile requires a tunnel, do not silently fall back to direct networking after engine failure.
5. Do not confuse process-alive with end-to-end connectivity. Health checks must be bounded, cancellable, and documented.
6. Never include credentials, private keys, tokens, or raw secret-bearing configurations in normal logs.

## Components

- internal/config: strict JSON loading, rejection of unknown fields, and structural validation.
- internal/engine: adapter contract and common health model.
- internal/orchestrator: serialized engine lifecycle, health refresh, and bounded cleanup after startup failures.
- internal/policy: deterministic candidate selection that prefers healthy engines, then lower measured latency, and uses degraded engines only as fallback.
- internal/malwarescan: bounded SHA-256 signature scanning for explicitly authorized directories, with optional quarantine and cancellable periodic scheduling.
- cmd/chimera: validation-only CLI in this scaffold.

## Engine roles

- Xray-core: optional protocol engine where the selected configuration is supported.
- sing-box: optional all-in-one engine when its TUN, DNS, and routing features fit.
- Mihomo: optional policy/group engine when its proxy-group and rules model fits.
- Aether: evaluate exact repository version, license, API, and whether path-management features can be reused outside its client.

These are alternative integrations, not four mandatory stages in one packet pipeline. Routing semantics and configuration formats differ; the coordinator must not assume direct compatibility.

## TLS and traffic testing

Use maintained TLS libraries, correct certificate validation, and documented configuration. No fingerprint guarantees acceptance by every server or network. Traffic generation belongs in an isolated test environment and should use synthetic workloads rather than impersonating named third-party apps.

## Lifecycle state machine

stopped -> starting -> healthy / degraded / failed -> stopping -> stopped

- Start: validate configuration and capabilities before changing system networking.
- Failure: report a stable error code and cancel dependent work.
- Stop: release resources and restore system state.
- Recovery: bounded retries with backoff; never create unbounded reconnect loops.

## Current status

Implemented: strict config loading/validation, engine adapter contract, lifecycle coordinator, deterministic candidate selection, bounded signature-based malware scanning/quarantine for authorized directories, unit tests, and CI workflow. Not implemented: TUN integration, DNS proxy, OS route management, VPN service, real health probes, or third-party engine adapters.
