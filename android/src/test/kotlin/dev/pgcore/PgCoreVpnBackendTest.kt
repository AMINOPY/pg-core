package dev.pgcore

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class PgCoreVpnBackendTest {
    @Test
    fun unavailableBackendFailsClosedInsteadOfClaimingConnected() {
        val backend = UnavailableVpnBackend
        val start = backend.start(
            tunFd = 17,
            config = VpnBackendConfig(sessionId = "test-session"),
            protectSocket = { true },
        )

        assertFalse(start.ok)
        assertEquals("vpn_backend_unavailable", start.code)
        assertFalse(backend.healthCheck().ok)
        assertTrue(backend.stop().ok)
    }

    @Test
    fun backendConfigRejectsUnsafeOrAmbiguousBounds() {
        assertThrows<IllegalArgumentException> {
            VpnBackendConfig(sessionId = "session", mtu = 575)
        }
        assertThrows<IllegalArgumentException> {
            VpnBackendConfig(sessionId = "session", enableIpv4 = false, enableIpv6 = false)
        }
        assertThrows<IllegalArgumentException> {
            VpnBackendConfig(
                sessionId = "session",
                paths = listOf(
                    VpnPathConfig(id = "primary", endpoint = "127.0.0.1:1080"),
                    VpnPathConfig(id = "primary", endpoint = "127.0.0.1:1081"),
                ),
            )
        }
        assertThrows<IllegalArgumentException> {
            VpnPathConfig(id = "primary", endpoint = "127.0.0.1:1080", weight = 0)
        }
    }

    @Test
    fun defaultRegistryUsesUnavailableBackendUntilHostRegistersOne() {
        val previous = PgCoreVpnBackendRegistry.factory
        try {
            PgCoreVpnBackendRegistry.factory = { UnavailableVpnBackend }
            assertTrue(PgCoreVpnBackendRegistry.create() === UnavailableVpnBackend)
            assertFalse(PgCoreVpnBackendRegistry.create().healthCheck().ok)
        } finally {
            PgCoreVpnBackendRegistry.factory = previous
        }
    }

    private inline fun <reified T : Throwable> assertThrows(block: () -> Unit) {
        try {
            block()
        } catch (error: Throwable) {
            if (error is T) return
            throw AssertionError("Expected ${T::class.java.simpleName}, got ${error::class.java.simpleName}", error)
        }
        throw AssertionError("Expected ${T::class.java.simpleName} to be thrown")
    }
}
