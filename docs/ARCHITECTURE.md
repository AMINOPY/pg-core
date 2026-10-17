# Architecture

## Design principles

1. One owner per system resource. A profile chooses one active data-plane engine. TUN and DNS ownership are explicit.
2. Separate control plane from data plane. The coordinator validates profiles and observes lifecycle/health; an adapter owns engine integration.
3. Version adapters. Each integration declares supported versions, capabilities, config validation, startup/stop behavior, and tests.
4. Fail closed when policy requires it. If a profile requires a tunnel, do not silently fall back to direct networking after engine failure.
5. Do not confuse process-alive with end-to-end connectivity. Health checks must be bounded, cancellable, and documented.
6. Never include credentials, private keys, tokens, or raw secret-bearing configurations in normal logs.

## Components

- internal/config: strict JSON loading and structural validation.
- internal/engine: adapter contract and common health model.
- cmd/chimera: validation-only CLI in this scaffold.
- Future internal/orchestrator: lifecycle manager, active-engine selection, bounded retries, and state transitions.
- Future internal/policy: normalized policy with explicit precedence.
- Future internal/telemetry: privacy-preserving metrics and redacted diagnostics.

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

Only config loading/validation and the adapter contract are implemented. There is no TUN integration, DNS proxy, routing engine, VPN service, real health probe, or third-party engine adapter yet.
