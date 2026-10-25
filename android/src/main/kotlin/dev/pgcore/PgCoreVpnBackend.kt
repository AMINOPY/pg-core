package dev.pgcore

/**
 * Data-plane contract for the Android VpnService.
 * The backend must forward packets between TUN and a real engine; process liveness is insufficient.
 * The fd is borrowed: the service owns and closes it after stop() completes.
 */
interface PgCoreVpnBackend {
    /** Return only after packet forwarding is ready. */
    fun start(tunFd: Int, config: VpnBackendConfig): EngineResult
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
) {
    init {
        require(sessionId.isNotBlank()) { "sessionId must not be blank" }
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
    }
}

data class VpnPathConfig(
    val id: String,
    /** Engine-owned local SOCKS/transport endpoint, not an arbitrary remote URL. */
    val endpoint: String,
    val weight: Int = 1,
    val enabled: Boolean = true,
) {
    init { require(weight in 1..100) { "weight must be between 1 and 100" } }
}

/** Safe default: no TUN packet-forwarding backend/JNI library is bundled yet. */
object UnavailableVpnBackend : PgCoreVpnBackend {
    override fun start(tunFd: Int, config: VpnBackendConfig) =
        EngineResult.failure("vpn_backend_unavailable", "No Android TUN data-plane backend is installed")
    override fun stop() = EngineResult.success()
    override fun healthCheck() =
        EngineResult.failure("vpn_backend_unavailable", "No Android TUN data-plane backend is installed")
}

/** Host apps must explicitly register a tested backend before starting VPN. */
object PgCoreVpnBackendRegistry {
    @Volatile var factory: () -> PgCoreVpnBackend = { UnavailableVpnBackend }
    fun create(): PgCoreVpnBackend = factory()
}
