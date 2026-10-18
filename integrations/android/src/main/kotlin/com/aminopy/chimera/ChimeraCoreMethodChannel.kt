package com.aminopy.chimera

import io.flutter.embedding.engine.plugins.FlutterPlugin
import io.flutter.plugin.common.MethodCall
import io.flutter.plugin.common.MethodChannel
import org.json.JSONObject

/**
 * Stable boundary implemented by a small adapter around the generated gomobile AAR.
 * Keep generated Java names out of the Flutter-facing API.
 */
interface ChimeraGoCore : AutoCloseable {
    fun evaluate(sampleJson: String): String
    fun confirmSwitch(targetId: String)
    fun cancelSwitch()
}

interface ChimeraGoLearningStore {
    fun observe(observationJson: String)
    fun snapshot(): String
    fun export(): String
    fun import(snapshotJson: String)
}

fun interface ChimeraGoCoreFactory {
    fun create(configJson: String): ChimeraGoCore

    /** Implement against gomobile NewLearningStore for adaptive evidence support. */
    fun createLearningStore(): ChimeraGoLearningStore =
        throw UnsupportedOperationException("Learning store adapter is not configured")
}

/**
 * Flutter MethodChannel host for the Go policy and learning APIs.
 * This plugin does not create a VpnService: flutter_v2ray remains the only VPN/TUN owner.
 */
class ChimeraCoreMethodChannel(
    private val factory: ChimeraGoCoreFactory
) : FlutterPlugin, MethodChannel.MethodCallHandler {
    private val lock = Any()
    private var channel: MethodChannel? = null
    private var core: ChimeraGoCore? = null
    private var learningStore: ChimeraGoLearningStore? = null

    override fun onAttachedToEngine(binding: FlutterPlugin.FlutterPluginBinding) {
        channel = MethodChannel(binding.binaryMessenger, CHANNEL_NAME).also {
            it.setMethodCallHandler(this)
        }
    }

    override fun onDetachedFromEngine(binding: FlutterPlugin.FlutterPluginBinding) {
        channel?.setMethodCallHandler(null)
        channel = null
        synchronized(lock) {
            core?.close()
            core = null
            learningStore = null
        }
    }

    override fun onMethodCall(call: MethodCall, result: MethodChannel.Result) {
        try {
            when (call.method) {
                "initialize" -> {
                    val config = requireJsonString(call.argument("config"))
                    val replacement = factory.create(config)
                    synchronized(lock) {
                        val previous = core
                        core = replacement
                        try {
                            previous?.close()
                        } catch (_: Exception) {
                            // Replacement is active; do not expose native details.
                        }
                    }
                    result.success(mapOf("ready" to true))
                }
                "evaluate" -> {
                    val sample = requireJsonString(call.argument("sample"))
                    val decision = synchronized(lock) {
                        requireNotNull(core) { "Chimera core is not initialized" }.evaluate(sample)
                    }
                    result.success(JSONObject(decision).toMap())
                }
                "confirmSwitch" -> {
                    val target = call.argument<String>("targetId")
                        ?.takeIf { it.isNotBlank() && it.length <= MAX_ID_LENGTH }
                        ?: throw IllegalArgumentException("Invalid targetId")
                    synchronized(lock) {
                        requireNotNull(core) { "Chimera core is not initialized" }.confirmSwitch(target)
                    }
                    result.success(mapOf("confirmed" to true))
                }
                "cancelSwitch" -> {
                    synchronized(lock) {
                        requireNotNull(core) { "Chimera core is not initialized" }.cancelSwitch()
                    }
                    result.success(mapOf("cancelled" to true))
                }
                "recordOutcome" -> {
                    val observation = requireJsonString(call.argument("observation"))
                    synchronized(lock) { getLearningStore().observe(observation) }
                    result.success(mapOf("recorded" to true))
                }
                "learningSnapshot" -> {
                    val snapshot = synchronized(lock) { getLearningStore().snapshot() }
                    result.success(snapshot)
                }
                "exportLearning" -> {
                    val snapshot = synchronized(lock) { getLearningStore().export() }
                    result.success(snapshot)
                }
                "importLearning" -> {
                    val snapshot = requireJsonString(call.argument("snapshot"))
                    synchronized(lock) { getLearningStore().import(snapshot) }
                    result.success(mapOf("imported" to true))
                }
                else -> result.notImplemented()
            }
        } catch (error: IllegalArgumentException) {
            result.error("INVALID_ARGUMENT", "Invalid argument", null)
        } catch (error: IllegalStateException) {
            result.error("CORE_NOT_READY", "Core is not initialized", null)
        } catch (error: Exception) {
            result.error("CHIMERA_CORE_ERROR", "Core operation failed", null)
        }
    }

    private fun getLearningStore(): ChimeraGoLearningStore {
        if (learningStore == null) learningStore = factory.createLearningStore()
        return requireNotNull(learningStore)
    }

    private fun requireJsonString(value: String?): String {
        val raw = value?.takeIf { it.isNotBlank() && it.length <= MAX_JSON_LENGTH }
            ?: throw IllegalArgumentException("Expected a bounded JSON string")
        JSONObject(raw)
        return raw
    }

    private fun JSONObject.toMap(): Map<String, Any?> {
        val output = linkedMapOf<String, Any?>()
        val keys = keys()
        while (keys.hasNext()) {
            val key = keys.next()
            val value = get(key)
            output[key] = if (value == JSONObject.NULL) null else value
        }
        return output
    }

    companion object {
        const val CHANNEL_NAME = "com.aminopy.chimera/core"
        private const val MAX_JSON_LENGTH = 256 * 1024
        private const val MAX_ID_LENGTH = 256
    }
}
