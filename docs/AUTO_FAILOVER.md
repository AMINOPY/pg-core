# Smart auto-failover policy

The engine-agnostic policy in `internal/policy/failover.go` computes when a caller should move away from an unhealthy or slow active server. Candidates include a `SubscriptionID`, so the selection can compare servers from the same subscription or across multiple subscriptions when the caller supplies them in one candidate set.

## Behavior

- A negative latency value (including sentinel `-1`) is treated as an invalid measurement, never as a fast server.
- A server is eligible as a target only when its latest status is `healthy`, latency is non-negative, and latency is within `MaxLatency`.
- `FailureThreshold` consecutive bad observations are required before switching, reducing one-sample flaps. Set to 1 for immediate failover.
- Candidates are ranked by latency, then subscription ID, then server ID for deterministic ties.
- `Cooldown` prevents repeated rapid switches. `MinImprovement` avoids switching solely for a small latency gain when the active server is still healthy.
- If no target qualifies, the policy reports `no_healthy_alternative` and does not invent a fallback.

## Integration boundary

This package is a decision policy, not a connection manager. It does not probe servers, import subscriptions, start/stop engines, transfer a live connection, or change Android VPN routes. The application must supply trustworthy health/latency samples, serialize calls, execute the switch through its platform-specific lifecycle, verify DNS and end-to-end connectivity after switching, and roll back safely if startup fails.

For Android, `VpnService` remains the single TUN owner. Do not stop the current engine until the replacement can be started safely under the app's supported design; do not run competing engines against the same TUN. A live process alone is not a connectivity check. Add emulator/device tests for disconnect, slow response, `-1` probe results, subscription changes, failed replacement, VPN revocation, DNS, IPv4/IPv6, and restored connectivity.

This policy does not promise to defeat filtering or guarantee that a server is reachable from every network. Use bounded, consent-based probes and avoid mimicking named third-party services.
