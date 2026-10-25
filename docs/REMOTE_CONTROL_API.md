# PG CORE Remote Control API (v1)

This package defines a tenant-scoped HTTP/JSON control API that app developers can embed in a service. It is an API foundation, not a hosted control-plane service yet.

## Current status and limits

- Implemented: bearer-key authentication, per-key application tenant binding, capability scopes, strict JSON decoding, bounded request bodies, announcement publication/read, release metadata, and app-local user status flags.
- The API uses a `Store` interface. `MemoryStore` is for tests and demos only; its contents disappear on restart. A durable database-backed Store, deployment CLI, operator identity/login, rate limiting, key rotation/revocation, audit persistence, and production hosting are still required before exposing this to real users.
- The API does not remotely inspect a user's device, read private app data, execute arbitrary code, silently install updates, or bypass platform permissions. User IDs must be IDs owned by the integrating app; this service only stores status metadata.
- A published release is metadata only. The client must validate the release and use the platform's normal, consent-based update mechanism.

## Tenant isolation and credentials

Every credential is bound to one `AppID`; every read/write uses that authenticated tenant ID internally. The client cannot select another tenant through request parameters. Give the mobile/client app only `public:read`. Keep write-capable credentials on your backend/bot only, never in an APK, desktop executable, browser bundle, or public repository. Generate at least 32 random bytes per key. Base64 is encoding, not encryption; if used to transport key bytes, it does not make them secret.

Scopes:
- `public:read`: app metadata, current announcement, release metadata.
- `announcements:write`: publish or replace this app's announcement.
- `releases:write`: publish this app's release metadata.
- `users:write`: read/change status for users belonging to this app.

Deploy only behind HTTPS. Add an authenticated operator login and MFA in the management panel; API keys are service credentials, not human identities. Avoid logging Authorization headers or request bodies containing personal data.

## HTTP endpoints

All responses are JSON. Send `Authorization: Bearer <key>`.

| Method | Path | Scope | Purpose |
|---|---|---|---|
| GET | `/healthz` | none | Liveness only; never exposes tenant data |
| GET | `/v1/app` | `public:read` | Return the authenticated app ID and API version |
| GET | `/v1/announcement` | `public:read` | Current announcement or `null` |
| GET | `/v1/releases/latest?platform=android` | `public:read` | Latest metadata for android/linux/windows/ios/macos/web |
| POST | `/v1/admin/announcements` | `announcements:write` | `{"message":"...","expires_at":"RFC3339 timestamp"}` |
| POST | `/v1/admin/releases` | `releases:write` | `{"platform":"android","version":"1.2.3","url":"https://...","notes":"...","force":false}` |
| GET | `/v1/admin/users/{user_id}` | `users:write` | Read this app's user status |
| PUT | `/v1/admin/users/{user_id}` | `users:write` | `{"status":"active"}` or `{"status":"suspended"}` |

Errors use `{"error":"stable_code"}`; 401 means missing/invalid key, 403 means missing scope. Request bodies are capped at 1 MiB and reject unknown JSON fields and trailing JSON values. Announcement text is capped at 4,000 bytes; release links must use HTTPS.

## Start the API in an embedding Go service

```go
store := controlplane.NewMemoryStore() // demo/test only; not durable
api, err := controlplane.NewServer(store, []controlplane.Credential{
    {Key: os.Getenv("APP_A_READ_KEY"), AppID: "app-a", Scopes: []string{controlplane.ScopeRead}},
    {Key: os.Getenv("APP_A_ADMIN_KEY"), AppID: "app-a", Scopes: []string{
        controlplane.ScopeRead, controlplane.ScopeManageUsers,
        controlplane.ScopeManageAnnouncements, controlplane.ScopeManageReleases,
    }},
})
if err != nil { log.Fatal(err) }
srv := &http.Server{Addr: "127.0.0.1:8788", Handler: api, ReadHeaderTimeout: 5 * time.Second}
log.Fatal(srv.ListenAndServe())
```

Use a durable Store, TLS termination, request/read/write/idle timeouts, rate limiting, and graceful shutdown before production. Do not expose this example directly to the public internet.

## Python client (standard library)

```python
import json
import os
import urllib.request

BASE_URL = os.environ["PGCORE_API_URL"].rstrip("/")
READ_KEY = os.environ["PGCORE_READ_KEY"]

def get_json(path):
    req = urllib.request.Request(
        BASE_URL + path,
        headers={"Authorization": "Bearer " + READ_KEY, "Accept": "application/json"},
    )
    with urllib.request.urlopen(req, timeout=8) as response:
        return json.loads(response.read().decode("utf-8"))

print(get_json("/v1/app"))
print(get_json("/v1/announcement"))
print(get_json("/v1/releases/latest?platform=android"))
```

Keep write keys in a trusted backend or Telegram bot's server-side secret store. Never put admin keys into a bot command, chat message, or client-side code.

## Kotlin client (Android/JVM)

```kotlin
import java.net.HttpURLConnection
import java.net.URL

fun getPgCoreJson(baseUrl: String, readKey: String, path: String): String {
    require(baseUrl.startsWith("https://")) { "HTTPS is required" }
    val connection = (URL(baseUrl.trimEnd('/') + path).openConnection() as HttpURLConnection)
    connection.requestMethod = "GET"
    connection.connectTimeout = 5_000
    connection.readTimeout = 8_000
    connection.setRequestProperty("Authorization", "Bearer $readKey")
    connection.setRequestProperty("Accept", "application/json")
    try {
        val code = connection.responseCode
        val stream = if (code in 200..299) connection.inputStream else connection.errorStream
        val body = stream.bufferedReader(Charsets.UTF_8).use { it.readText() }
        check(code in 200..299) { "PG CORE API HTTP $code: $body" }
        return body
    } finally {
        connection.disconnect()
    }
}
```

On Android, run network work off the main thread and comply with the app's network-security configuration. The read key embedded in a client app must be treated as extractable and therefore low privilege.

## C client (libcurl)

```c
#include <curl/curl.h>
#include <stdio.h>
#include <stdlib.h>

int main(void) {
    const char *base = getenv("PGCORE_API_URL");
    const char *key = getenv("PGCORE_READ_KEY");
    if (!base || !key) return 2;
    if (strncmp(base, "https://", 8) != 0) return 2;

    char url[2048];
    if (snprintf(url, sizeof(url), "%s/v1/announcement", base) >= (int)sizeof(url)) return 2;
    CURL *curl = curl_easy_init();
    if (!curl) return 2;
    struct curl_slist *headers = NULL;
    char auth[512];
    if (snprintf(auth, sizeof(auth), "Authorization: Bearer %s", key) >= (int)sizeof(auth)) {
        curl_easy_cleanup(curl); return 2;
    }
    headers = curl_slist_append(headers, auth);
    curl_easy_setopt(curl, CURLOPT_URL, url);
    curl_easy_setopt(curl, CURLOPT_HTTPHEADER, headers);
    curl_easy_setopt(curl, CURLOPT_CONNECTTIMEOUT, 5L);
    curl_easy_setopt(curl, CURLOPT_TIMEOUT, 8L);
    curl_easy_setopt(curl, CURLOPT_FOLLOWLOCATION, 0L);
    CURLcode result = curl_easy_perform(curl);
    long status = 0;
    curl_easy_getinfo(curl, CURLINFO_RESPONSE_CODE, &status);
    if (result == CURLE_OK && status >= 200 && status < 300) {
        /* libcurl writes the JSON response to stdout by default. */
    }
    curl_slist_free_all(headers);
    curl_easy_cleanup(curl);
    return result == CURLE_OK && status >= 200 && status < 300 ? 0 : 1;
}
```

Add `#include <string.h>` for `strncmp`. Compile with `cc client.c $(pkg-config --cflags --libs libcurl)`.

## Telegram bot integration

The bot calls the API from its server process. The bot must authenticate the operator (for example, allow-listed Telegram user IDs plus an explicit admin role) before it uses a server-side admin key. Do not treat possession of a bot username, chat ID, or Base64-encoded string as authorization. Keep bot actions tenant-bound, log who initiated changes, and require confirmation for suspensions or forced-release notices.
