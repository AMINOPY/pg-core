# Automatic Capability Learning Pipeline

Status: proposed pipeline; only the capability manifest validator is introduced with this change. No automatic code execution or plugin publishing is implemented by this document.

## Pipeline

1. Discover: read explicitly permitted metadata, API descriptions, protocol schemas, or documentation.
2. Normalize: convert claims into a versioned capability manifest.
3. Validate: enforce size, schema, identifier, and version limits.
4. Generate: optionally generate an adapter in a disposable build environment with no production credentials.
5. Test: run contract tests, malformed-input tests, resource-limit tests, and behavior regression tests.
6. Review risk: compute required permissions from a static policy baseline; model-generated risk labels are advisory only.
7. Package: create a reproducible package and digest; sign release metadata with a key held outside the build worker.
8. Stage: install only into an isolated staging environment.
9. Activate: apply local policy, verify package provenance, and use staged rollout/rollback.
10. Learn: retain sanitized outcomes and compatibility metadata. Do not automatically rewrite the trusted policy baseline from observations.

## Automation boundaries

Safe candidates for full automation include fetching public schemas, parsing documentation, producing manifests, running tests, and proposing low-privilege adapters. Code generation and execution must happen in a disposable, OS-isolated worker.

Never automatically grant a generated connector unrestricted filesystem, process, network, credential-store, VPN/TUN, or administrator access. High-risk capabilities remain disabled unless a trusted policy explicitly authorizes them.

## Knowledge storage

Persist connector identity, source/version, schema digest, test results, compatible core versions, required permissions, known limitations, and revocation state. Store no secrets or raw private configuration. Knowledge is evidence about compatibility; it is not a replacement for authorization.

## Release gates

- All unit and contract tests pass.
- Fuzz and resource-limit tests pass.
- Package digest and publisher signature verify.
- Permissions are within an approved policy.
- No critical/high unresolved security findings.
- Rollback and revocation tests pass.
