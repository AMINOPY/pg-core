# Xray helper chaining

`ComposeHelperChain` combines a trusted primary Xray profile with one selected outbound from a trusted helper profile. The primary profile remains authoritative for inbounds, routing, DNS, logging, and app-facing listeners. The selected helper outbound is appended, and the selected primary outbound receives `proxySettings.tag` pointing at the helper outbound.

```go
combined, err := xray.ComposeHelperChain(primaryJSON, helperJSON, "primary-vless", "helper-vless")
```

## What this does and does not do

- Provides explicit two-profile **sequential proxy chaining**: the primary outbound dials through the helper outbound.
- Does not import helper inbounds, routing, DNS, or logging settings.
- Rejects missing/duplicate tags, ambiguous target tags, trailing JSON, and nested helper chains.
- Requires both profiles to be trusted and valid for the installed Xray version; validate the composed result with the existing adapter before starting it.
- Does **not** aggregate bandwidth across two servers. Sequential chaining may improve reachability or route quality, but often adds latency and can reduce throughput. True parallel speed aggregation needs a separate multipath/packet-scheduling data plane and server-side support; this helper does not claim to implement that.
- Does not configure OS VPN/TUN routing. The host app remains responsible for route ownership and end-to-end health checks.

Never merge untrusted configs, disable TLS certificate verification, or expose profile credentials in logs or UI metadata.
