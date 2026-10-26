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
    fun pathEndpointsMustBeLoopbackAndHaveAValidPort() {
        assertTrue(VpnPathConfig(id = "local", endpoint = "127.0.0.1:1080").endpoint.isNotBlank())
        assertTrue(VpnPathConfig(id = "local-v6", endpoint = "[::1]:1080").endpoint.isNotBlank())
        assertTrue(VpnPathConfig(id = "localhost", endpoint = "localhost:1080").endpoint.isNotBlank())

        listOf(
            "vpn.example:1080",
            "192.0.2.10:1080",
            "127.0.0.1:0",
            "127.0.0.1:65536",
            "127.0.0.1:1080/path",
            "user@127.0.0.1:1080",
            "[2001:db8::1]:1080",
        ).forEach { endpoint ->
            assertThrows<IllegalArgumentException> {
                VpnPathConfig(id = "unsafe", endpoint = endpoint)
            }
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
