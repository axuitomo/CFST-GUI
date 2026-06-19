package io.github.axuitomo.cfstgui

import com.getcapacitor.JSObject
import java.io.OutputStreamWriter
import java.net.HttpURLConnection
import java.net.URL
import java.util.LinkedHashSet
import org.json.JSONArray
import org.json.JSONObject

object AndroidTelegramNotificationSender {
    private data class TelegramTestReceipt(
        val chatId: String,
        val purposes: List<String>,
    )

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
                telegram.optString("target_chat_id", ""),
                telegram.optString("targetChatId", ""),
                telegram.optString("channel_chat_id", ""),
                telegram.optString("channelChatId", ""),
                telegram.optString("group_chat_id", ""),
                telegram.optString("groupChatId", ""),
            )
            val personalChatId = firstNonEmpty(
                telegram.optString("personal_chat_id", ""),
                telegram.optString("personalChatId", ""),
                telegram.optString("private_chat_id", ""),
                telegram.optString("privateChatId", ""),
                telegram.optString("user_chat_id", ""),
                telegram.optString("userChatId", ""),
            )
            val legacyRecipientMode = firstNonEmpty(
                telegram.optString("recipient_mode", ""),
                telegram.optString("recipientMode", ""),
                telegram.optString("target_mode", ""),
                telegram.optString("targetMode", ""),
            )
            val uploadRecipientMode = firstNonEmpty(
                telegram.optString("upload_recipient_mode", ""),
                telegram.optString("uploadRecipientMode", ""),
                legacyRecipientMode,
            )
            val topNRecipientMode = firstNonEmpty(
                telegram.optString("top_n_recipient_mode", ""),
                telegram.optString("topNRecipientMode", ""),
                telegram.optString("top_recipient_mode", ""),
                telegram.optString("topRecipientMode", ""),
                legacyRecipientMode,
            )
            val includeTopN = telegram.optBoolean(
                "include_top_n",
                telegram.optBoolean("includeTopN", telegram.optBoolean("top_n_enabled", telegram.optBoolean("topNEnabled", false))),
            )
            val uploadChatIds = telegramChatIds(uploadRecipientMode, chatId, personalChatId)
            val topNChatIds = if (includeTopN) {
                telegramChatIds(topNRecipientMode, chatId, personalChatId)
            } else {
                emptyList()
            }
            val chatIds = compactChatIds(uploadChatIds + topNChatIds)
            if (token.isBlank() || chatIds.isEmpty() || isMaskedSecret(token)) {
                return AndroidPluginCommands.command("TELEGRAM_NOTIFICATION_TEST_FAILED", JSObject(), "Telegram 通知配置不完整", false)
            }
            if (uploadChatIds.isEmpty()) {
                return AndroidPluginCommands.command("TELEGRAM_NOTIFICATION_TEST_FAILED", JSObject(), "Telegram 通知目标配置不完整", false)
            }
            if (includeTopN && topNChatIds.isEmpty()) {
                return AndroidPluginCommands.command("TELEGRAM_NOTIFICATION_TEST_FAILED", JSObject(), "Telegram 通知目标配置不完整", false)
            }
            val receipts = telegramTestReceipts(uploadChatIds, topNChatIds)
            receipts.forEach { receipt ->
                postTelegramMessage(token, receipt.chatId, telegramTestReceiptText(receipt))
            }
            val data = JSObject()
            data.put("chat_id", receipts.first().chatId)
            data.put("chat_ids", JSONArray(receipts.map { it.chatId }))
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
        return values.firstOrNull { it.isNotBlank() }?.trim().orEmpty()
    }

    private fun telegramChatIds(mode: String, chatId: String, personalChatId: String): List<String> {
        return when (mode.trim().lowercase()) {
            "personal", "private", "direct", "user", "me" -> compactChatIds(personalChatId)
            "both", "all", "chat_personal", "chat_and_personal", "personal_and_chat" -> compactChatIds(chatId, personalChatId)
            else -> compactChatIds(chatId)
        }
    }

    private fun compactChatIds(vararg values: String): List<String> {
        return compactChatIds(values.asIterable())
    }

    private fun compactChatIds(values: Iterable<String>): List<String> {
        return values
            .map { it.trim() }
            .filter { it.isNotBlank() }
            .toCollection(LinkedHashSet())
            .toList()
    }

    private fun telegramTestReceipts(uploadChatIds: List<String>, topNChatIds: List<String>): List<TelegramTestReceipt> {
        val purposeMap = LinkedHashMap<String, MutableList<String>>()
        uploadChatIds.forEach { chatId ->
            purposeMap.getOrPut(chatId) { mutableListOf() }.add("upload")
        }
        topNChatIds.forEach { chatId ->
            purposeMap.getOrPut(chatId) { mutableListOf() }.add("topn")
        }
        return purposeMap.map { (chatId, purposes) ->
            TelegramTestReceipt(chatId, purposes.toList())
        }
    }

    private fun telegramTestReceiptText(receipt: TelegramTestReceipt): String {
        return buildString {
            append("CFST Telegram 通知测试\n")
            append("状态：Telegram 通知渠道可用。\n")
            append("用途：")
            append(receipt.purposes.joinToString("、") { telegramTestPurposeLabel(it) })
        }
    }

    private fun telegramTestPurposeLabel(purpose: String): String {
        return when (purpose) {
            "topn" -> "Top N 列表"
            else -> "上传结论"
        }
    }

    private fun isMaskedSecret(value: String): Boolean {
        val trimmed = value.trim()
        return trimmed.contains("***") || trimmed.contains("...") || trimmed.all { it == '*' }
    }
}
