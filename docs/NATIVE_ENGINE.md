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

```
go run ./cmd/chimera --config config.native.example.json --profile profile.native.example.json --run
```

It prints the SOCKS5 address and an edge check (a real TLS + WebSocket probe, not only "process alive"). For a real destination check through the tunnel, set `health_url` in the native profile, for example `https://example.com/`. Health is reported as degraded if the edge handshake succeeds but the configured endpoint does not return HTTP 2xx through the local SOCKS5 tunnel. Remote health endpoints must use HTTPS; plain HTTP is accepted only for loopback test targets. Redirects are not followed.

## What is and is not included

Included: TLS with certificate verification, SNI/IP list with failover and "last good IP first", SOCKS5 CONNECT (TCP), multi-frame streaming, optional end-to-end HTTP health checks through SOCKS5, tests, Android hook (`Adapter.Control` for `VpnService.protect`).

Not included: UDP, TUN device, DNS proxy, system routes, IP scanner, Beast Mode, traffic obfuscation beyond standard TLS. On Android the TUN side is `VpnService` + a tun2socks library pointing at the SOCKS5 port.
