# Java, Python, and Kotlin bindings

Chimera's policy core remains implemented in Go. This repository now includes
small client facades for Java, Python, and Kotlin so applications can connect
their own host transport to the same evaluate / confirm / cancel workflow.

## Available scaffolds

- Python: `integrations/python/chimera_core.py` — accepts a transport callable
  and validates the returned decision JSON.
- Java: `integrations/java/src/main/java/com/aminopy/chimera/ChimeraCoreClient.java`
  — accepts an application-provided bridge.
- Kotlin: `integrations/kotlin/src/main/kotlin/com/aminopy/chimera/ChimeraCoreClient.kt`
  — accepts an application-provided bridge.

## Important status

These are integration scaffolds, not complete native bindings. They do not
embed the Go runtime, provide a running local service, configure Android
routes/DNS, or own a VPN session. The host application must implement the
bridge against the exact generated gomobile AAR or another explicitly designed
transport. Generated gomobile signatures must be inspected rather than guessed.
The Java and Kotlin APIs return the raw decision JSON; callers must parse it
and must only confirm a proposed switch after independent connectivity
verification. Never send credentials, raw share links, or full profile contents
through the policy bridge.

For the underlying JSON schema and safe switch sequence, see
[ANDROID_MOBILE_BRIDGE.md](ANDROID_MOBILE_BRIDGE.md).
