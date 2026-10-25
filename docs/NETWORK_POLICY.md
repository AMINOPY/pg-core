# Cross-platform network policy contract

The `internal/networkpolicy` package defines a validated, OS-neutral policy
contract for future Android/Linux/Windows adapters. It intentionally does not
modify routes, DNS settings, firewall rules, or TUN devices.

## Policy invariants

- Tunnel DNS cannot be selected with direct routing.
- Fail-closed mode cannot be declared for direct routing.
- Address-family choice is explicit (dual-stack, IPv4-only, or IPv6-only).
- MTU and timeout values are bounded before an adapter can consume them.
- A valid policy is **not** proof of enforcement. An OS adapter must apply the
  policy, report enforcement status, restore prior state on shutdown/crash, and
  pass platform integration and leak tests before fail-closed can be advertised.

## Platform readiness

- **Android:** requires a host application using `VpnService`, user consent,
  lifecycle handling, and tests for network changes and always-on VPN behavior.
- **Linux:** requires a documented privilege model plus TUN, route, DNS, and
  firewall integration with rollback.
- **Windows:** requires a supported TUN/Wintun deployment model, route/DNS
  integration, service lifecycle, and rollback tests.

## DPI resistance and performance

DPI resistance is a transport/engine capability, not a boolean that this
coordinator can guarantee. Use a maintained engine and explicitly configured,
supported transports; test against the actual network and jurisdiction. No
fingerprint or obfuscation method works universally. Speed improvements must be
measured end-to-end: route selection, DNS latency, connection setup, loss, and
throughput. Avoid claiming that changing MTU, DNS, or protocol automatically
increases speed. This repository currently has no verified cross-platform
traffic path, so the policy contract alone does not provide anti-DPI or
throughput improvements.
