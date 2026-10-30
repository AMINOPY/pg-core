package dev.pgcore

import org.junit.Assert.assertEquals
import org.junit.Test

class PgCoreVpnRuntimeStatusTest {
    @Test
    fun lifecycleStatusIsObservableAndFailureIsExplicit() {
        PgCoreVpnRuntimeStatus.update(PgCoreVpnRuntimeStatus.State.STOPPED)
        assertEquals(PgCoreVpnRuntimeStatus.State.STOPPED, PgCoreVpnRuntimeStatus.state)
        assertEquals(null, PgCoreVpnRuntimeStatus.failureCode)

        PgCoreVpnRuntimeStatus.update(PgCoreVpnRuntimeStatus.State.STARTING)
        assertEquals(PgCoreVpnRuntimeStatus.State.STARTING, PgCoreVpnRuntimeStatus.state)

        PgCoreVpnRuntimeStatus.update(PgCoreVpnRuntimeStatus.State.RUNNING)
        assertEquals(PgCoreVpnRuntimeStatus.State.RUNNING, PgCoreVpnRuntimeStatus.state)

        PgCoreVpnRuntimeStatus.update(PgCoreVpnRuntimeStatus.State.STOPPING)
        assertEquals(PgCoreVpnRuntimeStatus.State.STOPPING, PgCoreVpnRuntimeStatus.state)

        PgCoreVpnRuntimeStatus.update(PgCoreVpnRuntimeStatus.State.FAILED, "BACKEND_STOP_FAILED")
        assertEquals(PgCoreVpnRuntimeStatus.State.FAILED, PgCoreVpnRuntimeStatus.state)
        assertEquals("BACKEND_STOP_FAILED", PgCoreVpnRuntimeStatus.failureCode)

        PgCoreVpnRuntimeStatus.update(PgCoreVpnRuntimeStatus.State.STOPPED)
        assertEquals(PgCoreVpnRuntimeStatus.State.STOPPED, PgCoreVpnRuntimeStatus.state)
        assertEquals(null, PgCoreVpnRuntimeStatus.failureCode)
    }
}
