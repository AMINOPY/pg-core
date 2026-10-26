package dev.pgcore

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

class GhostUltraPlusTest {
    @Test fun classifiesHealthySamples() {
        assertEquals("healthy", GhostUltraPlus.analyze(GhostObservation(2,2,2)).status)
    }
    @Test fun supportsPersianOutput() {
        val result = GhostUltraPlus.analyze(GhostObservation(1,0,0), GhostLanguage.PERSIAN)
        assertTrue(result.title.contains("DNS"))
        assertTrue(result.details.contains("اثبات DPI نیست"))
    }
    @Test(expected = IllegalArgumentException::class)
    fun rejectsUnboundedSamples() { GhostUltraPlus.analyze(GhostObservation(21,0,0)) }
}
