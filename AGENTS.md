# Repository agent instructions

## Scope and change safety

- Treat this repository as a network-engine core, not as a complete consumer VPN app.
- Before editing, inspect the relevant implementation, tests, README, and architecture docs.
- Preserve existing functionality and public contracts by default. Make focused, reviewable changes; do not rewrite unrelated components to satisfy a narrow task.
- Never describe process liveness, a mock, a compile-only check, or a benchmark smoke test as proof of real tunnel connectivity or measured real-world speed.
- Do not mark the core production-ready until platform-specific TUN/routes/DNS ownership, cleanup/rollback, and device-level tests exist and pass.

## Android work

- Consult the matching skill under `.agents/skills/android-upstream/` when one is installed. Installation/update instructions are in `docs/ANDROID_SKILLS_INTEGRATION.md`.
- Keep one explicit owner for Android `VpnService`, TUN, system routes, and DNS. Engine adapters should not silently compete for those resources.
- Test cancellation, permission revocation, network changes, reconnect, stop failures, and cleanup.
- Avoid logging credentials, tunnel keys, full sensitive URLs, or private connection profiles.
- Do not introduce Android-only dependencies into the Go core unless the architecture requires them.

## Validation and reporting

- Run the most targeted tests first, then the full applicable CI checks.
- Report exact commands and outcomes; distinguish tests actually run from checks that require a real device or unavailable platform toolchain.
- Update docs and the language/architecture inventory when adding a language, adapter, workflow, or platform contract.
