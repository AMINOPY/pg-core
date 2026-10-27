package dev.pgcore

import java.lang.reflect.InvocationHandler
import java.lang.reflect.InvocationTargetException
import java.lang.reflect.Proxy

/**
 * Must exercise traffic through the device VPN (not directly through the SOCKS
 * endpoint). A host app should implement bounded TCP, UDP and DNS probes and
 * fail unless each result is independently verified.
 */
fun interface VpnTrafficProbe {
    fun verify(tunFd: Int, config: VpnBackendConfig, timeoutMillis: Long): EngineResult
}

/**
 * Concrete bridge to the gomobile AAR built by android-mobile-bridge.yml.
 *
 * Reflection avoids making this Android library falsely appear self-contained:
 * the consuming app must package the generated pgcore mobile AAR and the pinned
 * Hev AAR. A real VpnTrafficProbe is mandatory; native process health alone is
 * deliberately insufficient to report a connected tunnel.
 */
class GoMobilePgCoreSocksEngine(
    private val trafficProbe: VpnTrafficProbe,
    private val classLoader: ClassLoader = Thread.currentThread().contextClassLoader
        ?: GoMobilePgCoreSocksEngine::class.java.classLoader!!,
    private val engineClassName: String = "dev.pgcore.mobile.mobile.Engine",
    private val protectorClassName: String = "dev.pgcore.mobile.mobile.SocketProtector",
    private val probeTimeoutMillis: Long = 8_000,
) : PgCoreSocksEngine {
    private var nativeEngine: Any? = null
    private var tunFd: Int? = null
    private var activeConfig: VpnBackendConfig? = null

    init {
        require(probeTimeoutMillis in 1..60_000) { "probe timeout must be between 1 and 60000 ms" }
    }

    @Synchronized
    override fun start(profileJson: String, protectSocket: (socketFd: Int) -> Boolean): EngineResult {
        if (nativeEngine != null) return EngineResult.failure("engine_already_started")
        if (profileJson.isBlank()) return EngineResult.failure("missing_engine_profile")
        return try {
            val engineClass = Class.forName(engineClassName, true, classLoader)
            val protectorClass = Class.forName(protectorClassName, true, classLoader)
            val engine = engineClass.getDeclaredConstructor().newInstance()
            val handler = InvocationHandler { _, method, args ->
                when (method.name) {
                    "protect" -> {
                        val fd = (args?.firstOrNull() as? Number)?.toLong()
                            ?: return@InvocationHandler false
                        if (fd !in 0..Int.MAX_VALUE.toLong()) false else protectSocket(fd.toInt())
                    }
                    "toString" -> "PgCoreVpnSocketProtector"
                    "hashCode" -> System.identityHashCode(this)
                    "equals" -> args?.firstOrNull() === this
                    else -> throw UnsupportedOperationException("Unsupported socket protector method: ${method.name}")
                }
            }
            val protector = Proxy.newProxyInstance(classLoader, arrayOf(protectorClass), handler)
            engineClass.getMethod("start", String::class.java, protectorClass)
                .invoke(engine, profileJson, protector)
            val address = engineClass.getMethod("addr").invoke(engine) as? String
            if (address.isNullOrBlank()) {
                runCatching { engineClass.getMethod("stop", Long::class.javaPrimitiveType).invoke(engine, 3_000L) }
                EngineResult.failure("native_engine_missing_socks_endpoint")
            } else {
                nativeEngine = engine
                EngineResult.success()
            }
        } catch (e: InvocationTargetException) {
            EngineResult.failure("native_engine_start_failed", e.targetException.message ?: e.targetException.javaClass.simpleName)
        } catch (e: ReflectiveOperationException) {
            EngineResult.failure("native_engine_bridge_unavailable", e.message ?: e.javaClass.simpleName)
        } catch (e: LinkageError) {
            EngineResult.failure("native_engine_library_unavailable", e.message ?: e.javaClass.simpleName)
        } catch (e: Exception) {
            EngineResult.failure("native_engine_start_failed", e.message ?: e.javaClass.simpleName)
        }
    }

    @Synchronized
    override fun socksEndpoint(): String {
        val engine = nativeEngine ?: throw IllegalStateException("Native engine is not started")
        return invokeString(engine, "addr")
    }

    @Synchronized
    override fun endToEndHealthCheck(): EngineResult {
        val engine = nativeEngine ?: return EngineResult.failure("native_engine_not_started")
        val fd = tunFd ?: return EngineResult.failure("tun_probe_context_missing")
        val config = activeConfig ?: return EngineResult.failure("tun_probe_context_missing")
        return try {
            invoke(engine, "health", longArrayOf(probeTimeoutMillis))
            trafficProbe.verify(fd, config, probeTimeoutMillis)
        } catch (e: Exception) {
            EngineResult.failure("native_engine_health_failed", rootMessage(e))
        }
    }

    /** Called by the backend after native engine start and before its health probe. */
    @Synchronized
    fun attachTunnelContext(fd: Int, config: VpnBackendConfig) {
        require(fd >= 0) { "TUN fd must be non-negative" }
        tunFd = fd
        activeConfig = config
    }

    @Synchronized
    override fun stop(): EngineResult {
        val engine = nativeEngine ?: run {
            tunFd = null
            activeConfig = null
            return EngineResult.success()
        }
        return try {
            invoke(engine, "stop", longArrayOf(5_000L))
            nativeEngine = null
            tunFd = null
            activeConfig = null
            EngineResult.success()
        } catch (e: Exception) {
            // Keep the reference so callers can retry cleanup.
            EngineResult.failure("native_engine_stop_failed", rootMessage(e))
        }
    }

    private fun invokeString(target: Any, name: String): String {
        return target.javaClass.getMethod(name).invoke(target) as? String
            ?: throw IllegalStateException("Native method $name returned no string")
    }

    private fun invoke(target: Any, name: String, args: LongArray) {
        val method = target.javaClass.getMethod(name, Long::class.javaPrimitiveType)
        try {
            method.invoke(target, args[0])
        } catch (e: InvocationTargetException) {
            throw e.targetException
        }
    }

    private fun rootMessage(error: Throwable): String {
        var current = error
        while (current is InvocationTargetException && current.targetException != null) {
            current = current.targetException
        }
        return current.message ?: current.javaClass.simpleName
    }
}
