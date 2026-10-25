# Multilanguage Core Plan (Go + C + Python + Kotlin)

## Decision

Do not rewrite the core in several languages at once. Keep Go as the authoritative coordinator and engine-adapter contract. Add another language only where a measured need or platform boundary justifies it. This avoids duplicated lifecycle state, inconsistent security checks, and cross-language debugging before the core is stable.

## Responsibilities

| Language | Owns | Must not own |
|---|---|---|
| Go | Engine registry, profile validation, lifecycle state machine, cancellation, health reports, resource ownership, public core API | Android UI lifecycle or unmeasured native optimizations |
| C | Small, isolated native primitives only after profiling demonstrates a bottleneck; explicit buffer/length APIs | Engine selection, secrets, process lifecycle, general configuration parsing |
| Python | CI orchestration, reproducible benchmarks, test fixtures, log/report analysis, fuzz-input generation | Runtime packet path or production credential storage |
| Kotlin | Android app/service adapter, foreground-service lifecycle, permission prompts, Android-specific socket protection/JNI bridge | A second independent core state machine or copied engine policy |

## Required boundaries

1. **One lifecycle owner:** Go owns start/stop/status decisions. Platform adapters forward lifecycle events and do not independently declare a connection healthy.
2. **Versioned contract:** any cross-language API must include an API version and explicit struct sizes/field types. Reject unknown versions.
3. **No implicit secrets:** credentials are passed only for the duration of an operation, never placed in diagnostic output or long-lived UI metadata.
4. **Cancellation and timeouts:** every operation that can block accepts a deadline or cancellation signal. Stop and cleanup have a bounded timeout.
5. **Resource ownership:** every socket, child process, native allocation, temporary file, and Android service has one documented owner and cleanup path.
6. **Health means data-path health:** process liveness alone is not sufficient to report Connected.
7. **No premature C:** profile the Go implementation first. A C module must have a benchmark showing a meaningful improvement and tests for boundary lengths, invalid inputs, and concurrent use.
8. **Cross-platform parity:** Linux and Windows tests run in CI; Android-specific behavior is tested on Android/Gradle CI or explicitly marked unverified.

## Suggested repository layout

```text
internal/
  engine/          # Go adapter contract and capability model
  orchestrator/    # single lifecycle owner
pkg/
  integration/     # host-app UI-neutral extension contract
platform/
  android/         # Kotlin adapter; no duplicate core state machine
native/
  include/         # C ABI only when an actual C implementation is needed
  src/             # measured native primitives only
tools/
  corecheck/       # Python CI/benchmark/report tooling
```

Create `platform/android`, `native`, or `tools/corecheck` only when their first real implementation and tests are ready; empty scaffolding directories are intentionally not part of this change.

## Implementation sequence

### Phase 1 — stabilize Go (current priority)
- Make `go vet ./...`, `go test ./...`, and `go test -race ./...` meaningful required checks.
- Resolve the incomplete Xray adapter without deleting tests or weakening TLS validation.
- Add lifecycle tests for cancellation, failed startup cleanup, repeated stop, concurrent start/stop, and resource leaks.
- Confirm Windows/Linux builds and document what cannot yet be tested on Android.

### Phase 2 — measurable diagnostics
- Add reproducible benchmark fixtures and machine-readable results.
- Use Python only to launch tests, compare benchmark output, and summarize failures.
- Record baseline throughput, allocations, CPU, memory, startup time, and health-check latency before optimizing.

### Phase 3 — Android adapter
- Add a Kotlin adapter only when the host Android app/module and its Gradle configuration are available.
- Map Android service lifecycle and socket-protection callbacks into the Go core contract.
- Test cancellation, app backgrounding, process recreation, and stop/cleanup.

### Phase 4 — optional C primitives
- Select a specific hotspot from profiling; do not port the entire core to C.
- Define a narrow, versioned C ABI with explicit buffer lengths and ownership rules.
- Keep a Go fallback and compare output equivalence, performance, and fuzz results.

## Acceptance gates

A phase is not complete because it compiles on one machine. Record the exact command, OS/architecture, commit, and result. Do not call the core production-ready until lifecycle, real-engine health, cleanup, cross-platform builds, and platform integration have been verified.
