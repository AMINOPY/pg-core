# sing-box adapter — Phase B

Chimera's first engine adapter wraps an already installed sing-box CLI and implements the shared `engine.Adapter` contract.

## Implemented

- Validate a profile using `sing-box check -c <config>`.
- Store temporary profile files in a private directory (directory mode 0700, file mode 0600).
- Start `sing-box run -c <config>` and report child-process liveness through the common health report.
- Optionally verify TCP reachability through a loopback, no-auth SOCKS5 inbound when `Adapter.HealthTarget` is configured (explicit `host:port`).
- Prevent duplicate concurrent starts, support cancellation during validation, stop the child process, and remove the temporary profile.
- Limit profile input to 4 MiB and avoid returning raw validation output, which may contain secrets.
- Unit tests use a fake executable, so CI does not require sing-box to be installed.

The expected input is a complete JSON profile accepted by the installed sing-box version. The upstream configuration documentation lists the JSON structure and the `check` command: https://sing-box.sagernet.org/configuration/ .

## Example

```go
adapter := singbox.New("/usr/bin/sing-box")
manager, err := orchestrator.New(adapter)
if err != nil {
    return err
}
profile, err := os.ReadFile("profile.json")
if err != nil {
    return err
}
if err := manager.Start(ctx, profile); err != nil {
    return err
}
```

## Boundaries and remaining work

This adapter does not install or pin sing-box, elevate privileges, configure system routes, install OS DNS settings, or create a system TUN device itself. It delegates whatever behavior the supplied profile requests to sing-box. Without `HealthTarget`, a live child process is only a process-level health signal, not proof that DNS, routes, or end-to-end connectivity work. With `HealthTarget`, the adapter performs a SOCKS5 CONNECT to the configured `host:port`; this checks TCP reachability through the proxy, not TLS validity, HTTP semantics, DNS privacy, full app routing, or throughput. Configure only a stable endpoint you control or are authorized to test. A failed configured probe reports `data_path_probe_failed`, allowing the manager to invoke its cleanup path. There is no kill-switch guarantee.

The optional probe requires an explicit loopback SOCKS inbound without configured users; authenticated SOCKS and non-loopback listeners are rejected for this health-check path. Before production use, add platform-specific integration tests for route/DNS ownership and rollback, define privilege requirements, pin and test supported sing-box versions, and verify real connectivity on the supported OS. Do not run multiple engines as competing owners of the same traffic path.
