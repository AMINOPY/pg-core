# Engine-backed flow routing and network recovery

## Route new TCP flows through real engine listeners

`internal/fusion.RoutesFromSOCKS5` converts already-running engine SOCKS5 listeners into path-specific dialers. For example, a host can supply a sing-box listener at `127.0.0.1:1080` and an Xray listener at `127.0.0.1:1081`, construct a `fusion.Router`, and use `Router.DialContext` for outbound TCP connections. Selection, active-flow reservations, path failure handling, and bounded health refresh can then act on the connections that actually traverse those engine listeners.

`fusion.ProbeSOCKS5Path` performs an actual TCP connect through the selected engine to a caller-chosen controlled/trusted target. Use it with `fusion.RefreshRoutes` to refresh RTT and eligibility. Configure a probe target that reflects the service the app needs: a TCP connect does not prove HTTPS correctness, DNS correctness, or system-wide VPN health.

The host remains responsible for starting/stopping each engine, binding listeners to loopback, ensuring distinct ports and resource ownership, and supplying current health snapshots. Only TCP connections explicitly opened through `Router.DialContext` use this flow router; existing sockets are not migrated, and this is not packet-level bandwidth bonding.

## Transactional network restoration

`internal/netrecovery.Session` implements the lifecycle safety core for host networking:

1. Capture the original network state before applying tunnel settings.
2. Apply the desired state only after capture succeeds.
3. If apply partially fails, attempt immediate rollback.
4. On disconnect or engine failure, restore the snapshot and verify the restored state.
5. If restoration or verification fails, retain the snapshot and allow `Stop` to retry instead of falsely reporting cleanup success.

A platform `netrecovery.Backend` must snapshot and restore every resource it actually owns, including routes, DNS configuration, interface settings, and any firewall changes. `Restore` should be idempotent, and `Verify` must inspect actual state. Keep restoration operations bounded and cancellation-aware.

## Critical platform boundary

This package provides the tested transaction and rollback mechanism, **not** an operating-system backend. This repository still needs real, separately tested backends for Android `VpnService`, Linux TUN/routes/DNS, and Windows TUN/routes/DNS. Until a platform backend is installed and tested on-device/on-host, do not claim system-wide routing, DNS leak protection, crash recovery, or fail-closed enforcement. The engine-backed flow router is real for app-owned TCP dials; it does not transparently redirect arbitrary device traffic.
