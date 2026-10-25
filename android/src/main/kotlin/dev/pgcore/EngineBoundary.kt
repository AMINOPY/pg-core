package dev.pgcore

/**
 * Host-app boundary for PG CORE. The Go core remains the sole owner of
 * engine lifecycle decisions; this interface deliberately does not create a
 * second connection state machine in Android UI code.
 *
 * Implementations should bridge to the host application's existing service
 * or a versioned native/JNI library. No JNI library is bundled by this file.
 */
interface EngineBoundary {
    suspend fun start(profileJson: String): EngineResult
    suspend fun stop(): EngineResult
    suspend fun status(): EngineStatus
}

data class EngineResult(
    val ok: Boolean,
    val code: String,
    val message: String? = null,
) {
    companion object {
        fun success() = EngineResult(ok = true, code = "ok")
        fun failure(code: String, message: String? = null) =
            EngineResult(ok = false, code = code, message = message)
    }
}

enum class EngineStatus {
    STOPPED,
    STARTING,
    CONNECTED,
    DEGRADED,
    STOPPING,
    FAILED,
    UNKNOWN,
}

/** Validate native ABI compatibility before enabling a JNI-backed adapter. */
object NativeAbi {
    const val EXPECTED_VERSION: Int = 1

    fun isCompatible(version: Int): Boolean = version == EXPECTED_VERSION
}
