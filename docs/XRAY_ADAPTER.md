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
