# Xray adapter

The Xray adapter implements the shared Go engine contract for an already-installed
Xray executable. It is an optional tunnel process adapter, not an Xray distribution
or an OS VPN installer.

## Implemented

- Validate a supplied JSON profile by invoking `xray run -test -config <private-file>`.
- Store the temporary profile in a private directory with restrictive permissions.
- Start, observe, stop, and clean up the child process.
- Bound config input size and honor context cancellation during validation.
- Avoid returning upstream command output because it can include sensitive config.
- Optionally apply the existing core CDN/SNI settings to a single TLS `vnext`
  outbound. Certificate verification is not disabled.
- Optionally compose an app-facing SOCKS5 inbound at `127.0.0.1:<LocalSOCKSPort>`
  directly into the same Xray profile, so the host app can send local proxy traffic
  through the Xray outbound that received the CDN edge IP and SNI settings.
- Optionally verify the TCP data path through a loopback, no-auth SOCKS5 inbound when `Adapter.HealthTarget` is configured; the target is an explicit `host:port` and is probed after process liveness.

## Local app tunnel and CDN/SNI composition

Set `Adapter.LocalSOCKSPort` to a free port (for example `18080`) and set
`Adapter.Fronting` to the SNI plus one explicitly configured edge IP. `ValidateConfig`
and `Start` then run the same preparation pipeline: validate the inputs, rewrite the
TLS outbound address to the edge IP, set `tlsSettings.serverName`, preserve the
existing transport settings (including WebSocket path/Host), and add a SOCKS5 inbound
bound only to `127.0.0.1`. The embedding app can point its local proxy setting at
`127.0.0.1:<port>`. Set `HealthTarget` as well if you want a bounded SOCKS CONNECT
probe after startup. The port must be free; a conflicting inbound is rejected.

This connects the app to Xray through a local proxy, not through Android `VpnService`
or system-wide routing. Apps that require full-device VPN still need a real TUN packet
forwarder and platform-specific socket protection/route management. Multiple CDN IPs
should be tried as separate bounded profile attempts; the adapter intentionally accepts
one edge IP per attempt.

## Important limitations

- The adapter does not install Xray, configure OS routes, create a TUN device,
  alter system DNS, or guarantee end-to-end connectivity.
- Without `HealthTarget`, health is process-liveness-only and reports `degraded`
  (`process_liveness_only`) rather than implying the tunnel is connected.
- With `HealthTarget`, health performs a SOCKS5 CONNECT to the configured `host:port`.
  This verifies TCP reachability through the proxy, not TLS certificate validity,
  HTTP semantics, DNS privacy, full app routing, or sustained throughput. Pick a
  stable endpoint you control or are authorized to test; do not use this as a scan.
- Fronting support is intentionally limited to one configured edge IP per
  profile attempt; candidate selection and retries belong to a higher-level,
  bounded policy.
- The optional SOCKS probe requires a loopback-only, no-auth SOCKS5 inbound.
  Configurations using SOCKS authentication need an explicit credential-safe probe implementation.
  A failed configured probe reports `data_path_probe_failed`; the lifecycle manager
  then treats the engine as unhealthy and invokes its cleanup path.
- The POSIX shell-script process tests are skipped on Windows. Windows process
  lifecycle and Android packaging still require platform-specific CI/device tests.

## Example

Register the adapter through the existing engine registry, then pass it to the
orchestrator. Keep credentials in the host application's secure profile storage;
never put them in UI action metadata or diagnostics.

Do not mark the Xray engine as available just because its ID appears in a list.
Check that the executable is installed and compatible, the profile validates,
the process starts, and a tunnel-specific health check succeeds.


## Run Xray through PG CORE's CDN/SNI settings

The CLI can start an already-installed Xray executable with the core's validated
`fronting.sni` and `fronting.edge_ips` settings applied to a TLS outbound. This
reuses the existing profile's protocol, credentials, transport, WebSocket path,
and other settings; SNI and edge IPs alone cannot create a server profile.

The Xray profile must contain a valid TLS outbound and a loopback-only, no-auth
SOCKS inbound (for example `127.0.0.1:1080`). Then run:

```sh
export CHIMERA_XRAY_BINARY=/path/to/xray
export CHIMERA_XRAY_HEALTH_TARGET=example.com:443
go run ./cmd/chimera --run --config config.json --profile xray-profile.json
```

If Xray is on `PATH`, `CHIMERA_XRAY_BINARY` may be omitted. On Windows
PowerShell, use `$env:CHIMERA_XRAY_BINARY` and
`$env:CHIMERA_XRAY_HEALTH_TARGET`. The health target should be a stable host
and port you control or are authorized to test. It checks TCP reachability
through the local SOCKS5 inbound; it does not prove system-wide routing or DNS
privacy.

This CLI path does not install Xray, create a TUN device, or change system
routes/DNS. Apps must explicitly use the printed local SOCKS5 endpoint until
platform-specific VPN/TUN integration is implemented and tested. A saved profile
and server credentials are still required; this is not a zero-information
connection method.
