package io.github.axuitomo.cfstgui

import com.getcapacitor.JSObject
import java.io.OutputStreamWriter
import java.net.HttpURLConnection
import java.net.URL
import org.json.JSONObject

object AndroidTelegramNotificationSender {
    @JvmStatic
    fun send(payloadJSON: String?): JSObject {
        return try {
            val payload = JSONObject(payloadJSON ?: "{}")
            val config = payload.optJSONObject("config")
                ?: payload.optJSONObject("config_snapshot")
                ?: payload.optJSONObject("configSnapshot")
                ?: payload
            val telegram = config.optJSONObject("notifications")?.optJSONObject("telegram")
                ?: config.optJSONObject("telegram")
                ?: JSONObject()
            val token = firstNonEmpty(
                telegram.optString("bot_token", ""),
                telegram.optString("botToken", ""),
                telegram.optString("token", ""),
            )
            val chatId = firstNonEmpty(
                telegram.optString("chat_id", ""),
                telegram.optString("chatId", ""),
                telegram.optString("chat", ""),
            )
            if (token.isBlank() || chatId.isBlank() || token.contains("***") || token.contains("...")) {
                return AndroidPluginCommands.command("TELEGRAM_NOTIFICATION_TEST_FAILED", JSObject(), "Telegram 通知配置不完整", false)
            }
            postTelegramMessage(token, chatId, "CFST 上传通知测试\n状态：Telegram 通知渠道可用。")
            val data = JSObject()
            data.put("chat_id", chatId)
            AndroidPluginCommands.command("TELEGRAM_NOTIFICATION_TEST_OK", data, "Telegram 通知测试已发送。", true)
        } catch (error: Exception) {
            AndroidPluginCommands.command("TELEGRAM_NOTIFICATION_TEST_FAILED", JSObject(), error.message ?: "Telegram 通知测试失败", false)
        }
    }

    private fun postTelegramMessage(token: String, chatId: String, text: String) {
        val connection = (URL("https://api.telegram.org/bot$token/sendMessage").openConnection() as HttpURLConnection).apply {
            connectTimeout = 15000
            readTimeout = 15000
            requestMethod = "POST"
            doOutput = true
            setRequestProperty("Content-Type", "application/json")
        }
        val body = JSONObject()
        body.put("chat_id", chatId)
        body.put("disable_web_page_preview", true)
        body.put("text", text)
        OutputStreamWriter(connection.outputStream, Charsets.UTF_8).use { writer ->
            writer.write(body.toString())
        }
        val statusCode = connection.responseCode
        if (statusCode !in 200..299) {
            val message = connection.errorStream?.bufferedReader()?.use { it.readText() }?.trim().orEmpty()
            throw IllegalStateException(if (message.isBlank()) "Telegram HTTP $statusCode" else "Telegram HTTP $statusCode：$message")
        }
    }

    private fun firstNonEmpty(vararg values: String): String {
        for (value in values) {
            if (value.isNotBlank()) {
                return value.trim()
            }
        }
        return ""
    }
}
