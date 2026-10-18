package com.aminopy.chimera

/**
 * Host-provided transport for the Chimera JSON policy bridge.
 *
 * This facade does not create a VPN or assume generated gomobile class names.
 */
fun interface ChimeraCoreBridge {
    @Throws(Exception::class)
    fun invoke(method: String, payload: Map<String, Any?>): String
}

class ChimeraCoreClient(private val bridge: ChimeraCoreBridge) {
    @Throws(Exception::class)
    fun evaluate(sampleJson: String): String {
        require(sampleJson.isNotBlank()) { "sampleJson must not be blank" }
        return bridge.invoke("evaluate", mapOf("sample" to sampleJson))
    }

    @Throws(Exception::class)
    fun confirmSwitch(targetId: String) {
        require(targetId.isNotBlank()) { "targetId must not be blank" }
        bridge.invoke("confirmSwitch", mapOf("targetId" to targetId))
    }

    @Throws(Exception::class)
    fun cancelSwitch() {
        bridge.invoke("cancelSwitch", emptyMap())
    }
}
