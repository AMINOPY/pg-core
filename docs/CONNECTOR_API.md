# Connector API Contract (Proposal)

Status: design proposal. These endpoints are not yet implemented as an HTTP server.

## Principles

- Keep the contract transport-neutral; REST is the first server transport, with MCP and gRPC as optional adapters.
- Every request has a bounded body, deadline, authenticated principal, request ID, and structured error.
- Connector manifests are untrusted declarations. The core validates syntax and policy separately.
- Never place access tokens, private keys, raw proxy/share links, or secret-bearing configuration in manifests or ordinary logs.
- Capability IDs and versions are stable identifiers, not authorization grants.

## Proposed endpoints

| Method | Path | Purpose |
| --- | --- | --- |
| POST | /v1/connectors | Register a connector identity and transport metadata |
| GET | /v1/connectors/{id} | Read connector status and version |
| POST | /v1/connectors/{id}/discover | Fetch and validate an advertised manifest |
| GET | /v1/capabilities | List known capability declarations and their verification state |
| POST | /v1/operations/execute | Invoke one named capability after authorization |
| POST | /v1/learning/jobs | Start a bounded discovery/evaluation job |
| GET | /v1/learning/jobs/{id} | Read job status and sanitized results |
| POST | /v1/plugins/{id}/activate | Request activation of a tested, policy-approved package |
| POST | /v1/plugins/{id}/disable | Disable a connector or package |
| GET | /v1/audit/events | Read authorized, redacted audit events |
| POST | /v1/devices/enroll | Enroll a device with a scoped identity |

## Operation execution

Every execution request should include:
- request_id: unique bounded identifier for replay/audit correlation.
- connector_id, capability_id, and exact capability version.
- input: validated against the stored input schema and byte limit.
- deadline_ms: clamped by server policy; callers cannot set unbounded deadlines.
- idempotency_key for operations that may be retried.

The policy evaluator determines whether the authenticated principal may invoke the capability. A manifest's permissions list is a request for permissions, not a grant. The executor must return a stable error code and must not leak stack traces, tokens, or raw secret-bearing input.

## Suggested response envelope

    {
      "schema_version": "1.0",
      "request_id": "req-01J-example",
      "status": "rejected",
      "error": {
        "code": "POLICY_DENIED",
        "message": "operation is not permitted"
      }
    }

## Compatibility

- Reject unknown major schema versions.
- Add optional fields only in backward-compatible minor revisions.
- Test old and new client/server pairs in CI.
- Keep existing engine lifecycle and mobile evaluate/confirm/cancel contracts separate from this proposed generic connector API.
