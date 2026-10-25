# Xray integration and low-latency tunnel design

## Product requirement: two-field, no-manual-config connection

The intended user experience is deliberately simple:

1. The user enters an **SNI hostname**.
2. The user enters one or more **CDN edge IPs** (or the configured edge addresses for the selected deployment).
3. The user presses **Connect**. Chimera validates inputs, selects a healthy candidate route, starts the configured engine, waits for an actual data-path health check, and only then reports Connected.

The user should not have to create or edit a JSON file, type a UUID, or manually assemble a VLESS/VMess/Trojan profile. Those are implementation details, not additional UI fields.

This is **no manual configuration**, not **no configuration exists anywhere**. Before the button can connect, the installation/deployment must have a trusted server profile containing the endpoint, protocol, authentication material, and transport requirements. That profile may be provisioned by the installer, an administrator, or a trusted app-managed profile store. Secrets must not be hard-coded into source code, committed to Git, exposed in the UI, or logged. If no usable profile or engine exists, show an explicit “engine/profile not configured” state rather than inventing credentials or reporting a false connection.

SNI and CDN edge IPs have distinct roles: SNI is a TLS name and an edge IP is a network destination. They do not, by themselves, identify an Xray user, protocol, origin, or authentication credentials. The implementation must use the selected deployment's known profile to map those two user inputs to a valid connection. Do not scan unrelated third-party hosts or imply that arbitrary SNI/IP combinations will work.

## Engine boundary

Chimera Core is the coordinator; Xray is a pluggable data-plane engine. The core must not pretend that its native WebSocket tunnel is Xray, and must not require Cloudflare. CDN/SNI is an optional transport/deployment choice, not the engine itself.

The existing \`engine.Adapter\` contract is the integration point:

- \`ValidateConfig(ctx, profile)\`: validate without starting processes or exposing secrets.
- \`Start(ctx, profile)\`: start the selected engine and report startup errors.
- \`Health(ctx)\`: test the active engine/data path, not merely whether a local process exists.
- \`Stop(ctx)\`: terminate child processes and local listeners and honor cancellation.
- \`Capabilities()\`: declare TUN, DNS, routing and health support truthfully.

The future Xray adapter should implement this contract in a separate package such as \`internal/engine/xray\`. It should invoke a user-installed or explicitly configured Xray binary (never silently download or execute an untrusted binary), build a temporary app-managed profile with restrictive permissions for any temporary files, avoid logging UUIDs/keys, wait for a local inbound listener, and stop the child process on cancellation. The adapter must not claim TUN support unless the platform-specific TUN integration exists.

The UI remains SNI + CDN edge IPs only. Engine choice and credentials belong to trusted app/deployment configuration, not extra mandatory user fields. If multiple trusted profiles exist, the app may select the intended profile through an installation setting or a safe internal rule; it must not guess authentication material.

## “Wormhole” routing analogy

A network tunnel can make two endpoints appear connected through a logical overlay, but it cannot bend physical space or bypass propagation delay. The practical equivalent is a reliable virtual path with measured route selection:

1. Measure candidate endpoints with bounded, cancellable probes.
2. Prefer the lowest-latency healthy candidate; use hysteresis so small jitter does not flap routes.
3. Reconnect/fail over when the active path fails.
4. Use protocol-supported multiplexing only when it helps the workload; excessive multiplexing can increase head-of-line blocking.
5. Keep congestion control, flow control, and bounded queues so load does not create unbounded latency.
6. Measure DNS, TCP, TLS, protocol handshake, time-to-first-byte, throughput, loss and reconnect time separately.
7. Never describe a TCP/TLS handshake as proof that application traffic is successfully tunneled. Test an authenticated request through the local proxy to a known endpoint.

There is no guaranteed “faster than direct” route. The target is lower measured latency and fewer failures than a baseline, not faster-than-light networking.

## Current implementation status

The current quick-connect implementation starts Chimera's **native TLS/WebSocket protocol**, not Xray. It requires the matching secret and a compatible server endpoint, and exposes a local SOCKS5 proxy. It does not yet implement an Xray adapter, TUN routing, system-wide VPN behavior, or a physical wormhole. Keep those limitations explicit until the corresponding code and end-to-end tests exist.
