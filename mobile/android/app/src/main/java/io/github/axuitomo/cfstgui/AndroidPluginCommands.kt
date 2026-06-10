package io.github.axuitomo.cfstgui

import android.content.Context
import android.util.Log
import com.getcapacitor.JSObject
import org.json.JSONArray
import org.json.JSONObject

object AndroidPluginCommands {
    private const val TAG = "CfstPlugin"

    @JvmStatic
    fun command(code: String, data: JSObject, message: String, ok: Boolean): JSObject {
        val result = JSObject()
        result.put("code", code)
        result.put("data", data)
        result.put("message", message)
        result.put("ok", ok)
        result.put("schema_version", "cfst-gui-mobile-v1")
        result.put("task_id", JSONObject.NULL)
        result.put("warnings", JSONArray())
        return result
    }

    @JvmStatic
    fun commandJSON(code: String, data: JSObject, message: String, ok: Boolean): String {
        return command(code, data, message, ok).toString()
    }

    @JvmStatic
    fun finalizeLoadConfigResponse(context: Context, responseJSON: String): String {
        return finalizeServiceResponse(context, responseJSON)
    }

    @JvmStatic
    fun finalizeServiceResponse(context: Context, responseJSON: String): String {
        val command = JSONObject(responseJSON)
        try {
            attachStorageState(context, command)
        } catch (error: Exception) {
            Log.e(TAG, "Failed to attach Android storage state to plugin response.", error)
            appendWarning(command, command.optJSONObject("data"), "Android 储存状态附加失败：" + error.message)
        }
        return command.toString()
    }

    @JvmStatic
    fun appendWarning(command: JSONObject, data: JSONObject?, warning: String) {
        var topWarnings = command.optJSONArray("warnings")
        if (topWarnings == null) {
            topWarnings = JSONArray()
            command.put("warnings", topWarnings)
        }
        topWarnings.put(warning)
        if (data != null) {
            var dataWarnings = data.optJSONArray("warnings")
            if (dataWarnings == null) {
                dataWarnings = JSONArray()
                data.put("warnings", dataWarnings)
            }
            dataWarnings.put(warning)
        }
    }

    private fun attachStorageState(context: Context, command: JSONObject) {
        val data = command.optJSONObject("data") ?: return
        data.put("storage", AndroidStorageState.currentStorageStatus(context))
    }
}
