# Profile import compatibility

PG CORE's Xray runner accepts a deliberately bounded set of profile inputs through
`--profile`. Import conversion is performed locally and rejects formats or
options that cannot be mapped without losing connection semantics.

## Supported inputs

- Xray JSON with at least one outbound.
- NPV Tunnel JSON exports/wrappers that contain a non-empty embedded
  `v2rayProfile.v2rayJson` (or top-level `v2rayJson`) containing Xray JSON.
- Standard `vless://`, `vmess://`, `trojan://`, and basic `ss://` share links.
- For converted profiles without a suitable inbound, PG CORE adds a SOCKS5
  listener bound only to `127.0.0.1:1080`.

The converter currently supports TCP, WebSocket, gRPC, and HTTP/HTTP2 stream
mappings plus TLS and REALITY fields where supplied by the share link. It
rejects unsupported transport modes and Shadowsocks plugin options rather than
silently omitting them. Review the generated/imported profile and use a trusted
server configuration.

## Not yet supported

- SlipNet `slipnet://` URIs. SlipNet profiles are protocol-specific and must run
  through a compatible SlipNet runtime; they cannot safely be treated as Xray
  JSON. PG CORE reports this as an explicit error instead of trying to launch
  the wrong engine.
- Password-protected or proprietary `.npvt` / NPVS containers. Import/export
  needs a verified format implementation and, where applicable, the owner's
  password. Do not upload private profile files to online converters.
- NPV profile wrappers that only contain native fields and no embedded Xray
  JSON, or wrappers containing multiple profiles. Select one profile and export
  it to a supported URI or Xray JSON first.
- DNS tunnel, SSH, NaiveProxy, WireGuard, Hysteria2, or other native-only
  SlipNet/NPV modes unless the input is represented in one of the supported
  Xray-compatible formats above.

## Run a supported profile

Use the normal PG CORE configuration with `active_engine: "xray"` and point
`--profile` at a supported JSON or share-link text file. The installed Xray
binary is still required (optionally selected with `CHIMERA_XRAY_BINARY`).
The runner exposes only a local SOCKS endpoint; it does not create an OS VPN,
install TUN, or configure system routes/DNS. Process liveness alone is not proof
of end-to-end tunnel connectivity.
