# Native engine

Chimera's own tunnel engine (`internal/engine/native`). No sing-box, Xray or other binary is needed. It gives a local SOCKS5 proxy whose traffic goes through TLS + WebSocket to a server you control.

```
app / tun2socks -> 127.0.0.1:1080 (SOCKS5) -> TLS(SNI) -> edge IP -> your server -> internet
```

## Two ways to connect

1. **CDN/SNI (minimal):** `fronting.sni` + `fronting.edge_ips` in the config, and only `key` in the profile. The client connects to the listed IPs, announces the SNI, and checks the certificate for that name.
2. **Direct:** no fronting; `server` in the profile (`host` or `host:port`).

Both need the secret `key` of your own server. There is no built-in public server: a tunnel always ends at a server somewhere.

## Free server: Cloudflare Worker

1. Create a Worker on a free Cloudflare account and paste `deploy/cloudflare-worker.js`.
2. Add a secret variable `KEY` with a long random value.
3. SNI = your `*.workers.dev` (or custom) domain; IP list = Cloudflare edge IPs that work for you; profile `key` = `KEY`.

Check Cloudflare's current terms and free-plan limits before relying on it. On a VPS use `native.Server` behind a TLS terminator instead.

## Run

Basic local SOCKS5 profile (edge handshake only; health remains degraded because end-to-end data-plane traffic is not verified):

```sh
go run ./cmd/chimera --config config.native.example.json --profile profile.native.example.json --run
```

To also check a real HTTP destination through the active SOCKS5 tunnel, use the separate [health-check profile example](../profile.native.healthcheck.example.json):

```sh
go run ./cmd/chimera --config config.native.example.json --profile profile.native.healthcheck.example.json --run
```

The example checks `https://example.com/`; change it to an HTTPS endpoint you control if you prefer. A successful health check requires an HTTP 2xx response through the tunnel, not just a live process or successful edge handshake. Remote health endpoints must use HTTPS; plain HTTP is accepted only for loopback test targets. Redirects are not followed. Without `health_url`, an edge handshake alone is reported as `data_plane_unverified` (degraded), not healthy. This destination probe is not proof of system-wide VPN routing or DNS-leak protection.

## What is and is not included

Included: TLS with certificate verification, SNI/IP list with failover and "last good IP first", SOCKS5 CONNECT (TCP), multi-frame streaming, optional end-to-end HTTP health checks through SOCKS5, tests, Android hook (`Adapter.Control` for `VpnService.protect`).

Not included: UDP, TUN device, DNS proxy, system routes, IP scanner, Beast Mode, traffic obfuscation beyond standard TLS. On Android the TUN side is `VpnService` + a tun2socks library pointing at the SOCKS5 port.
