"""Bilingual formatter for bounded Ghost JSON reports; performs no network activity."""
from __future__ import annotations
import json
import sys
from typing import Any

def summarize(report: dict[str, Any], language: str = "en") -> str:
    if language not in ("en", "fa"):
        raise ValueError("language must be 'en' or 'fa'")
    domain = str(report.get("domain", "unknown"))[:253]
    samples, dns, https = (report.get(k, 0) for k in ("samples", "dns_successes", "https_successes"))
    if not all(isinstance(x, int) and not isinstance(x, bool) for x in (samples, dns, https)):
        raise ValueError("samples and success counts must be integers")
    if samples < 0 or samples > 20 or not 0 <= dns <= samples or not 0 <= https <= samples:
        raise ValueError("report counters are outside safe bounds")
    if language == "fa":
        return (f"گزارش شبح برای {domain}\nنمونه‌ها: {samples}\nموفقیت DNS: {dns}/{samples}\n"
                f"موفقیت HTTPS: {https}/{samples}\nاین نتایج به‌تنهایی وجود DPI را اثبات نمی‌کنند. "
                "تنظیمات شبکه به‌صورت خودکار تغییر نکرده است.")
    return (f"Ghost report for {domain}\nSamples: {samples}\nDNS successes: {dns}/{samples}\n"
            f"HTTPS successes: {https}/{samples}\nThese observations do not prove DPI. "
            "Network settings were not changed automatically.")

def main() -> int:
    language = sys.argv[1] if len(sys.argv) > 1 else "en"
    try:
        report = json.load(sys.stdin)
        if not isinstance(report, dict):
            raise ValueError("input must be a JSON object")
        print(summarize(report, language))
    except (json.JSONDecodeError, ValueError) as exc:
        print(f"invalid Ghost report: {exc}", file=sys.stderr)
        return 2
    return 0

if __name__ == "__main__":
    raise SystemExit(main())
