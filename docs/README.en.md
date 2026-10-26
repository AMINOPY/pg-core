# PG CORE / Chimera Core

[فارسی](README.fa.md) | [English](README.en.md)

**Status: early scaffold; not production-ready.**

PG CORE is a modular network-engine coordinator. It includes configuration validation, engine lifecycle coordination, bounded DNS/HTTPS diagnostics, connection-level path selection, and initial sing-box/Xray adapters. These features do not constitute a complete production VPN.

## Features

- Go core for validation, engine coordination, and network diagnostics
- Initial sing-box and Xray adapters; installed binaries and real connectivity tests are still required
- Native TLS/WebSocket tunnel and local SOCKS5 proxy
- CDN/SNI candidate selection from supplied catalogs; no live public-range discovery
- Ghost: bounded DNS/HTTPS symptom analysis; a failure alone does not prove DPI
- Ghost Plus (C): conservative classification of supplied observations
- Ghost Ultra Plus (Kotlin and Python): local report interpretation with Persian/English output
- Unit tests for implemented components

## Run Ghost analysis

```sh
go run ./cmd/chimera --ghost example.com --samples 5
python tools/ghost_ultra_plus/ghost_report.py en < report.json
```

C test for Ghost Plus:

```sh
cc -std=c11 -Wall -Wextra -Werror -pedantic native/ghost_plus.c native/test_ghost_plus.c -o /tmp/ghost-plus-test
/tmp/ghost-plus-test
```

## Important limitations

The complete system traffic path for Android/Linux/Windows, OS-wide TUN/routes/DNS integration, and real-device validation across all engines are not finished. Connection-level path selection is not packet aggregation across tunnels. Ghost and its Plus variants do not guarantee DPI circumvention and do not automatically change network settings.

## Documentation

- [Ghost Plus](GHOST_PLUS.md)
- [Architecture](ARCHITECTURE.md)
- [Roadmap](ROADMAP.md)
- [Host-app integration](HOST_APP_INTEGRATION.md)
