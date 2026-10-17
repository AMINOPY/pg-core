# Weak-network resilience

The `internal/resilience` package provides bounded exponential reconnect delays
for engine adapters. The default schedule starts at 500 ms, doubles after each
consecutive failure, and caps at 30 seconds. A scheduler may apply symmetric
jitter to avoid synchronized retry bursts.

Recommended adapter behavior for weak links:

- Treat transient DNS, timeout, and connection-reset errors as retryable; do not
  retry invalid configuration or authentication failures indefinitely.
- Apply per-attempt deadlines and a total startup/recovery budget.
- Reset the consecutive-failure counter only after the tunnel has remained
  healthy for a configurable stability window, not immediately after a TCP
  handshake.
- Probe health with bounded, low-frequency checks; avoid high-rate polling on
  metered or congested networks.
- Keep last-known-good configuration and make route/engine changes atomic.
- On network-interface changes, re-resolve endpoints and re-establish sessions
  without creating parallel unbounded reconnect loops.
- Expose state and metrics (connecting, connected, degraded, retrying) so a UI
  can explain recovery rather than appearing frozen.

This is a timing policy, not a complete tunnel implementation. The current
scaffold still needs engine adapters to consume it, plus platform-specific TUN,
DNS, route management, and end-to-end leak/recovery tests. It does not promise
connectivity where the network blocks the selected transport.
