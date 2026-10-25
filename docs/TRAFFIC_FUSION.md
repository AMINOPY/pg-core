# Traffic fusion: implementation boundary and first-stage path selector

## What “fusion” means in this repository

Several independent VPN/proxy engines running at once is not, by itself, traffic fusion. True packet-level bonding needs a common data plane, sequence/reassembly logic, congestion control, loss recovery, path authentication, MTU handling, and a reachable server-side endpoint that cooperates with the client. Sending arbitrary TCP packets over multiple unrelated proxy engines will not safely create one faster connection.

PG CORE currently implements lifecycle coordination and exclusive resource claims. The first safe increment is **flow-level path selection**: choose one healthy path for each new flow, using current RTT, loss, configured weight, active-flow count, and a concurrency cap. This can spread independent connections across paths; it does not add the bandwidth of multiple paths to one existing flow.

## First-stage policy (internal/fusion)

The fusion.Select function is a pure, deterministic decision function:

- Rejects malformed snapshots and duplicate path IDs.
- Excludes unhealthy paths and paths at their configured concurrency cap.
- Scores eligible paths using RTT, loss, active flow count, and configured weight.
- Uses stable tie-breaking so the same snapshot produces the same result.
- Returns ErrNoEligiblePath instead of silently choosing an unhealthy path.

The host owns measurement collection, atomic flow allocation/counter updates, and actual dial operations. Stale or racing snapshots can still overload a path if the caller does not serialize allocation.

## Not implemented by this stage

- No packet-level bonding or single-flow bandwidth aggregation.
- No packet duplication, striping, resequencing, or cross-path retransmission.
- No TUN, route table, DNS, firewall, or OS VPN integration.
- No probes, live telemetry collector, server protocol, or automatic failover wiring.
- No promise of higher throughput or filtering bypass.

## Next engineering stages

1. Integrate this selector with a connection allocator and live path-health samples.
2. Add integration tests with two controlled local paths, cancellation, concurrent allocation, and recovery.
3. Define an authenticated multiplexing protocol between a client and a cooperating server before considering packet striping.
4. Prototype packet-level bonding only with explicit sequence numbers, bounded reordering buffers, congestion control, path failure recovery, and MTU tests.
5. Add platform-specific TUN ownership and rollback only after a single authoritative owner is defined for routes and DNS.

Performance claims require repeatable measurements against a single-path baseline on the same endpoint and network. Do not infer real throughput gains from unit tests or in-memory benchmarks.
