# PG CORE Python Control API SDK

This dependency-free client wraps the PG CORE Control API v1. It supports the currently implemented Android and Windows release metadata, app announcement reads/publication, and app-local user status reads/updates.

## Quick start: client app (read-only)

Use a `public:read` key for client-side reads. The API key is still a secret; for widely distributed apps, use a public read-only key only and assume it can be extracted.

```python
import os
from pgcore_control_api import PGCoreControlClient

api = PGCoreControlClient(
    os.environ["PGCORE_API_URL"],
    os.environ["PGCORE_READ_KEY"],
)
print(api.get_app())
print(api.get_announcement())
print(api.get_latest_release("android"))
```

## Quick start: trusted backend / Telegram bot

Write-capable credentials belong only on a trusted backend. Do not put them in an APK, Windows executable, browser JavaScript, or Telegram bot command text.

```python
import os
from pgcore_control_api import PGCoreControlClient

admin = PGCoreControlClient(
    os.environ["PGCORE_API_URL"],
    os.environ["PGCORE_ADMIN_KEY"],
)
admin.publish_announcement("Scheduled maintenance at 22:00 UTC")
admin.set_user_status("app-owned-user-123", "suspended")
```

## Behavior and safety

- Requires HTTPS except for loopback HTTP during local development.
- Rejects redirects so a bearer key is not forwarded to a redirect destination.
- Limits response reads to 1 MiB and uses an explicit request timeout.
- Does not include response bodies, URL contents, or API credentials in raised error messages.
- The server binds each credential to one `app_id`; a caller cannot select another app by passing a tenant ID.
- User IDs must be IDs owned by the integrating app. The API stores status metadata only; it does not inspect devices or private app data.
- Release metadata does not silently install updates. The host app must validate it and use the platform's normal consent-based update process.
- The current API can read/update a known user ID; it does not yet provide user enumeration, remote command execution, device surveillance, or arbitrary app control.

## Run tests

From the repository root:

```sh
python -m unittest discover -s integrations/python -p 'test_*.py' -v
```

See [the API contract](../../docs/REMOTE_CONTROL_API.md) for deployment and server-side requirements. This SDK is a client library, not a hosted control-plane service; production use still requires operator authentication/MFA, rate limiting, key rotation/revocation, persistent audit logs, TLS, and an appropriate database-backed store for multi-instance deployments.
