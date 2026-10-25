package dev.pgcore

import org.junit.Assert.assertArrayEquals
import org.junit.Assert.assertEquals
import org.junit.Assert.assertThrows
import org.junit.Test

class FusionFrameTest {
    private val golden = byteArrayOf(
        0x50, 0x47, 0x46, 0x31, 0x01, 0x01, 0x00, 0x07,
        0x10, 0x20, 0x30, 0x40, 0x00, 0x00, 0x00, 0x02,
        0x68, 0x69
    )

    @Test
    fun matchesSharedGoldenVector() {
        val frame = FusionFrame(
            type = FusionFrame.DATA,
            flags = 7,
            sequence = 0x10203040L,
            payload = byteArrayOf(0x68, 0x69)
        )
        assertArrayEquals(golden, FusionFrame.encode(frame))

        val decoded = FusionFrame.decode(golden)
        assertEquals(FusionFrame.DATA, decoded.type)
        assertEquals(7, decoded.flags)
        assertEquals(0x10203040L, decoded.sequence)
        assertArrayEquals(byteArrayOf(0x68, 0x69), decoded.payload)
    }

    @Test
    fun rejectsMalformedAndOversizedFrames() {
        assertThrows(IllegalArgumentException::class.java) { FusionFrame.decode(byteArrayOf(1, 2)) }
        val badVersion = golden.copyOf().also { it[4] = 2 }
        assertThrows(IllegalArgumentException::class.java) { FusionFrame.decode(badVersion) }
        val trailing = golden + byteArrayOf(0)
        assertThrows(IllegalArgumentException::class.java) { FusionFrame.decode(trailing) }
        assertThrows(IllegalArgumentException::class.java) {
            FusionFrame.encode(FusionFrame(FusionFrame.DATA, sequence = 1, payload = ByteArray(FusionFrame.MAX_PAYLOAD_SIZE + 1)))
        }
    }
}
