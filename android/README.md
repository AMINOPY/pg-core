# Android Kotlin boundary

`src/main/kotlin/dev/pgcore/EngineBoundary.kt` defines a UI-neutral suspend
interface for an Android host app to forward start/stop/status requests to
the existing PG CORE lifecycle owner.

This is an integration contract, not a complete Android application module:
the repository does not yet include a Gradle project, foreground service,
VPN permission flow, JNI binding, or Android instrumentation tests. A host app
must implement the interface using its existing service/lifecycle owner and
must not run a competing engine state machine. Do not report `CONNECTED`
until a data-path health check has succeeded.
