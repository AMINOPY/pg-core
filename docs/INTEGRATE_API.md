# Integrate Chimera Core into a Website or Application

This guide describes the first runnable integration surface. The current HTTP API validates connector capability manifests; it does **not** execute arbitrary connectors, learn new code, start a VPN tunnel, or bypass DPI.

## Run the API locally

Requires Go 1.22+.

```sh
go test ./...
export CHIMERA_API_KEY="$(openssl rand -hex 32)"
export CHIMERA_API_ADDR="127.0.0.1:8080"
# Optional: exact browser origins, comma-separated. Do not use * for production.
export CHIMERA_ALLOWED_ORIGINS="http://localhost:3000"
go run ./cmd/chimera-api
```

The API refuses to start without `CHIMERA_API_KEY`. The default bind address is loopback, not a public interface. For deployment, put it behind a TLS-terminating reverse proxy, keep the key in a server-side secret manager, restrict network access, and configure rate limits at the proxy. Do not expose a plaintext public listener.

## Endpoint

`POST /v1/manifests/validate`

Headers:
- `Authorization: Bearer <server-side-api-key>`
- `Content-Type: application/json`

Body: a manifest matching the contract in [CONNECTOR_API.md](CONNECTOR_API.md).

Example using curl:

```sh
curl --fail-with-body http://127.0.0.1:8080/v1/manifests/validate \
  -H "Authorization: Bearer $CHIMERA_API_KEY" \
  -H "Content-Type: application/json" \
  --data-binary @connector-manifest.json
```

Successful response includes the connector ID, version, and validated capability summaries. Invalid manifests return a generic error code without parser internals.

## Website integration

**Do not put `CHIMERA_API_KEY` in browser JavaScript, HTML, a mobile app bundle, or a public repository.** A browser app should call its own backend; that backend adds the secret and calls Chimera. CORS is an origin policy, not authentication.

Example server-side Node.js handler:

```js
// Example only: run on your backend, not in browser code.
export async function validateConnector(req, res) {
  const response = await fetch(process.env.CHIMERA_API_URL + "/v1/manifests/validate", {
    method: "POST",
    headers: {
      "Authorization": "Bearer " + process.env.CHIMERA_API_KEY,
      "Content-Type": "application/json"
    },
    body: JSON.stringify(req.body),
    signal: AbortSignal.timeout(5000)
  });
  const payload = await response.json();
  res.status(response.status).json(payload);
}
```

## Python application integration

```python
import os
import requests

response = requests.post(
    os.environ["CHIMERA_API_URL"] + "/v1/manifests/validate",
    headers={
        "Authorization": "Bearer " + os.environ["CHIMERA_API_KEY"],
        "Content-Type": "application/json",
    },
    json=manifest,
    timeout=5,
)
response.raise_for_status()
result = response.json()
```

## Security and production checklist

- Terminate TLS and use a trusted certificate.
- Store API keys in a secret manager; rotate them and never ship them to clients.
- Set exact allowed browser origins; CORS does not replace auth.
- Configure reverse-proxy rate limiting, request logging redaction, and monitoring.
- Keep this API on loopback or a private network unless a hardened gateway is configured.
- Use per-customer credentials and authorization before expanding beyond manifest validation.
- Treat manifests and all discovered metadata as untrusted. Validation does not authorize or execute a capability.
- Do not use this validation endpoint as proof a plugin is safe.
- Run the full CI, platform integration tests, and an independent security review before production.

## What is not provided yet

Connector registry persistence, account management, OAuth, scoped per-tenant keys, operation execution, sandbox workers, automatic adapter generation, signed package publishing, device enrollment, mobile SDK packaging, and a functioning DPI-resilient tunnel remain future work. The current endpoint is a safe first integration point, not the finished adaptive runtime.
