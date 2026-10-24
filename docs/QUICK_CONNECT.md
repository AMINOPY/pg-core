# Quick Connect (no local JSON config)

Chimera opens a local web UI with four modes: Auto, Fronting CDN, CDN IR, and CDN US. The only editable manual values are an SNI hostname and a comma/space-separated list of CDN edge IPs. Auto and the two regional modes scan their separately configured candidate catalogs, test at most 1,000 SNI/IP pairs, and populate the two multi-line fields with up to 20 top-ranked distinct values. It currently uses the native TLS + WebSocket tunnel, not Xray, and does not require the user to create a local JSON config.

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

Open `http://127.0.0.1:8787`. In **Fronting CDN**, enter the SNI hostname and CDN edge IP list. In **Auto**, **CDN IR**, or **CDN US**, select the mode and press Connect; the app probes the configured catalog, fills the fields with up to 20 ranked values, then tries the best measured routes until the native tunnel health check succeeds. **Disconnect** stops the local SOCKS5 listener.

By default, the SOCKS5 endpoint is `127.0.0.1:1080`. Configure an individual application to use that proxy. This quick-connect mode does not install a TUN interface or change system-wide routes/DNS, so the button must not be interpreted as a whole-device VPN switch.

To change the UI address, pass `--quick-connect 127.0.0.1:8787`. It refuses non-loopback bind addresses and validates Host/Origin to reduce DNS-rebinding and cross-origin risks.

## Why SNI and IPs alone are not enough

SNI chooses the TLS name and the edge IP chooses where TCP connects. Neither provides the server's shared secret or guarantees the CDN forwards WebSocket traffic to the tunnel server. The server key is therefore supplied out of band through an environment variable; the Worker/server must use the matching key and route. A generic Xray server cannot be inferred from SNI/IP alone: Xray also needs a real server address, protocol, and credentials. This flow uses the native engine, not Xray.

## Auto and regional candidate catalogs

Set the relevant environment variable to a JSON catalog exported/curated from your own Network Checker workflow or another authorized source:

- `CHIMERA_AUTO_CANDIDATES`: general catalog
- `CHIMERA_CDN_IR_CANDIDATES`: operator-curated Iran catalog
- `CHIMERA_CDN_US_CANDIDATES`: operator-curated US catalog

Each file may contain up to 1,000 SNI hostnames and 1,000 public CDN edge IPs. See [AUTO_CDN_CATALOG.md](AUTO_CDN_CATALOG.md) and the two regional example files. Network Checker is a Flutter app and does not publish a documented CLI/API for direct invocation by this Go app, so this is catalog-based integration—not a claim that Chimera controls or launches Network Checker itself. Regional labels reflect which curated catalog was selected; they do not independently verify an IP's country. A TLS handshake is only a candidate filter; the actual tunnel health check is still required.
