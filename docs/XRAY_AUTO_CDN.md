# Xray Auto CDN integration

`xray.PrepareAutoCDNProfile` connects the existing regional catalog scanner to the Xray profile rewriter.

## Flow

1. Load the operator-curated catalog for the chosen mode: general Auto, Auto IR, or Auto US.
2. Run bounded TLS probes with certificate verification enabled and rank the SNI/IP pairs by measured response time.
3. Rewrite only the TLS endpoint address and TLS SNI in the existing Xray profile. The protocol, UUID/credentials, transport, WebSocket Host/path, routing, and other profile fields are retained.
4. Start the resulting profile with the normal Xray adapter.
5. Report Connected only after a real tunnel/data-path health check passes. A TLS probe alone is not a successful VPN connection.

Example call:

```go
prepared, selected, err := xray.PrepareAutoCDNProfile(
    ctx, existingXrayProfileJSON, regionalCatalog,
    autocdn.Scanner{}, nil, // nil probe uses the built-in bounded TLS/HTTPS probe
)
if err != nil {
    return err
}
// Validate/start "prepared" with the ordinary Xray adapter and run the
// configured end-to-end health check before showing Connected.
_ = selected
```

## Important constraints

- Auto IR and Auto US use separate, trusted catalogs; the mode name is not proof of geolocation. Populate those catalogs with endpoints you administer or are authorized to test.
- Manual CDN/SNI remains supported by fronting.Settings and the existing Xray adapter path.
- CDN/SNI is not a complete tunnel configuration. A valid Xray server endpoint, protocol, credentials, and transport are still required. The function intentionally fails if the profile is absent or contains no supported TLS outbound.
- The selected route is a candidate, not a guarantee. If the origin/CDN routing or transport does not work, Xray/data-path validation must fail closed.
- This helper does not alter the Android TUN backend, Auto IR/US catalog files, or existing engine registration defaults.
