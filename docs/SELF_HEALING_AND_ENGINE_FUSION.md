# Self-healing diagnostics and engine fusion

## Self-healing

The `diagnostics` package offers a single-cycle, bounded repair runner:
1. A registered checker reports a structured finding.
2. Only findings explicitly marked recoverable can trigger a repair.
3. Repairs are implemented by the host application; the core does not execute arbitrary shell commands or rewrite its own source/configuration.
4. A verifier must confirm recovery before the result is marked repaired.
5. Attempts are bounded (maximum three) and may be cancelled by context timeout/cancellation.

A host can register safe actions such as restarting a failed adapter, reloading a validated configuration, or rechecking DNS. Destructive actions, silent configuration changes, and unbounded restart loops are intentionally excluded. Findings should be sanitized and must not include credentials, private keys, full URLs containing tokens, or user traffic.

## Engine fusion

The `fusion.Engine` contract is the common adapter boundary for different cores. Each adapter reports an ID, version, capabilities, and exclusive OS resource claims. Existing or future cores (for example sing-box, Xray, Mihomo, or another implementation) can participate by implementing the contract in a dedicated adapter. Their native configuration formats and runtime semantics are not magically interchangeable; each adapter is responsible for validation and translation.

The registry prevents two running engines from claiming the same exclusive resource, such as an Android VPN/TUN interface. It does not assume that multiple cores can safely process the same traffic simultaneously. On Android, keep a single VPN/TUN owner and use explicit handoff/verification when changing owners.

## Current limitations

These packages provide reusable contracts and unit-tested policy primitives, not a fully deployed watchdog or a universal adapter for every engine. Applications must wire real engine checkers, repairers, verifiers, and adapter implementations. Platform integration, native builds, and end-to-end device tests are still required before unattended recovery should be enabled in production.
