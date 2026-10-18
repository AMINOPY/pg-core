# Java, Python, and Kotlin clients

Chimera's policy core remains implemented in Go. The repository provides small
clients for Java, Python, and Kotlin that share the same evaluate / confirm /
cancel protocol.

## Client source and tests

- Python: `integrations/python/chimera_core.py`
- Java: `integrations/java/src/main/java/com/aminopy/chimera/ChimeraCoreClient.java`
- Kotlin: `integrations/kotlin/src/main/kotlin/com/aminopy/chimera/ChimeraCoreClient.kt`

Run the contract tests independently:

```sh
(cd integrations/python && python -m unittest -v)
(cd integrations/java && mvn --batch-mode test)
(cd integrations/kotlin && gradle test)
```

GitHub Actions runs these alongside the Go format, vet, and test checks.

## Transport contract

Each client accepts or implements a host transport. The transport receives one
of these method names:

- `evaluate` with `sample`, a JSON string following the schema in
  [ANDROID_MOBILE_BRIDGE.md](ANDROID_MOBILE_BRIDGE.md).
- `confirmSwitch` with `targetId`, called only after the platform switches
  and independent connectivity verification succeeds.
- `cancelSwitch` with an empty payload, when a proposal is abandoned.

The Python client parses and validates the decision object. Java and Kotlin
return the raw decision JSON so the calling application can parse it with its
chosen JSON library. Callers must not confirm a switch merely because the
policy suggested one or because a VPN plugin reports that startup completed.

## What is not included yet

These clients are tested protocol facades, not complete native bindings. They
do not embed the Go runtime, start a local bridge service, configure Android
routes/DNS, or own a VPN session. The host application must implement the
transport against the exact generated gomobile AAR or another explicitly
designed transport. Generated gomobile signatures must be inspected rather
than guessed. Android device tests are still required for permissions, network
changes, routing, DNS, and rollback.

Never send credentials, raw share links, or full profile contents through the
policy bridge. The bridge should receive only bounded, sanitized health samples.
