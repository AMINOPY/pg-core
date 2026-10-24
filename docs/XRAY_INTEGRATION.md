# Xray integration and low-latency tunnel design

## Product boundary

Chimera Core is the coordinator; Xray is a pluggable data-plane engine. The core must not pretend that its native WebSocket tunnel is Xray, and must not require Cloudflare. A user may run an authorized Xray server on a VPS or another supported host. CDN/SNI is an optional transport/deployment choice, not the engine itself.

The existing `engine.Adapter` contract is the integration point:

- `ValidateConfig(ctx, profile)`: validate without starting processes or exposing secrets.
- `Start(ctx, profile)`: start the selected engine and report startup errors.
- `Health(ctx)`: test the active engine/data path, not merely whether a local process exists.
- `Stop(ctx)`: terminate child processes and local listeners and honor cancellation.
- `Capabilities()`: declare TUN, DNS, routing and health support truthfully.

The Xray adapter should implement this contract in a separate package such as `internal/engine/xray`. It should invoke a user-installed or explicitly configured Xray binary (never silently download or execute an untrusted binary), write a temporary least-privilege configuration with restrictive file permissions, avoid logging UUIDs/keys, wait for a local inbound listener, and stop the child process on cancellation. The adapter must not claim TUN support unless the platform-specific TUN integration exists.

## No-config user experience

“No config file” means the UI builds a validated in-memory profile or imports a share link; it does not mean a connection can be inferred from only SNI and an IP address. An Xray connection generally needs a real server endpoint, protocol, and authentication material. SNI only selects the TLS name. Do not invent credentials or auto-scan third-party servers.

Recommended compact UI:
1. **Server link / profile** — accept a user-provided VLESS/VMess/Trojan share link or a server profile from a trusted source.
2. **SNI override (optional)** — an override only when the server configuration supports it.

If the product requirement strictly demands exactly two plain fields, the server must be pre-provisioned in the app; the two fields can then be SNI and a server/edge address. The UI should display which engine is active and explain when Xray is not installed/configured rather than falling back silently to the native engine.

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

The quick-connect PR currently starts Chimera's **native TLS/WebSocket protocol**, not Xray. It requires the matching secret and a compatible server endpoint, and exposes a local SOCKS5 proxy. It does not implement an Xray adapter, TUN routing, system-wide VPN behavior, or a physical wormhole. Those claims must remain explicit until corresponding code and end-to-end tests exist.
