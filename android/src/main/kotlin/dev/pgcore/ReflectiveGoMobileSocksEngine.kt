package dev.pgcore

import java.lang.reflect.InvocationTargetException
import java.lang.reflect.Proxy

/**
 * Adapter for the generated gomobile AAR (package dev.pgcore.mobile.mobile).
 *
 * The consuming host must package the generated AAR and the pinned Hev AAR.
 * An explicit endToEndProbe is mandatory: Engine.health() only checks the
 * native engine/upstream and cannot prove that packets traverse Android TUN.
 */
class ReflectiveGoMobileSocksEngine(
    private val endToEndProbe: () -> EngineResult,
    private val engineClassName: String = "dev.pgcore.mobile.mobile.Engine",
    private val protectorInterfaceName: String = "dev.pgcore.mobile.mobile.SocketProtector",
    private val stopTimeoutMillis: Long = 5_000,
    private val healthTimeoutMillis: Long = 5_000,
) : PgCoreSocksEngine {
    private var engine: Any? = null
    private var protectorProxy: Any? = null
    private var started = false

    @Synchronized
    override fun start(
        profileJson: String,
        protectSocket: (socketFd: Int) -> Boolean,
    ): EngineResult {
        if (started || engine != null) {
            return EngineResult.failure("mobile_engine_already_started", "Native engine is already active")
        }
        if (profileJson.isBlank()) {
            return EngineResult.failure("missing_engine_profile", "A native engine profile is required")
        }
        return try {
            val engineClass = Class.forName(engineClassName)
            val protectorClass = Class.forName(protectorInterfaceName)
            val instance = engineClass.getDeclaredConstructor().newInstance()
            val callback = Proxy.newProxyInstance(
                protectorClass.classLoader,
                arrayOf(protectorClass),
            ) { proxy, method, args ->
                when (method.name) {
                    "protect" -> {
                        val rawFd = (args?.firstOrNull() as? Number)?.toLong()
                            ?: return@newProxyInstance false
                        rawFd in 0..Int.MAX_VALUE.toLong() && protectSocket(rawFd.toInt())
                    }
                    "toString" -> "PgCoreVpnSocketProtector"
                    "hashCode" -> System.identityHashCode(proxy)
                    "equals" -> proxy === args?.firstOrNull()
                    else -> throw UnsupportedOperationException("Unsupported protector method: ${method.name}")
                }
            }
            try {
                engineClass.getMethod("start", String::class.java, protectorClass)
                    .invoke(instance, profileJson, callback)
            } catch (e: InvocationTargetException) {
                throw (e.targetException ?: e)
            }
            engine = instance
            protectorProxy = callback
            started = true
            EngineResult.success()
        } catch (e: Exception) {
            engine = null
            protectorProxy = null
            started = false
            EngineResult.failure("mobile_engine_start_failed", e.cause?.message ?: e.message ?: "Native engine failed to start")
        } catch (e: LinkageError) {
            EngineResult.failure("mobile_engine_unavailable", "Generated Go AAR or native ABI is missing: ${e.message}")
        }
    }

    @Synchronized
    override fun socksEndpoint(): String {
        val instance = engine ?: throw IllegalStateException("Native engine is not started")
        return try {
            instance.javaClass.getMethod("addr").invoke(instance) as? String
                ?: throw IllegalStateException("Native engine returned no SOCKS endpoint")
        } catch (e: InvocationTargetException) {
            throw IllegalStateException("Could not read native SOCKS endpoint", e.targetException)
        }
    }

    @Synchronized
    override fun endToEndHealthCheck(): EngineResult {
        val instance = engine ?: return EngineResult.failure("mobile_engine_not_started", "Native engine is not started")
        return try {
            instance.javaClass.getMethod("health", Long::class.javaPrimitiveType)
                .invoke(instance, healthTimeoutMillis)
            val probe = endToEndProbe()
            if (probe.ok) probe else EngineResult.failure(
                probe.code.ifBlank { "tun_data_plane_unverified" },
                probe.message ?: "End-to-end TUN traffic probe failed",
            )
        } catch (e: InvocationTargetException) {
            EngineResult.failure("mobile_engine_health_failed", e.targetException?.message ?: "Native health check failed")
        } catch (e: Exception) {
            EngineResult.failure("mobile_engine_health_failed", e.message ?: "Native health check failed")
        } catch (e: LinkageError) {
            EngineResult.failure("mobile_engine_unavailable", "Generated Go AAR or native ABI is missing: ${e.message}")
        }
    }

    @Synchronized
    override fun stop(): EngineResult {
        val instance = engine ?: run {
            started = false
            protectorProxy = null
            return EngineResult.success()
        }
        return try {
            instance.javaClass.getMethod("stop", Long::class.javaPrimitiveType)
                .invoke(instance, stopTimeoutMillis)
            engine = null
            protectorProxy = null
            started = false
            EngineResult.success()
        } catch (e: InvocationTargetException) {
            // Retain the object and callback so callers can retry shutdown.
            EngineResult.failure("mobile_engine_stop_failed", e.targetException?.message ?: "Native engine did not stop")
        } catch (e: Exception) {
            EngineResult.failure("mobile_engine_stop_failed", e.message ?: "Native engine did not stop")
        } catch (e: LinkageError) {
            EngineResult.failure("mobile_engine_unavailable", "Generated Go AAR or native ABI is missing: ${e.message}")
        }
    }
}
