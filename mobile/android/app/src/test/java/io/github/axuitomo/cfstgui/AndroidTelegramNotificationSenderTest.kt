package io.github.axuitomo.cfstgui

import org.json.JSONObject
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [35])
class AndroidTelegramNotificationSenderTest {
    @Test
    fun sendFailsWhenUploadRecipientTargetsAreMissing() {
        val result = JSONObject(
            AndroidTelegramNotificationSender.send(
                """
                {
                  "config": {
                    "notifications": {
                      "telegram": {
                        "bot_token": "token:secret",
                        "chat_id": "group-chat",
                        "include_top_n": true,
                        "top_n_recipient_mode": "chat",
                        "upload_recipient_mode": "personal"
                      }
                    }
                  }
                }
                """.trimIndent(),
            ).toString(),
        )

        assertFalse(result.getBoolean("ok"))
        assertEquals("Telegram 通知目标配置不完整", result.getString("message"))
    }

    @Test
    fun telegramChatIdsSupportsBothRecipientMode() {
        val method = AndroidTelegramNotificationSender::class.java.getDeclaredMethod(
            "telegramChatIds",
            String::class.java,
            String::class.java,
            String::class.java,
        )
        method.isAccessible = true

        @Suppress("UNCHECKED_CAST")
        val chatIds = method.invoke(AndroidTelegramNotificationSender, "both", "group-chat", "personal-chat") as List<String>

        assertEquals(listOf("group-chat", "personal-chat"), chatIds)
    }

    @Test
    fun compactChatIdsDedupesAndTrims() {
        val method = AndroidTelegramNotificationSender::class.java.getDeclaredMethod(
            "compactChatIds",
            Iterable::class.java,
        )
        method.isAccessible = true

        @Suppress("UNCHECKED_CAST")
        val chatIds = method.invoke(
            AndroidTelegramNotificationSender,
            listOf(" group-chat ", "personal-chat", "group-chat", ""),
        ) as List<String>

        assertEquals(listOf("group-chat", "personal-chat"), chatIds)
    }

    @Test
    fun telegramTestReceiptsMergePurposesForSameChat() {
        val method = AndroidTelegramNotificationSender::class.java.getDeclaredMethod(
            "telegramTestReceipts",
            List::class.java,
            List::class.java,
        )
        method.isAccessible = true

        @Suppress("UNCHECKED_CAST")
        val receipts = method.invoke(
            AndroidTelegramNotificationSender,
            listOf("group-chat"),
            listOf("group-chat"),
        ) as List<Any>

        assertEquals(1, receipts.size)
        val receipt = receipts.first()
        val chatId = receipt.javaClass.getDeclaredMethod("getChatId").invoke(receipt) as String
        @Suppress("UNCHECKED_CAST")
        val purposes = receipt.javaClass.getDeclaredMethod("getPurposes").invoke(receipt) as List<String>
        val textMethod = AndroidTelegramNotificationSender::class.java.getDeclaredMethod(
            "telegramTestReceiptText",
            receipt.javaClass,
        )
        textMethod.isAccessible = true
        val text = textMethod.invoke(AndroidTelegramNotificationSender, receipt) as String

        assertEquals("group-chat", chatId)
        assertEquals(listOf("upload", "topn"), purposes)
        assertEquals("CFST Telegram 通知测试\n状态：Telegram 通知渠道可用。\n用途：上传结论、Top N 列表", text)
    }

    @Test
    fun telegramTestReceiptsKeepOneReceiptPerChat() {
        val method = AndroidTelegramNotificationSender::class.java.getDeclaredMethod(
            "telegramTestReceipts",
            List::class.java,
            List::class.java,
        )
        method.isAccessible = true

        @Suppress("UNCHECKED_CAST")
        val receipts = method.invoke(
            AndroidTelegramNotificationSender,
            listOf("personal-chat"),
            listOf("group-chat"),
        ) as List<Any>

        assertEquals(2, receipts.size)
        val firstReceipt = receipts[0]
        val secondReceipt = receipts[1]
        val firstChatId = firstReceipt.javaClass.getDeclaredMethod("getChatId").invoke(firstReceipt) as String
        val secondChatId = secondReceipt.javaClass.getDeclaredMethod("getChatId").invoke(secondReceipt) as String
        @Suppress("UNCHECKED_CAST")
        val firstPurposes = firstReceipt.javaClass.getDeclaredMethod("getPurposes").invoke(firstReceipt) as List<String>
        @Suppress("UNCHECKED_CAST")
        val secondPurposes = secondReceipt.javaClass.getDeclaredMethod("getPurposes").invoke(secondReceipt) as List<String>

        assertEquals("personal-chat", firstChatId)
        assertEquals(listOf("upload"), firstPurposes)
        assertEquals("group-chat", secondChatId)
        assertEquals(listOf("topn"), secondPurposes)
    }

    @Test
    fun maskedSecretIsDetected() {
        val method = AndroidTelegramNotificationSender::class.java.getDeclaredMethod(
            "isMaskedSecret",
            String::class.java,
        )
        method.isAccessible = true

        assertTrue(method.invoke(AndroidTelegramNotificationSender, "abc...xyz") as Boolean)
        assertTrue(method.invoke(AndroidTelegramNotificationSender, "******") as Boolean)
        assertFalse(method.invoke(AndroidTelegramNotificationSender, "token:secret") as Boolean)
    }
}
