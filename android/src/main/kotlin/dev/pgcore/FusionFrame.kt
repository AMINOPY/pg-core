package dev.pgcore

import java.nio.ByteBuffer
import java.nio.ByteOrder

/**
 * Bounded frame codec matching internal/fusionwire and native/fusion_frame.
 * Framing only: not a socket, authenticated tunnel, or packet-bonding data plane.
 */
data class FusionFrame(
    val type: Int,
    val flags: Int = 0,
    val sequence: Long,
    val payload: ByteArray = byteArrayOf()
) {
    companion object {
        const val VERSION = 1
        const val HEADER_SIZE = 16
        const val MAX_PAYLOAD_SIZE = 64 * 1024
        const val DATA = 1
        const val PING = 2
        const val PONG = 3
        const val CLOSE = 4
        private val MAGIC = byteArrayOf(80, 71, 70, 49)

        fun encode(frame: FusionFrame): ByteArray {
            require(frame.type in DATA..CLOSE) { "unsupported fusion frame type" }
            require(frame.flags in 0..0xffff) { "flags out of range" }
            require(frame.sequence in 0L..0xffff_ffffL) { "sequence out of range" }
            require(frame.payload.size <= MAX_PAYLOAD_SIZE) { "fusion payload exceeds limit" }

            val buffer = ByteBuffer.allocate(HEADER_SIZE + frame.payload.size).order(ByteOrder.BIG_ENDIAN)
            buffer.put(MAGIC)
            buffer.put(VERSION.toByte())
            buffer.put(frame.type.toByte())
            buffer.putShort(frame.flags.toShort())
            buffer.putInt(frame.sequence.toInt())
            buffer.putInt(frame.payload.size)
            buffer.put(frame.payload)
            return buffer.array()
        }

        fun decode(wire: ByteArray): FusionFrame {
            require(wire.size >= HEADER_SIZE) { "truncated fusion header" }
            val buffer = ByteBuffer.wrap(wire).order(ByteOrder.BIG_ENDIAN)
            val magic = ByteArray(4)
            buffer.get(magic)
            require(magic.contentEquals(MAGIC)) { "invalid fusion magic" }
            require((buffer.get().toInt() and 0xff) == VERSION) { "unsupported fusion version" }

            val type = buffer.get().toInt() and 0xff
            require(type in DATA..CLOSE) { "unsupported fusion frame type" }
            val flags = buffer.short.toInt() and 0xffff
            val sequence = buffer.int.toLong() and 0xffff_ffffL
            val payloadSize = buffer.int
            require(payloadSize in 0..MAX_PAYLOAD_SIZE) { "fusion payload exceeds limit" }
            require(wire.size == HEADER_SIZE + payloadSize) { "frame length mismatch" }

            return FusionFrame(type, flags, sequence, ByteArray(payloadSize).also { buffer.get(it) })
        }
    }
}
