package dev.pgcore

import java.io.File

/**
 * Explicit opt-in wiring for a consuming Android application.
 *
 * Call only after the host packages both the generated pgcore mobile AAR and
 * the checksum-pinned Hev AAR, and supplies a probe that exercises actual
 * TCP, UDP and DNS traffic through the VPN. No default registration occurs.
 */
object PgCoreHevMobileIntegration {
    @JvmStatic
    fun install(
        cacheDir: File,
        trafficProbe: VpnTrafficProbe,
        classLoader: ClassLoader? = Thread.currentThread().contextClassLoader,
    ) {
        require(cacheDir.isDirectory || cacheDir.mkdirs()) { "Unable to create VPN cache directory" }
        require(cacheDir.canWrite()) { "VPN cache directory is not writable" }
        val loader = classLoader ?: GoMobilePgCoreSocksEngine::class.java.classLoader!!
        PgCoreVpnBackendRegistry.factory = {
            HevSocks5TunnelBackend(
                cacheDir = cacheDir,
                engine = GoMobilePgCoreSocksEngine(
                    trafficProbe = trafficProbe,
                    classLoader = loader,
                ),
            )
        }
    }

    /** Restore the safe default backend if host initialization fails or is revoked. */
    @JvmStatic
    fun uninstall() {
        PgCoreVpnBackendRegistry.factory = { UnavailableVpnBackend }
    }
}
