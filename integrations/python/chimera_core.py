"""Typed Python client for the Chimera mobile JSON bridge.

This module does not load Go code or create a VPN itself. Supply a transport
callable that forwards method names and JSON-compatible payloads to a host
process or native bridge.
"""
from __future__ import annotations

import json
from typing import Any, Callable, Mapping

Transport = Callable[[str, Mapping[str, Any]], str]


class ChimeraCoreError(ValueError):
    """Raised when the bridge returns malformed or unexpected data."""


class ChimeraCoreClient:
    def __init__(self, transport: Transport) -> None:
        if not callable(transport):
            raise TypeError("transport must be callable")
        self._transport = transport

    def evaluate(self, sample: Mapping[str, Any]) -> dict[str, Any]:
        if not isinstance(sample, Mapping):
            raise TypeError("sample must be a mapping")
        raw = self._transport("evaluate", {"sample": json.dumps(dict(sample), separators=(",", ":"))})
        try:
            result = json.loads(raw)
        except (TypeError, json.JSONDecodeError) as exc:
            raise ChimeraCoreError("bridge returned invalid JSON") from exc
        if not isinstance(result, dict):
            raise ChimeraCoreError("bridge decision must be a JSON object")
        if not isinstance(result.get("switch"), bool):
            raise ChimeraCoreError("bridge decision is missing boolean 'switch'")
        if result["switch"] and not isinstance(result.get("target_id"), str):
            raise ChimeraCoreError("switch decision is missing target_id")
        return result

    def confirm_switch(self, target_id: str) -> None:
        self._require_id(target_id)
        self._transport("confirmSwitch", {"targetId": target_id})

    def cancel_switch(self) -> None:
        self._transport("cancelSwitch", {})

    @staticmethod
    def _require_id(value: str) -> None:
        if not isinstance(value, str) or not value.strip():
            raise ValueError("target_id must be a non-empty string")
