# Traffic fusion: flow selection and allocator boundary

## What “fusion” means in this repository

Several independent VPN/proxy engines running at once is not, by itself, traffic fusion. True packet-level bonding needs a common data plane, sequence/reassembly logic, congestion control, loss recovery, path authentication, MTU handling, and a reachable server-side endpoint that cooperates with the client. Sending arbitrary TCP packets over multiple unrelated proxy engines will not safely create one faster connection.

PG CORE now provides two separate primitives:

- `fusion.Select`: deterministic policy selection from a supplied path snapshot.
- `fusion.Allocator`: concurrency-safe selection and active-flow slot reservation for new flows, plus idempotent release leases.

The allocator prevents concurrent callers from exceeding a configured path capacity based on the same stale counter. Its counters represent reservations made through that allocator, not kernel socket state. The host still owns real connection creation/closure and must release each lease when the flow ends.

## Selection policy

- Reject malformed snapshots and duplicate path IDs.
- Exclude unhealthy paths and paths at their configured concurrency cap.
- Score eligible paths using RTT, loss, active flow count, and configured weight.
- Use stable tie-breaking so identical snapshots produce the same result.
- Return `ErrNoEligiblePath` rather than silently choosing an unhealthy path.
- Preserve active-flow counts across health-snapshot updates; do not remove a path while it has active leases.

## Not implemented by this stage

- No packet-level bonding or single-flow bandwidth aggregation.
- No packet duplication, striping, resequencing, or cross-path retransmission.
- No TUN, route table, DNS, firewall, or OS VPN integration.
- No live telemetry collector, probe scheduler, server protocol, or automatic failover wiring.
- No promise of higher throughput or filtering bypass.

## Next engineering stages

1. Connect allocator leases to real outbound dial/close lifecycles and live health samples.
2. Add integration tests with two controlled local paths, cancellation, and recovery.
3. Define an authenticated multiplexing protocol between a client and a cooperating server before considering packet striping.
4. Prototype packet-level bonding only with explicit sequence numbers, bounded reordering buffers, congestion control, path failure recovery, and MTU tests.
5. Add platform-specific TUN ownership and rollback only after a single authoritative owner is defined for routes and DNS.

Performance claims require repeatable measurements against a single-path baseline on the same endpoint and network. Do not infer real throughput gains from unit tests or in-memory benchmarks.
