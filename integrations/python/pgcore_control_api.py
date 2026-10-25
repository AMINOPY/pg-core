"""Small, dependency-free client for the PG CORE tenant-scoped control API.

Keep write-capable keys on trusted servers only. Never ship them in an APK,
desktop executable, browser bundle, or public source repository.
"""
from __future__ import annotations

import json
import re
import urllib.error
import urllib.parse
import urllib.request
from typing import Any, Mapping

_MAX_RESPONSE_BYTES = 1 << 20
_USER_ID = re.compile(r"^[a-zA-Z0-9][a-zA-Z0-9._:@-]{0,127}$")


class PGCoreAPIError(RuntimeError):
    """A safe API error; response bodies and credentials are never included."""

    def __init__(self, status: int, code: str):
        self.status = status
        self.code = code
        super().__init__(f"PG CORE API request failed ({status}: {code})")


class _NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):  # noqa: ANN001
        return None


class PGCoreControlClient:
    """Client for PG CORE Control API v1.

    The API key should be a read-only key in client applications. Admin methods
    are intended for a trusted backend, e.g. a Telegram bot's server process.
    """

    def __init__(self, base_url: str, api_key: str, timeout: float = 8.0):
        parsed = urllib.parse.urlsplit(base_url)
        local_http = parsed.scheme == "http" and parsed.hostname in {
            "localhost", "127.0.0.1", "::1"
        }
        if parsed.scheme != "https" and not local_http:
            raise ValueError("base_url must use HTTPS (HTTP is allowed only on loopback)")
        if not parsed.hostname or parsed.username or parsed.password or parsed.query or parsed.fragment:
            raise ValueError("base_url must be an origin/path without credentials, query, or fragment")
        if not api_key or len(api_key) < 32 or any(ch.isspace() for ch in api_key):
            raise ValueError("api_key must be a non-empty service key of at least 32 characters")
        if timeout <= 0 or timeout > 60:
            raise ValueError("timeout must be in (0, 60]")
        self._base_url = base_url.rstrip("/")
        self._api_key = api_key
        self._timeout = timeout
        self._opener = urllib.request.build_opener(_NoRedirect())

    def get_app(self) -> dict[str, Any]:
        return self._request("GET", "/v1/app")

    def get_announcement(self) -> dict[str, Any]:
        return self._request("GET", "/v1/announcement")

    def get_latest_release(self, platform: str) -> dict[str, Any]:
        if platform not in {"android", "windows"}:
            raise ValueError("platform must be android or windows")
        return self._request("GET", "/v1/releases/latest?" + urllib.parse.urlencode({"platform": platform}))

    def publish_announcement(self, message: str, expires_at: str | None = None) -> dict[str, Any]:
        if not isinstance(message, str) or not message.strip() or len(message.encode("utf-8")) > 4000:
            raise ValueError("message must be non-empty and at most 4000 UTF-8 bytes")
        payload: dict[str, Any] = {"message": message}
        if expires_at is not None:
            payload["expires_at"] = expires_at
        return self._request("POST", "/v1/admin/announcements", payload)

    def publish_release(
        self, platform: str, version: str, url: str, notes: str = "", force: bool = False
    ) -> dict[str, Any]:
        if platform not in {"android", "windows"}:
            raise ValueError("platform must be android or windows")
        if not isinstance(force, bool):
            raise ValueError("force must be a boolean")
        return self._request("POST", "/v1/admin/releases", {
            "platform": platform, "version": version, "url": url, "notes": notes, "force": force
        })

    def get_user_status(self, user_id: str) -> dict[str, Any]:
        return self._request("GET", self._user_path(user_id))

    def set_user_status(self, user_id: str, status: str) -> dict[str, Any]:
        if status not in {"active", "suspended"}:
            raise ValueError("status must be active or suspended")
        return self._request("PUT", self._user_path(user_id), {"status": status})

    @staticmethod
    def _user_path(user_id: str) -> str:
        if not isinstance(user_id, str) or not _USER_ID.fullmatch(user_id):
            raise ValueError("invalid user_id")
        return "/v1/admin/users/" + urllib.parse.quote(user_id, safe="")

    def _request(self, method: str, path: str, payload: Mapping[str, Any] | None = None) -> dict[str, Any]:
        data = None if payload is None else json.dumps(payload, separators=(",", ":")).encode("utf-8")
        headers = {
            "Authorization": "Bearer " + self._api_key,
            "Accept": "application/json",
        }
        if data is not None:
            headers["Content-Type"] = "application/json"
        req = urllib.request.Request(self._base_url + path, data=data, headers=headers, method=method)
        try:
            with self._opener.open(req, timeout=self._timeout) as response:
                if response.status < 200 or response.status >= 300:
                    raise PGCoreAPIError(response.status, "http_error")
                raw = response.read(_MAX_RESPONSE_BYTES + 1)
                if len(raw) > _MAX_RESPONSE_BYTES:
                    raise PGCoreAPIError(response.status, "response_too_large")
        except urllib.error.HTTPError as exc:
            code = "http_error"
            try:
                raw_error = exc.read(4096)
                parsed_error = json.loads(raw_error.decode("utf-8"))
                candidate = parsed_error.get("error") if isinstance(parsed_error, dict) else None
                if isinstance(candidate, str) and re.fullmatch(r"[a-z_]{1,64}", candidate):
                    code = candidate
            except (ValueError, UnicodeDecodeError, OSError):
                pass
            raise PGCoreAPIError(exc.code, code) from None
        except urllib.error.URLError as exc:
            # Do not include exception text: some handlers can include request details.
            raise PGCoreAPIError(0, "transport_error") from None

        try:
            result = json.loads(raw.decode("utf-8"))
        except (ValueError, UnicodeDecodeError):
            raise PGCoreAPIError(0, "invalid_json_response") from None
        if not isinstance(result, dict):
            raise PGCoreAPIError(0, "invalid_json_response")
        return result
