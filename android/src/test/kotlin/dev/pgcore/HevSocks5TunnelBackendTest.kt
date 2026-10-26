package dev.pgcore

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test
import java.io.File

class HevSocks5TunnelBackendTest {
    private class FakeEngine(
        var endpoint: String = "127.0.0.1:1080",
        var startResult: EngineResult = EngineResult.success(),
        var healthResult: EngineResult = EngineResult.success(),
    ) : PgCoreSocksEngine {
        var starts = 0
        var stops = 0
        override fun start(profileJson: String, protectSocket: (Int) -> Boolean): EngineResult {
            starts++
            return startResult
        }
        override fun socksEndpoint() = endpoint
        override fun endToEndHealthCheck() = healthResult
        override fun stop(): EngineResult { stops++; return EngineResult.success() }
    }

    private class FakeHev : HevTunnelApi {
        var startResult = true
        var running = false
        var starts = 0
        var stops = 0
        var configPath = ""
        var tunFd = -1
        override fun start(configPath: String, tunFd: Int): Boolean {
            starts++
            this.configPath = configPath
            this.tunFd = tunFd
            running = startResult
            return startResult
        }
        override fun stop(): Boolean { stops++; running = false; return true }
        override fun isRunning() = running
    }

    @Test
    fun startsEngineBeforeTunAndStopsInReverseOrder() {
        val cache = createTempDir(prefix = "pgcore-hev-test")
        try {
            val engine = FakeEngine()
            val hev = FakeHev()
            val backend = HevSocks5TunnelBackend(cache, engine, hev)
            val result = backend.start(41, VpnBackendConfig("s", engineProfileJson = """{"server":"test"}""")) { true }
            assertTrue(result.ok)
            assertEquals(1, engine.starts)
            assertEquals(41, hev.tunFd)
            assertTrue(File(hev.configPath).readText().contains("port: 1080"))
            assertTrue(backend.healthCheck().ok)
            assertTrue(backend.stop().ok)
            assertEquals(1, hev.stops)
            assertEquals(1, engine.stops)
        } finally {
            cache.deleteRecursively()
        }
    }

    @Test
    fun refusesMissingProfileAndNeverStartsNativeTunnel() {
        val cache = createTempDir(prefix = "pgcore-hev-test")
        try {
            val hev = FakeHev()
            val backend = HevSocks5TunnelBackend(cache, FakeEngine(), hev)
            val result = backend.start(41, VpnBackendConfig("s")) { true }
            assertFalse(result.ok)
            assertEquals("missing_engine_profile", result.code)
            assertEquals(0, hev.starts)
        } finally {
            cache.deleteRecursively()
        }
    }

    @Test
    fun rejectsRemoteSocksEndpointAndCleansEngine() {
        val cache = createTempDir(prefix = "pgcore-hev-test")
        try {
            val engine = FakeEngine(endpoint = "example.com:1080")
            val hev = FakeHev()
            val backend = HevSocks5TunnelBackend(cache, engine, hev)
            val result = backend.start(41, VpnBackendConfig("s", engineProfileJson = """{"server":"test"}""")) { true }
            assertFalse(result.ok)
            assertEquals("hev_backend_start_failed", result.code)
            assertEquals(1, engine.stops)
            assertEquals(0, hev.starts)
        } finally {
            cache.deleteRecursively()
        }
    }

    @Test
    fun doesNotStopEngineWhileHevStillRunningAfterFailedStop() {
        val cache = createTempDir(prefix = "pgcore-hev-test")
        try {
            val engine = FakeEngine()
            val hev = object : HevTunnelApi {
                override fun start(configPath: String, tunFd: Int) = true
                override fun stop() = false
                override fun isRunning() = true
            }
            val backend = HevSocks5TunnelBackend(cache, engine, hev)
            assertTrue(backend.start(41, VpnBackendConfig("s", engineProfileJson = """{"server":"test"}""")) { true }.ok)
            assertFalse(backend.stop().ok)
            assertEquals(0, engine.stops)
        } finally {
            cache.deleteRecursively()
        }
    }

    @Test
    fun engineStartFailureNeverStartsHev() {
        val cache = createTempDir(prefix = "pgcore-hev-test")
        try {
            val engine = FakeEngine(startResult = EngineResult.failure("engine_start_failed", "test"))
            val hev = FakeHev()
            val backend = HevSocks5TunnelBackend(cache, engine, hev)
            val result = backend.start(41, VpnBackendConfig("s", engineProfileJson = """{"server":"test"}""")) { true }
            assertFalse(result.ok)
            assertEquals(0, hev.starts)
            assertEquals(0, engine.stops)
        } finally {
            cache.deleteRecursively()
        }
    }

    @Test
    fun failedEndToEndHealthCheckRollsBackNativeAndEngine() {
        val cache = createTempDir(prefix = "pgcore-hev-test")
        try {
            val engine = FakeEngine(healthResult = EngineResult.failure("health_failed", "no traffic"))
            val hev = FakeHev()
            val backend = HevSocks5TunnelBackend(cache, engine, hev)
            val result = backend.start(41, VpnBackendConfig("s", engineProfileJson = """{"server":"test"}""")) { true }
            assertFalse(result.ok)
            assertEquals(1, hev.starts)
            assertEquals(1, hev.stops)
            assertEquals(1, engine.stops)
            assertFalse(File(cache, "pgcore-hev-tun.yml").exists())
        } finally {
            cache.deleteRecursively()
        }
    }

    @Test
    fun failedNativeStartIsCleanedBeforeEngineShutdown() {
        val cache = createTempDir(prefix = "pgcore-hev-test")
        try {
            val engine = FakeEngine()
            val hev = FakeHev().apply { startResult = false }
            val backend = HevSocks5TunnelBackend(cache, engine, hev)
            val result = backend.start(41, VpnBackendConfig("s", engineProfileJson = """{"server":"test"}""")) { true }
            assertFalse(result.ok)
            assertEquals(1, hev.starts)
            assertEquals(1, hev.stops)
            assertEquals(1, engine.stops)
        } finally {
            cache.deleteRecursively()
        }
    }

}
