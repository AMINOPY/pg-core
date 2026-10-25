# Multi-language fusion foundation (C + Kotlin + Go)

## Responsibilities

- **Go** is the control-plane authority: configuration, engine lifecycle, policy, health snapshots, and future session coordination.
- **C** provides a bounded native framing ABI where low-level byte handling is useful. It does not own OS VPN lifecycle or secrets.
- **Kotlin** is the Android host boundary. The app/service must own Android VPN permission and service lifecycle and delegate to exactly one TUN owner.
- The codecs share a versioned envelope so messages cross JNI, gomobile, or socket boundaries without relying on language-specific object layouts.

## PGF1 frame envelope

All integers use network byte order. The header is exactly 16 bytes: offset 0-3 magic ASCII PGF1; offset 4 version 1; offset 5 type DATA=1, PING=2, PONG=3, CLOSE=4; offset 6-7 flags; offset 8-11 unsigned sequence; offset 12-15 payload length; offset 16 onward payload, limited to 65,536 bytes.

A decoder accepts exactly one complete frame. Truncated headers, unsupported versions/types, oversized payloads, mismatched lengths, and trailing bytes are rejected. C returns a borrowed payload view; Go and Kotlin encode owned buffers.

## Implemented in this increment

- Go codec and unit tests.
- Dependency-free C11 encode/parse ABI and C test.
- Kotlin codec using ByteBuffer with matching bounds and field order.
- Shared wire-format document.

## Not production bonding yet

This envelope is **not an authenticated transport**. Do not expose it directly to untrusted networks. It has no cryptographic session handshake, replay protection, congestion control, retransmission, packet reordering, MTU negotiation, multi-path scheduler, TUN integration, or cooperating server implementation. A sequence field alone is not replay protection. Three codecs alone do not combine traffic or increase one flow's throughput.

Before network use, add authenticated session establishment with a reviewed standard protocol/library, cross-language golden vectors, socket deadlines/cancellation, replay handling, bounded reassembly, controlled two-path integration tests, and platform-specific TUN/route/DNS rollback. Keep one authoritative owner for TUN/routes/DNS.
