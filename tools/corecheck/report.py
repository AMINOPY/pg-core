#!/usr/bin/env python3
"""Validate and summarize machine-readable PG CORE benchmark results.

Input is a JSON object with a non-empty "benchmarks" array. Each row must
contain a non-empty name, non-negative duration_ns, and non-negative
iterations. The tool intentionally does not execute network probes.
"""
from __future__ import annotations

import argparse
import json
import math
import sys
from pathlib import Path
from typing import Any


def load_report(path: Path) -> dict[str, Any]:
    try:
        payload = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, UnicodeError, json.JSONDecodeError) as exc:
        raise ValueError(f"cannot read valid JSON from {path}: {exc}") from exc
    if not isinstance(payload, dict):
        raise ValueError("top-level JSON value must be an object")
    rows = payload.get("benchmarks")
    if not isinstance(rows, list) or not rows:
        raise ValueError("'benchmarks' must be a non-empty array")
    for index, row in enumerate(rows):
        prefix = f"benchmarks[{index}]"
        if not isinstance(row, dict):
            raise ValueError(f"{prefix} must be an object")
        name = row.get("name")
        duration = row.get("duration_ns")
        iterations = row.get("iterations")
        if not isinstance(name, str) or not name.strip():
            raise ValueError(f"{prefix}.name must be a non-empty string")
        if isinstance(duration, bool) or not isinstance(duration, (int, float)):
            raise ValueError(f"{prefix}.duration_ns must be numeric")
        if not math.isfinite(duration) or duration < 0:
            raise ValueError(f"{prefix}.duration_ns must be finite and >= 0")
        if isinstance(iterations, bool) or not isinstance(iterations, int) or iterations < 0:
            raise ValueError(f"{prefix}.iterations must be an integer >= 0")
    return payload


def render(payload: dict[str, Any]) -> str:
    lines = ["PG CORE benchmark summary", "name | duration (ns) | iterations", "--- | ---: | ---:"]
    for row in payload["benchmarks"]:
        lines.append(f"{row['name']} | {row['duration_ns']:g} | {row['iterations']}")
    metadata = payload.get("metadata")
    if isinstance(metadata, dict) and metadata:
        lines.extend(["", "Metadata"])
        for key in sorted(metadata):
            value = metadata[key]
            lines.append(f"- {key}: {value}")
    return "\n".join(lines)


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("report", type=Path, help="path to benchmark JSON")
    args = parser.parse_args(argv)
    try:
        print(render(load_report(args.report)))
    except ValueError as exc:
        print(f"corecheck: {exc}", file=sys.stderr)
        return 2
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
