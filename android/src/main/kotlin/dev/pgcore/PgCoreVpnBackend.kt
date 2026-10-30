package dev.pgcore

import java.net.InetAddress
import java.net.URI

/**
 * Data-plane contract for the Android VpnService.
 * The backend must forward packets between TUN and a real engine; process liveness is insufficient.
 * The fd is borrowed: the service owns and closes it after stop() completes.
 */
interface PgCoreVpnBackend {
    /**
     * Return only after packet forwarding is ready. Any socket used to reach an
     * upstream engine must be protected via protectSocket before connecting,
     * or it may be routed back into this VPN and create a loop.
     */
    fun start(
        tunFd: Int,
        config: VpnBackendConfig,
        protectSocket: (socketFd: Int) -> Boolean,
    ): EngineResult
    /** Stop workers and release sockets; safe to call more than once. */
    fun stop(): EngineResult
    /** Perform a bounded end-to-end data-plane probe. */
    fun healthCheck(): EngineResult
}

data class VpnBackendConfig(
    val sessionId: String,
    val mtu: Int = DEFAULT_MTU,
    val enableIpv4: Boolean = true,
    val enableIpv6: Boolean = true,
    val dnsServers: List<String> = emptyList(),
    val paths: List<VpnPathConfig> = emptyList(),
    /** JSON profile consumed by the host-provided native engine bridge. */
    val engineProfileJson: String = "",
) {
    init {
        require(sessionId.isNotBlank()) { "sessionId must not be blank" }
        require(engineProfileJson.length <= MAX_ENGINE_PROFILE_CHARS) { "engine profile is too large" }
        require(mtu in MIN_MTU..MAX_MTU) { "mtu out of range" }
        require(enableIpv4 || enableIpv6) { "at least one IP family must be enabled" }
        require(paths.size <= MAX_PATHS) { "too many paths" }
        require(paths.map { it.id }.distinct().size == paths.size) { "duplicate path id" }
        require(paths.all { it.id.isNotBlank() && it.endpoint.isNotBlank() }) {
            "path id and endpoint must not be blank"
        }
    }
    companion object {
        const val DEFAULT_MTU = 1500
        const val MIN_MTU = 576
        const val MAX_MTU = 9000
        const val MAX_PATHS = 8
        const val MAX_ENGINE_PROFILE_CHARS = 262144
    }
}

data class VpnPathConfig(
    val id: String,
    /** Engine-owned local SOCKS/transport endpoint, not an arbitrary remote URL. */
    val endpoint: String,
    val weight: Int = 1,
    val enabled: Boolean = true,
) {
    init {
        require(weight in 1..100) { "weight must be between 1 and 100" }
        require(isLoopbackEndpoint(endpoint)) {
            "endpoint must be a loopback host:port; remote endpoints are not allowed"
        }
    }

    private fun isLoopbackEndpoint(value: String): Boolean {
        if (value.isBlank() || value.any { it.isWhitespace() }) return false
        val uri = runCatching { URI("tcp://$value") }.getOrNull() ?: return false
        if (uri.rawUserInfo != null || !uri.rawPath.isNullOrEmpty() ||
            uri.rawQuery != null || uri.rawFragment != null) return false
        val host = uri.host?.removeSurrounding("[", "]") ?: return false
        val port = uri.port
        if (port !in 1..65535) return false
        if (host.equals("localhost", ignoreCase = true)) return true

        // Never resolve a configured hostname while validating a local proxy.
        if (':' in host) {
            if (host.any { it !in '0'..'9' && it !in 'a'..'f' && it !in 'A'..'F' && it != ':' && it != '.' }) {
                return false
            }
            val address = runCatching { InetAddress.getByName(host) }.getOrNull() ?: return false
            return address.isLoopbackAddress
        }

        val octets = host.split('.')
        if (octets.size != 4 || octets.any { it.isEmpty() || it.any { ch -> !ch.isDigit() } }) return false
        val numbers = octets.map { it.toIntOrNull() ?: return false }
        if (numbers.any { it !in 0..255 }) return false
        return numbers[0] == 127
    }
}

/** Safe default: no TUN packet-forwarding backend/JNI library is bundled yet. */
object UnavailableVpnBackend : PgCoreVpnBackend {
    override fun start(
        tunFd: Int,
        config: VpnBackendConfig,
        protectSocket: (socketFd: Int) -> Boolean,
    ) = EngineResult.failure("vpn_backend_unavailable", "No Android TUN data-plane backend is installed")
    override fun stop() = EngineResult.success()
    override fun healthCheck() =
        EngineResult.failure("vpn_backend_unavailable", "No Android TUN data-plane backend is installed")
}

/** Host apps must explicitly register a tested backend before starting VPN. */
object PgCoreVpnBackendRegistry {
    @Volatile var factory: () -> PgCoreVpnBackend = { UnavailableVpnBackend }
    fun create(): PgCoreVpnBackend = factory()
}
