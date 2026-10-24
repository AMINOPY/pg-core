# Quick Connect (no local JSON config)

Chimera can open a local web UI with two user-entered values: an SNI hostname and a comma/space-separated list of CDN edge IPs. It uses the native TLS + WebSocket tunnel and does not require a local JSON config or an installed Xray/sing-box executable.

## Prerequisites

The server side must already exist and be under your control (for example, a deployed Cloudflare Worker from `deploy/cloudflare-worker.js`, with its `KEY` secret configured). The client process needs the same key as `CHIMERA_TUNNEL_KEY`. This key is intentionally not a third browser field and is never sent back by the UI.

Generate a long URL-safe secret and configure the same value as the Worker secret `KEY` and client environment variable `CHIMERA_TUNNEL_KEY`. Do not commit the secret to Git.

## Run

Linux/macOS:

```sh
export CHIMERA_TUNNEL_KEY='replace-with-your-own-long-random-secret'
go run ./cmd/chimera --quick-connect
```

Windows PowerShell:

```powershell
$env:CHIMERA_TUNNEL_KEY = "replace-with-your-own-long-random-secret"
go run ./cmd/chimera --quick-connect
```

Open `http://127.0.0.1:8787`, enter the SNI hostname and CDN edge IP list, and select **Connect tunnel**. The UI probes the TLS + WebSocket endpoint before reporting success. **Disconnect** stops the local SOCKS5 listener.

By default, the SOCKS5 endpoint is `127.0.0.1:1080`. Configure an individual application to use that proxy. This quick-connect mode does not install a TUN interface or change system-wide routes/DNS, so the button must not be interpreted as a whole-device VPN switch.

To change the UI address, pass `--quick-connect 127.0.0.1:8787`. It refuses non-loopback bind addresses and validates Host/Origin to reduce DNS-rebinding and cross-origin risks.

## Why SNI and IPs alone are not enough

SNI chooses the TLS name and the edge IP chooses where TCP connects. Neither provides the server's shared secret or guarantees the CDN forwards WebSocket traffic to the tunnel server. The server key is therefore supplied out of band through an environment variable; the Worker/server must use the matching key and route. A generic Xray server cannot be inferred from SNI/IP alone: Xray also needs a real server address, protocol, and credentials. This flow uses the native engine, not Xray.
