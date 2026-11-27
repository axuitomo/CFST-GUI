package io.github.axuitomo.cfstgui

import android.content.Context
import android.util.Log
import com.getcapacitor.JSObject
import org.json.JSONArray
import org.json.JSONObject

object AndroidPluginCommands {
    private const val TAG = "CfstPlugin"

    // JSON 文本里 0x20 以下都是控制字符：字符串外只允许空白，字符串内必须转义。
    private const val JSON_PRINTABLE_MIN = 0x20
    private const val HEX_RADIX = 16
    private const val UNICODE_ESCAPE_WIDTH = 4

    @JvmStatic
    fun usesDedicatedExecutor(command: String): Boolean =
        command == "colo.update" || command == "colo.process"

    @JvmStatic
    fun command(code: String, data: JSObject?, message: String, ok: Boolean): JSObject {
        val result = JSObject()
        result.put("code", code)
        result.put("data", data ?: JSONObject.NULL)
        result.put("message", message)
        result.put("ok", ok)
        result.put("schema_version", "cfst-gui-command-v2")
        result.put("task_id", JSONObject.NULL)
        result.put("warnings", JSONArray())
        return result
    }

    @JvmStatic
    fun commandJSON(code: String, data: JSObject?, message: String, ok: Boolean): String {
        return command(code, data, message, ok).toString()
    }

    // normalizeJSON 把可能带 BOM 或裸控制字符的 JSON 文本修正为严格可解析形态。
    // 字符串内的控制字符必须转义成 \uXXXX：直接丢弃会把错误信息里的换行/制表符静默吃掉，
    // 而保留裸控制字符则会让严格解析器整条拒绝（Go encoding/json 报 invalid character in string）。
    @JvmStatic
    fun normalizeJSON(raw: String?): String {
        val value = raw.orEmpty().removePrefix("\uFEFF")
        val normalized = StringBuilder(value.length)
        var inString = false
        var escaped = false
        for (character in value) {
            if (escaped) {
                normalized.append(character)
                escaped = false
                continue
            }
            if (inString && character == '\\') {
                normalized.append(character)
                escaped = true
                continue
            }
            if (character == '"') {
                inString = !inString
                normalized.append(character)
                continue
            }
            if (character.code >= JSON_PRINTABLE_MIN) {
                normalized.append(character)
                continue
            }
            if (inString) {
                normalized.append(unicodeEscape(character.code))
                continue
            }
            if (isJSONWhitespace(character)) {
                normalized.append(character)
            }
        }
        return normalized.toString()
    }

    private fun unicodeEscape(code: Int): String {
        return "\\u" + code.toString(HEX_RADIX).uppercase().padStart(UNICODE_ESCAPE_WIDTH, '0')
    }

    private fun isJSONWhitespace(character: Char): Boolean {
        return character == '\t' || character == '\n' || character == '\r'
    }

    @JvmStatic
    fun finalizeServiceResponse(context: Context, responseJSON: String): String {
        val command = JSONObject(normalizeJSON(responseJSON))
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
