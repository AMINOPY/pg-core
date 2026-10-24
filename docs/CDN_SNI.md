# CDN/SNI

Two inputs, nothing more: an **SNI** host name and a list of **edge IPs**.

```json
{
  "schema_version": 1,
  "profile": "development",
  "active_engine": "sing-box",
  "fronting": {
    "sni": "cdn.example.com",
    "edge_ips": ["203.0.113.10", "203.0.113.11"]
  }
}
```

`edge_ips` can be an array or one string separated by comma, semicolon, space or new line (`"edge_ips": "1.1.1.1, 8.8.8.8\n9.9.9.9"`). Any valid IP is accepted; there is no check against CDN ranges. Entries that are not IPs and duplicates are skipped, and only a list with no usable IP is an error. Whether an IP is alive is decided at connection time: the `urltest` outbound drops IPs that do not answer.

When both are set, the sing-box adapter rewrites every TLS outbound of the profile before `sing-box check` and `sing-box run`:

- `server` becomes the edge IP, so the server name is not resolved through DNS;
- `tls.server_name` becomes the SNI;
- with several IPs, one outbound per IP is generated (`<tag>-edge-N`) and a `urltest` outbound keeps the original tag, so routes keep working and the fastest reachable IP is used.

Everything else in the profile (protocol, credentials, transport path, WebSocket `Host`) is left as it is. Certificate verification stays on: the edge must present a valid certificate for the SNI.

```go
adapter := singbox.New("/usr/bin/sing-box")
adapter.Fronting = cfg.Fronting // *fronting.Settings, nil or empty = feature off
```

## Limits

- The profile must contain at least one TLS outbound with a `server`; otherwise start fails instead of silently ignoring the setting.
- SNI and IPs alone do not guarantee a tunnel. The CDN must route that SNI/host to your server, and the profile must use a protocol your server speaks.
- No IP scanning or Beast Mode is included.
