# Language strategy

Chimera should be polyglot by component, not by rewriting the same logic in every language. Each additional language adds build, release, audit, and cross-platform maintenance cost.

## Proposed allocation

| Component | Preferred language | Reason | Status |
|---|---|---|---|
| Core lifecycle coordinator, CLI, engine adapters, diagnostics and tests | Go | Existing module, straightforward concurrency and portable builds | Implemented; platform data planes remain incomplete |
| Packet/transport hot path, if profiling proves it necessary | Rust | Memory safety and predictable low-level control | Evaluation only; no production Rust runtime code yet |
| Android host and VPNService integration | Kotlin (Java interoperability) | Native Android lifecycle and permissions | Lifecycle/options scaffolding and unit tests exist; packet-forwarding backend is unavailable by default |
| Windows desktop/service integration | C# | Native Windows UX and service APIs | Planned |
| Optional web configuration UI | TypeScript | UI ecosystem and type-safe client logic | Planned |
| Local reporting, test helpers and tooling | Python | Supporting diagnostics and repeatable experiments | Supporting code only; not a tunnel data plane |
| Narrow native validation/classification helpers | C | Keep small native ABI helpers where a concrete integration needs them | Limited helpers exist; not a complete transport or tunnel |
| Linux platform adapter | Rust or a small Go adapter, selected after prototype | TUN, routes, DNS and privilege handling need platform tests first | Undecided |

C, C++, PHP, Ruby, and R are not planned by default. They should only be introduced when a concrete platform SDK, integration boundary, or measured requirement justifies them. Adding languages for their own sake would increase risk rather than capability.

## Interoperability rules

- Define stable, versioned JSON contracts for configuration and diagnostics.
- Keep secrets out of config files, logs, test fixtures, and crash reports.
- Use a narrow process or FFI boundary; avoid duplicating lifecycle ownership.
- Require tests and dependency/license review for every new runtime.
- Benchmark before moving hot paths from Go to Rust or C/C++.
- Native OS networking must report what it actually applied and restore prior state on stop/failure.
- Ship reproducible builds and a support matrix per OS and architecture.

## Delivery order

1. Finish profile validation and signed discovery tests.
2. Implement and benchmark a transport dialer with explicit TLS identity checks.
3. Pick one initial OS for end-to-end route/DNS/rollback tests.
4. Add Kotlin/Android or C#/Windows host integration only after the core contract stabilizes.
5. Add Rust only for the measured hot path or a platform-specific requirement, with Go as the coordinator until the migration is proven.
