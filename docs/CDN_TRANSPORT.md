# CDN-backed TLS transport and automatic profile discovery

**Status: profile contract only; no traffic transport or discovery client is implemented yet.**

Chimera can support a CDN-backed endpoint when the operator controls (or is explicitly authorized to use) the hostname and configures the CDN to proxy that origin. TLS must authenticate the configured endpoint. SNI is the hostname used during the TLS handshake; it does not itself create a tunnel or make a CDN relay arbitrary protocols.

## Current code

The `internal/transportprofile` package validates an explicit endpoint profile:

- HTTPS on port 443, with no URL credentials, query, or fragment
- DNS hostname required; IP literals are rejected
- `server_name` must match the endpoint hostname
- transport kind is explicit (`https`, `websocket`, or `grpc`)
- secrets are referenced by environment-variable name rather than embedded in JSON

This package is a validation contract, not a dialer. It does not configure a CDN, establish a tunnel, discover a profile, or modify system routes/DNS.

## Proposed no-manual-config flow

A universal zero-config mode is not technically possible: a client still needs a trusted bootstrap identity, an endpoint or discovery source, credentials when required, and a supported server/CDN. The intended low-friction mode is:

1. The app ships with a trusted bootstrap key or the user supplies their own service URL once.
2. The client fetches `https://<operator-owned-host>/.well-known/pg-core/transport.json` using normal certificate verification.
3. The manifest is versioned, size-bounded, short-lived, and signed (Ed25519); the signature is verified against a trust key already provisioned in the app or profile.
4. The client validates the manifest and the selected transport profile before connecting.
5. If discovery fails or the signature/expiry is invalid, the client fails closed or asks the user to retry. It must not silently pick random public hosts.
6. The app reports the selected transport, certificate-validation status, discovery timestamp, and failure reason without logging credentials.

The signed manifest, key rotation, expiry/rollback protection, discovery client, and endpoint health scoring remain future work.

## CDN and SNI requirements

- Use a hostname and CDN zone controlled by the operator, or an explicitly authorized service.
- Keep the TLS server name and HTTP authority aligned. Do not depend on a mismatch between SNI and HTTP Host to impersonate an unrelated tenant.
- Configure the CDN to forward the chosen application protocol to the origin; many CDN products only proxy specific HTTP/WebSocket/gRPC modes and ports.
- Use end-to-end TLS validation where supported, short-lived credentials, and an authenticated origin.
- Treat CDN compatibility as provider- and plan-specific. A successful TLS handshake does not prove that a long-lived tunnel or arbitrary byte stream will be relayed.
- Do not claim universal DPI resistance. Networks and CDN operators can block endpoints, protocols, or traffic patterns.

## Planned validation

Before enabling this as a product feature, add tests for manifest signatures, expiry and rollback, hostname binding, invalid certificates, oversized responses, redirect handling, credential redaction, provider protocol limits, reconnect behavior, and controlled end-to-end traffic. Then test actual Android, Linux, and Windows routing separately; transport discovery does not implement TUN, routes, DNS, or leak protection.
