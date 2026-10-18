# Adaptive Capability Learning Pipeline

Status: bounded outcome learning and strict capability-manifest validation are implemented. Discovery, generated-adapter execution, signing, staged activation, and automatic publishing remain future work.

## Implemented evidence store

The Go package `internal/learning` records verified outcomes keyed by connector ID and version. It tracks sample/success/failure counts, consecutive failures, an exponentially weighted latency estimate, and the observation timestamp. Ranking hints use success rate, sample confidence, latency, and a consecutive-failure penalty.

The JSON-only mobile facade is in `mobile/learning.go`:

- `NewLearningStore`
- `ObserveJSON`
- `SnapshotJSON`
- `ExportJSON`
- `ImportJSON`

Observations must come from the host after an independent connectivity/result check; a connector must not be trusted to self-report success. Failed observations use `latency_ms: -1`. Data is bounded to 2,048 connector/version records and 1 MiB for import/export. Imports reject unknown fields, duplicate keys, invalid counters, and unsupported schema versions.

The store is currently in-memory. The host can export/import the versioned JSON and should keep it in platform-protected storage. It does not persist automatically, execute connectors, or automatically feed scores into authorization or failover. Callers must enforce the trusted manifest and permission policy separately.

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
10. Learn: retain sanitized outcomes and compatibility metadata. Never rewrite the trusted policy baseline from observations.

## Automation boundaries

Safe candidates for automation include fetching public schemas, parsing documentation, producing manifests, running tests, and proposing low-privilege adapters. Code generation and execution must happen in a disposable, OS-isolated worker.

Never automatically grant a generated connector unrestricted filesystem, process, network, credential-store, VPN/TUN, or administrator access. High-risk capabilities remain disabled unless a trusted policy explicitly authorizes them.

## Release gates

- All unit and contract tests pass.
- Fuzz and resource-limit tests pass.
- Package digest and publisher signature verify.
- Permissions are within an approved policy.
- No critical/high unresolved security findings.
- Rollback and revocation tests pass.
