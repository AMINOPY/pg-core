# Tunnel performance tuning

The native TLS/WebSocket tunnel uses 64 KiB WebSocket data frames to reduce
per-frame header, masking, and write-call overhead compared with the previous
32 KiB chunk size. This is a conservative implementation default, not a
universal optimum: real throughput depends on RTT, loss, CPU, TLS termination,
the CDN path, and the target server.

## Reproducible checks

Run the correctness suite and benchmark on the target machine:

```sh
go test ./internal/engine/native
go test -run '^$' -bench '^BenchmarkWebSocketTunnelWrite$' -benchmem ./internal/engine/native
go test -race ./internal/engine/native
```

Compare the benchmark before and after changes on the same machine and Go
version. Record `ns/op`, `MB/s`, `B/op`, and `allocs/op`. This in-memory
benchmark measures framing/masking overhead only; it does not model real CDN
latency or internet throughput.

## Safe tuning rules

- Keep TLS certificate verification enabled and keep the configured SNI.
- Do not disable TCP/TLS security checks to chase speed.
- Keep the edge failover order and last-known-good edge behavior.
- Tune frame size only against benchmark results on representative devices.
- Do not report a speed gain as verified until the benchmark and an end-to-end
  transfer test have run on the target platform.
