# Adaptive dual-path support

PG CORE's `internal/fusion` package now includes a deterministic policy decision helper for a primary path, a warm backup, and optional flow-level distribution.

## Modes

- `primary`: primary meets the configured quality thresholds; the backup remains available as a candidate.
- `backup`: the caller has explicit evidence that the primary is blocked, or its measured quality is below thresholds, and a healthy alternate path exists.
- `dual-flow`: both primary and alternate paths are healthy and the caller enabled distribution. The existing fusion router can distribute new outbound TCP connections across eligible paths.
- `offline`: no supplied path meets the quality policy.

## Inputs and safety

`DecideAdaptivePaths` consumes caller-supplied `QualityReport` snapshots and explicit thresholds. It does not run probes, change engine lifecycle, or autonomously identify DPI. A caller may set `primaryBlocked` only when a real probe supplies evidence of reachability failure. Latency and loss alone can indicate degradation, but cannot reliably identify DPI as the cause.

Use bounded, protocol-appropriate probes against controlled or trusted endpoints. The host is responsible for refreshing measurements and applying the decision to its connection lifecycle. The decision helper does not install TUN, alter OS routes/DNS, or wire itself into a platform VPN service.

## What this does not do

Dual-flow mode assigns each new connection to one path. It does not split one TCP connection into two byte streams and cannot promise doubled throughput. True single-flow bonding needs a cooperating server protocol with authentication, sequencing/reassembly, congestion control, replay protection, MTU handling, and failure recovery. System-wide support still needs platform-specific TUN and route/DNS ownership plus real-device tests.

## Tests

`go test ./internal/fusion` covers primary/backup/dual-flow/offline decisions, threshold validation, and malformed quality reports.
