package com.aminopy.chimera

import io.flutter.embedding.engine.plugins.FlutterPlugin
import io.flutter.plugin.common.MethodCall
import io.flutter.plugin.common.MethodChannel
import org.json.JSONObject

/**
 * Stable boundary implemented by a tiny adapter around the generated gomobile AAR.
 * Keep this interface independent of generated Java names so gomobile can be pinned
 * and upgraded without leaking generated types into the Flutter API.
 */
interface ChimeraGoCore : AutoCloseable {
    /** Returns the Go policy decision JSON for a sanitized sample. */
    fun evaluate(sampleJson: String): String
    fun confirmSwitch(targetId: String)
    fun cancelSwitch()
}

fun interface ChimeraGoCoreFactory {
    fun create(configJson: String): ChimeraGoCore
}

/**
 * Flutter MethodChannel host for the Go policy API.
 *
 * The embedding application must supply a ChimeraGoCoreFactory backed by the
 * generated gomobile AAR. This plugin deliberately does not create a VpnService:
 * flutter_v2ray remains the only VPN/TUN owner and performs the actual profile switch.
 */
class ChimeraCoreMethodChannel(
    private val factory: ChimeraGoCoreFactory
) : FlutterPlugin, MethodChannel.MethodCallHandler {
    private val lock = Any()
    private var channel: MethodChannel? = null
    private var core: ChimeraGoCore? = null

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
        }
    }

    override fun onMethodCall(call: MethodCall, result: MethodChannel.Result) {
        try {
            when (call.method) {
                "initialize" -> {
                    val config = requireJsonString(call.argument("config"))
                    // Construct first: if initialization fails, keep the old core usable.
                    val replacement = factory.create(config)
                    synchronized(lock) {
                        val previous = core
                        core = replacement
                        try {
                            previous?.close()
                        } catch (_: Exception) {
                            // The replacement is already active; do not expose native details.
                        }
                    }
                    result.success(mapOf("ready" to true))
                }
                "evaluate" -> {
                    val sample = requireJsonString(call.argument("sample"))
                    val decision = synchronized(lock) {
                        requireNotNull(core) { "Chimera core is not initialized" }
                            .evaluate(sample)
                    }
                    // Return a structured map rather than an opaque JSON string to Dart.
                    result.success(JSONObject(decision).toMap())
                }
                "confirmSwitch" -> {
                    val target = call.argument<String>("targetId")
                        ?.takeIf { it.isNotBlank() && it.length <= MAX_ID_LENGTH }
                        ?: throw IllegalArgumentException("Invalid targetId")
                    synchronized(lock) {
                        requireNotNull(core) { "Chimera core is not initialized" }
                            .confirmSwitch(target)
                    }
                    result.success(mapOf("confirmed" to true))
                }
                "cancelSwitch" -> {
                    synchronized(lock) {
                        requireNotNull(core) { "Chimera core is not initialized" }.cancelSwitch()
                    }
                    result.success(mapOf("cancelled" to true))
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

    private fun requireJsonString(value: String?): String {
        val raw = value?.takeIf { it.isNotBlank() && it.length <= MAX_JSON_LENGTH }
            ?: throw IllegalArgumentException("Expected a bounded JSON string")
        // Parse before passing to Go; Go remains authoritative for schema validation.
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
