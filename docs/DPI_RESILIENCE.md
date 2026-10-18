# Transport Resilience and DPI Considerations

## Current reality

The repository's architecture explicitly states that it does not yet implement a TUN integration, DNS proxy, OS route management, VPN service, or complete third-party engine adapters. Consequently, this change does not add a working DPI-bypass transport and must not be advertised as doing so.

Deep Packet Inspection (DPI) behavior differs by network, protocol, and operator. No single fingerprint, TLS setting, proxy protocol, or transport can guarantee reachability or defeat every filtering system.

## Proposed architecture

Keep transport selection separate from the policy core. Each transport adapter should declare:
- supported platforms and upstream versions;
- protocol and transport mode;
- configuration validation and safe defaults;
- TLS certificate-validation behavior;
- expected permissions and resource limits;
- active health-check semantics and known failure modes;
- fallback rules and whether fallback could leak traffic outside a required tunnel.

A future transport selector may choose among explicitly configured, supported transports based on reachability measurements and user policy. It must not silently weaken TLS validation, expose traffic directly when a tunnel is required, or claim that randomized fingerprints provide anonymity.

## Privacy and resilience requirements

- Prefer maintained upstream implementations and their documented transport modes over custom cryptographic or TLS implementations.
- Keep certificate verification enabled.
- Make DNS, IPv4, IPv6, route ownership, and failure behavior explicit.
- Bound probes and retries; probe only endpoints the operator is authorized to test.
- Do not impersonate a named third-party application or service.
- Explain to users which transport is active and what metadata may remain visible to network operators.
- Test in a controlled lab and on supported devices before enabling unattended switching.

## Acceptance tests for a future transport adapter

- Invalid certificates and hostname mismatches fail closed.
- Required-tunnel mode has no direct-network fallback.
- DNS and IPv4/IPv6 leak tests pass on each supported OS.
- Crash, cancellation, permission denial, and network changes restore system state.
- Health probes are bounded, cancellable, and do not log credentials or raw connection profiles.
- The adapter's documented support matrix matches tests against pinned upstream versions.

This document is an engineering plan, not evidence that DPI circumvention is implemented or guaranteed.
