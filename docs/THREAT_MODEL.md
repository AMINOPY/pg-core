# Adaptive Core Threat Model

## Scope

This document describes the proposed adaptive connector and learning layers. It does not claim these layers are already implemented. The existing repository remains an early scaffold; see SECURITY.md and docs/ARCHITECTURE.md for current engine and platform limitations.

## Assets

- User data, connection profiles, credentials, signing keys, and local files.
- Integrity of the core, policies, adapter packages, and update metadata.
- Availability of the server, device agent, and connected services.
- Privacy of connector metadata, audit records, and learning history.

## Trust boundaries

1. The core policy evaluator is trusted code.
2. Every connector manifest, remote response, documentation page, generated adapter, and model output is untrusted data.
3. Generated code is never loaded into the core process by default.
4. The server is not automatically trusted with all device data. Device-side policy remains authoritative for local permissions.
5. A declared capability is a claim, not proof of behavior or authorization.

## Primary threats and mitigations

| Threat | Required control |
| --- | --- |
| Malicious connector or generated code | OS-enforced sandbox, least privilege, bounded CPU/memory/time, network and filesystem restrictions |
| Prompt injection through docs or tool output | Treat external content as data; separate instructions from data; never grant permissions based on model output |
| Connector impersonation | Mutual authentication where supported, pinned trust roots, credential rotation, replay protection |
| Manifest or package tampering | Canonical package digest, signed release metadata, verified publisher identity |
| Excessive permissions | Capability-scoped authorization and explicit policy decisions per operation |
| Resource exhaustion | Request and body limits, timeouts, concurrency limits, quotas, bounded retries |
| Secret leakage | Secret-store integration, redaction, no credentials in prompts/manifests/audit payloads |
| Server compromise affecting devices | Device-side verification, signed policy/update metadata, staged rollout and revocation |
| Rollback to vulnerable code | Minimum-version policy and monotonic update metadata where platform support permits |
| Unsafe learning drift | Versioned knowledge, regression suite, immutable policy baseline, staged activation and rollback |

## Required security invariants

- No connector is trusted because it was discovered automatically.
- Discovery and testing must not execute arbitrary code with core privileges.
- High-risk actions require an explicit policy decision; model confidence is not authorization.
- Fail closed for required security controls; do not silently downgrade to an insecure transport.
- Audit logs are bounded and redact credentials and sensitive payloads.
- Local security policy remains enforceable when the server is offline.
- Disable/revoke must stop future invocations and terminate isolated workers within a bounded deadline.

## Acceptance criteria

Before production, demonstrate these invariants with negative tests, integration tests on each supported OS, dependency scanning, and an independent security review. A passing unit-test suite alone is not a penetration test.
