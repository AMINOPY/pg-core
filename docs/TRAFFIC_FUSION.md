# Traffic fusion: flow routing and allocator boundary

## What “fusion” means in this repository

Several independent VPN/proxy engines running at once is not, by itself, traffic fusion. True packet-level bonding needs a common data plane, sequence/reassembly logic, congestion control, loss recovery, path authentication, MTU handling, and a reachable server-side endpoint that cooperates with the client. Sending arbitrary TCP packets over multiple unrelated proxy engines will not safely create one faster connection.

PG CORE now provides three related primitives:

- `fusion.Select`: deterministic policy selection from a supplied path snapshot.
- `fusion.Allocator`: concurrency-safe selection and active-flow slot reservation, with idempotent release leases.
- `fusion.Router`: selects one route for each outbound connection, calls that route's context-aware dialer, and releases the lease when dialing fails or the returned connection closes.
- `fusion.SOCKS5Dialer`: opens a TCP connection through an existing no-auth SOCKS5 listener, with domain/IPv4/IPv6 target encoding and cancellation-aware handshakes.
- `fusion.RefreshRoutes`: runs bounded, context-aware probes and atomically updates route health/RTT snapshots. Probe failures mark only their route unhealthy and are returned by route ID. It does not invent packet-loss measurements or equate a TCP connect with end-to-end VPN health.

The allocator/router counters represent reservations made through this API, not kernel socket state. The host supplies path health/quality snapshots and path-specific dialers. `SOCKS5Dialer` does not start or configure the proxy process. Existing engine adapters and OS-wide VPN routing are not automatically wired into this router.

The integration test `TestRouterUsesSOCKS5PathAndUpdatesAfterPathFailure` exercises route selection, a SOCKS5 handshake, and a health update using local test proxies. These proxies validate the code path only; they do not emulate real engines, OS VPN routing, or Internet reachability.

## Bounded route health refresh

The host can call `RefreshRoutes` with a probe appropriate to its use case. It limits the route count, worker concurrency, and per-route timeout; cancellation leaves the existing router snapshot untouched. A probe must honor its context. Failed probes are exposed to the caller for diagnostics and remove that path from future allocation until a later refresh succeeds. Choose an application-level probe when end-to-end DNS/HTTPS behavior matters; a successful socket connection alone is not proof of full tunnel health.

## Selection and lifecycle guarantees

- Reject malformed snapshots and duplicate path IDs.
- Exclude unhealthy paths and paths at their configured concurrency cap.
- Score eligible paths using RTT, loss, active-flow count, and configured weight.
- Use stable tie-breaking so identical snapshots produce the same result.
- Return `ErrNoEligiblePath` rather than silently choosing an unhealthy path.
- Preserve active-flow counts across health-snapshot updates; do not remove a path while it has active leases.
- Release reservations on dial failure and when the returned connection is closed; repeated closes do not release twice.

## Not implemented by this stage

- No packet-level bonding or single-flow bandwidth aggregation.
- No packet duplication, striping, resequencing, or cross-path retransmission.
- No TUN, route table, DNS, firewall, or OS VPN integration.
- No live telemetry collector, probe scheduler, server protocol, or automatic failover wiring.
- No promise of higher throughput or filtering bypass.

## Next engineering stages

1. Integrate path-specific dialers with the actual outbound connection lifecycle of each supported engine.
2. Supply fresh, bounded health measurements and add cancellation/recovery tests with two controlled local paths.
3. Define an authenticated multiplexing protocol between a client and a cooperating server before considering packet striping.
4. Prototype packet-level bonding only with explicit sequence numbers, bounded reordering buffers, congestion control, path failure recovery, and MTU tests.
5. Add platform-specific TUN ownership and rollback only after a single authoritative owner is defined for routes and DNS.

Performance claims require repeatable measurements against a single-path baseline on the same endpoint and network. Do not infer real throughput gains from unit tests or in-memory benchmarks.
