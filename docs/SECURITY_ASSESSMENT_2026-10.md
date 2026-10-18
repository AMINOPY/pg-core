# Preliminary Security Assessment — 2026-10-18

## Scope and limitations

This is a source-code review of the `periodic-malware-scan` branch and its existing tests. It is not a certification, a full penetration test, or evidence that a deployed build is secure. No production device, tunnel endpoint, account, or third-party system was targeted. Dynamic tests must run in CI and on an authorized test device before this report can be considered complete.

## Changes in this hardening branch

- Reject scanner file-size limits above 1 GiB so `maxFileBytes + 1` cannot overflow; non-positive limits continue to select the bounded default.
- Check that the path still identifies the file originally opened before quarantine, verify the moved object identity after rename, and hash the quarantined content again before reporting success.
- Add a regression test for invalid file-size limits.
- Add `govulncheck` to Go CI and configure Dependabot updates.
- Configure Dependabot for Go modules, Maven, Gradle, and GitHub Actions.

## Findings and status

### Scanner file identity and content can change during scanning/quarantine

**Status: Mitigations added; residual race risk remains.** The scanner now checks file identity before and after the rename and verifies the quarantined bytes against the digest. A hostile process with write access to the scanned directory may still race path-based filesystem operations; portable Go APIs do not provide a fully race-free move-by-open-file-descriptor abstraction across all supported operating systems. Do not scan attacker-writable directories while automatic quarantine is enabled. A platform-specific, handle-relative implementation would be needed to close this class of race completely.

### Scanner size limit can overflow

**Status: Addressed in code; CI validation pending.** Configured maximums above 1 GiB are rejected, making the `maxFileBytes + 1` read limit bounded. A regression test covers an `int64` maximum input.

### Signed configuration verification and platform key storage

**Status: Not implemented by this branch.** HMAC helpers exist, but the repository review does not establish that the runtime configuration path verifies a signature before use. Android Keystore integration and signed update verification must be implemented in the platform application and tested there. Until then, do not treat configuration files as authenticated or use this scaffold to protect sensitive traffic.

### Go standard-library vulnerability in the previous CI toolchain

**Status: CI toolchain updated; rebuild/retest pending.** `govulncheck` found GO-2026-6617 (HTTP/2 HPACK encoder race causing a server crash) in the previous CI toolchain, Go 1.22.12. The advisory identifies Go 1.26.9 as a fixed version. CI now uses Go 1.26.9 and reruns vulnerability analysis. Any distributed binaries must also be rebuilt using a fixed Go release; changing CI alone does not patch already-built artifacts.

### Dependencies

**Status: Automated checks added; results pending.** CI runs Go vulnerability analysis and Dependabot is configured. No clean vulnerability result is claimed until the updated jobs complete. Review Java/Kotlin dependency update alerts as well as Go findings.

## Validation required before release

1. Confirm formatting, `go vet`, `go test`, and `go test -race` pass on this branch.
2. Review `govulncheck` results and Dependabot alerts; resolve or document every applicable finding.
3. Run the CI fuzz tests and extend coverage for scanner path handling, cancellation, and malformed signature feeds.
4. Exercise scan/quarantine behavior in a disposable test directory with symlinks, oversized files, permission failures, concurrent file replacement, and cancellation.
5. Verify the actual app's signature-checking path and platform key storage, including tampered configuration and rollback attempts.
6. Run authorized device-level tests and verify that failed startup, failed health checks, and process termination do not leave unintended processes or network routes.

## Security posture

This repository identifies itself as an early scaffold and states that it does not yet implement a complete tunnel/data plane. Passing these checks would improve confidence in the reviewed components but would not establish anonymity, resistance to DPI, or production readiness.
