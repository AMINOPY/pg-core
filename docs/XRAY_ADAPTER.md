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
- Optionally verify the TCP data path through a loopback, no-auth SOCKS5 inbound when `Adapter.HealthTarget` is configured; the target is an explicit `host:port` and is probed after process liveness.

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


## Run Xray through the core's local SOCKS tunnel

The CLI can launch an Xray profile through the same shared configuration that carries CDN/SNI settings. This uses the installed Xray binary and the profile's existing protocol credentials; SNI and edge IPs do not create a usable Xray account by themselves.

Example config.xray.json:

```json
{
  "schema_version": 1,
  "profile": "xray-fronting",
  "active_engine": "xray",
  "fronting": {
    "sni": "cdn.example.com",
    "edge_ips": ["203.0.113.10"]
  }
}
```

The separate Xray profile passed to --profile must contain a TLS-enabled single-server outbound supported by the adapter and an explicit loopback-only SOCKS inbound, for example 127.0.0.1:1080 with protocol socks and no authentication. Keep protocol credentials private. The adapter rewrites the outbound server address to the chosen edge IP and tlsSettings.serverName to the configured SNI while preserving the rest of the transport profile, including WebSocket path and Host header.

```sh
# Optional if the executable is not named xray or is not on PATH:
export CHIMERA_XRAY_BINARY=/path/to/xray
go run ./cmd/chimera --config config.xray.json --run --profile ./private-xray-profile.json
```

Set CHIMERA_XRAY_HEALTH_TARGET=example.com:443 only when you want a TCP CONNECT check through the loopback no-auth SOCKS inbound. Without it, the report intentionally says process-liveness-only/degraded. The CLI prints the local SOCKS address so a desktop app can opt into it. It does not install system routes, TUN, or Android VpnService.

## Relationship to Quick Connect

--quick-connect remains the no-profile, two-field native TLS/WebSocket tunnel and requires CHIMERA_TUNNEL_KEY plus a matching server/Worker. It is not silently switched to Xray: Xray needs a real Xray profile with protocol and authentication settings. The CLI's --run path is the direct Xray integration when such a profile is available.
