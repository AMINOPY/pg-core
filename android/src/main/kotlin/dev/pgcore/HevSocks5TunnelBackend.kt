package dev.pgcore

import java.io.File
import java.io.IOException
import java.lang.reflect.InvocationTargetException
import java.lang.reflect.Method
import java.util.concurrent.atomic.AtomicBoolean

/**
 * Narrow boundary around the upstream Hev JNI API. This makes lifecycle
 * behavior testable without loading native code on a JVM test runner.
 */
interface HevTunnelApi {
    fun start(configPath: String, tunFd: Int): Boolean
    fun stop(): Boolean
    fun isRunning(): Boolean
}

/**
 * Adapter for the public API shipped in heiher/hev-socks5-tunnel Android AAR.
 * The host must add a pinned, license-preserving AAR to its Android build.
 */
class ReflectiveHevTunnelApi(
    private val className: String = "hev.htproxy.TProxyService",
) : HevTunnelApi {
    private val apiClass by lazy { Class.forName(className) }
    private fun method(name: String, vararg types: Class<*>): Method =
        apiClass.getMethod(name, *types)

    override fun start(configPath: String, tunFd: Int): Boolean =
        invokeBoolean("TProxyStartService", arrayOf(String::class.java, Int::class.javaPrimitiveType!!), configPath, tunFd)

    override fun stop(): Boolean = invokeBoolean("TProxyStopService", emptyArray())

    override fun isRunning(): Boolean = invokeBoolean("TProxyIsRunning", emptyArray())

    private fun invokeBoolean(name: String, types: Array<Class<*>>, vararg args: Any): Boolean {
        try {
            return method(name, *types).invoke(null, *args) as? Boolean ?: false
        } catch (e: InvocationTargetException) {
            throw IllegalStateException("Hev JNI call $name failed", e.targetException)
        } catch (e: ReflectiveOperationException) {
            throw IllegalStateException("Hev JNI API $name is unavailable; verify the pinned AAR version", e)
        } catch (e: LinkageError) {
            throw IllegalStateException("Hev native library could not be loaded", e)
        }
    }
}

/**
 * Starts the host's protected SOCKS engine, then Hev's TUN-to-SOCKS data plane.
 *
 * This backend is intentionally not registered automatically: the host must
 * supply a real EngineBridge and package the pinned Hev AAR. The service retains
 * ownership of tunFd; this class must never close it.
 */
class HevSocks5TunnelBackend(
    cacheDir: File,
    private val engine: PgCoreSocksEngine,
    private val api: HevTunnelApi = ReflectiveHevTunnelApi(),
) : PgCoreVpnBackend {
    private val configFile: File = File(cacheDir, "pgcore-hev-tun.yml")
    private val running = AtomicBoolean(false)
    @Volatile private var engineStarted = false
    @Volatile private var nativeStarted = false

    override fun start(
        tunFd: Int,
        config: VpnBackendConfig,
        protectSocket: (socketFd: Int) -> Boolean,
    ): EngineResult {
        if (running.get() || engineStarted || nativeStarted) {
            return EngineResult.failure("backend_already_started", "Hev backend is already active")
        }
        if (config.engineProfileJson.isBlank()) {
            return EngineResult.failure("missing_engine_profile", "A native engine profile is required")
        }

        try {
            val engineResult = engine.start(config.engineProfileJson, protectSocket)
            if (!engineResult.ok) return engineResult
            engineStarted = true

            val endpoint = engine.socksEndpoint()
            val socks = parseLoopbackEndpoint(endpoint)
                ?: throw IOException("Engine returned an invalid loopback SOCKS endpoint")
            configFile.writeText(buildConfig(config, socks.first, socks.second))
            if (!api.start(configFile.absolutePath, tunFd)) {
                // Native startup may partially succeed even when its return value is false.
                nativeStarted = runCatching { api.isRunning() }.getOrDefault(true)
                throw IOException("Hev refused to start")
            }
            nativeStarted = true
            if (!api.isRunning()) throw IOException("Hev did not report a running data plane")

            val engineHealth = engine.endToEndHealthCheck()
            if (!engineHealth.ok) throw IOException(engineHealth.message ?: engineHealth.code)
            running.set(true)
            return EngineResult.success()
        } catch (e: Exception) {
            val cleanup = stop()
            val suffix = if (!cleanup.ok) "; cleanup also failed: ${cleanup.message}" else ""
            return EngineResult.failure("hev_backend_start_failed", (e.message ?: "Backend startup failed") + suffix)
        }
    }

    override fun stop(): EngineResult {
        var failure: String? = null
        if (nativeStarted) {
            try {
                if (!api.stop() && api.isRunning()) failure = "Hev did not stop cleanly"
                else nativeStarted = false
            } catch (e: Exception) {
                failure = "Hev stop failed: ${e.message}"
            }
        }
        // Do not stop the engine while Hev may still be using its SOCKS endpoint.
        if (!nativeStarted && engineStarted) {
            try {
                val result = engine.stop()
                if (result.ok) engineStarted = false
                else failure = listOfNotNull(failure, result.message ?: result.code).joinToString("; ")
            } catch (e: Exception) {
                failure = listOfNotNull(failure, "Engine stop failed: ${e.message}").joinToString("; ")
            }
        }
        running.set(false)
        if (!nativeStarted && !engineStarted) {
            runCatching { configFile.delete() }
        }
        return if (failure == null && !nativeStarted && !engineStarted) EngineResult.success()
        else EngineResult.failure("hev_backend_stop_failed", failure ?: "Backend resources remain active; retry stop")
    }

    override fun healthCheck(): EngineResult {
        if (!running.get() || !nativeStarted || !engineStarted) {
            return EngineResult.failure("hev_backend_not_running", "TUN data plane is not fully started")
        }
        return try {
            if (!api.isRunning()) {
                running.set(false)
                EngineResult.failure("hev_tunnel_not_running", "Hev no longer reports a running tunnel")
            } else {
                val result = engine.endToEndHealthCheck()
                if (!result.ok) running.set(false)
                result
            }
        } catch (e: Exception) {
            running.set(false)
            EngineResult.failure("hev_health_check_failed", e.message)
        }
    }

    private fun buildConfig(config: VpnBackendConfig, socksHost: String, socksPort: Int): String {
        // Values are constants or validated numeric values, never user-provided YAML.
        return buildString {
            appendLine("tunnel:")
            appendLine("  mtu: ${config.mtu}")
            if (config.enableIpv4) appendLine("  ipv4: 10.111.0.1")
            if (config.enableIpv6) appendLine("  ipv6: 'fd42:7067:636f:7265::1'")
            appendLine("  icmp: 'off'")
            appendLine("socks5:")
            appendLine("  address: $socksHost")
            appendLine("  port: $socksPort")
            appendLine("  udp: 'udp'")
            appendLine("misc:")
            appendLine("  log-level: error")
        }
    }

    private fun parseLoopbackEndpoint(value: String): Pair<String, Int>? {
        if (value.isBlank() || value.any(Char::isWhitespace)) return null
        val uri = runCatching { java.net.URI("tcp://$value") }.getOrNull() ?: return null
        if (uri.rawUserInfo != null || !uri.rawPath.isNullOrEmpty() || uri.rawQuery != null || uri.rawFragment != null) return null
        val host = uri.host?.removeSurrounding("[", "]") ?: return null
        val port = uri.port
        if (port !in 1..65535) return null
        val loopback = host.equals("localhost", true) ||
            host == "127.0.0.1" || host == "::1" || host == "[::1]"
        if (!loopback) return null
        return (if (host == "[::1]") "::1" else host) to port
    }
}

/**
 * Implement this with the pinned gomobile/JNI binding for package mobile.
 * It must protect each upstream socket before connect, must not own the TUN fd,
 * and healthCheck() must verify end-to-end traffic through TUN, not just a live upstream.
 */
interface PgCoreSocksEngine {
    fun start(profileJson: String, protectSocket: (socketFd: Int) -> Boolean): EngineResult
    fun socksEndpoint(): String
    fun endToEndHealthCheck(): EngineResult
    fun stop(): EngineResult
}
