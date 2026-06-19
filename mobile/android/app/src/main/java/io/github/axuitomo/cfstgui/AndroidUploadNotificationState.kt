package io.github.axuitomo.cfstgui

import android.content.Context
import java.util.Locale
import org.json.JSONObject

object AndroidUploadNotificationState {
    private const val PREFS_NAME = "cfst_android_upload_notification"
    private const val KEY_LAST_PAYLOAD = "last_payload"

    @JvmStatic
    fun recordFromEvent(context: Context, eventJSON: String?): Boolean {
        return try {
            val event = JSONObject(eventJSON ?: "{}")
            if (event.optString("event", "") != "upload.notification") {
                return false
            }
            val payload = event.optJSONObject("payload") ?: return false
            context.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE)
                .edit()
                .putString(KEY_LAST_PAYLOAD, payload.toString())
                .apply()
            true
        } catch (_: Exception) {
            false
        }
    }

    @JvmStatic
    fun notificationText(context: Context): String {
        val raw = context.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE).getString(KEY_LAST_PAYLOAD, "")
        if (raw.isNullOrBlank()) {
            return "常驻通知栏运行中，定时任务和后台测速更稳定。"
        }
        return try {
            formatPayload(JSONObject(raw))
        } catch (_: Exception) {
            "常驻通知栏运行中，定时任务和后台测速更稳定。"
        }
    }

    @JvmStatic
    fun clear(context: Context) {
        context.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE)
            .edit()
            .clear()
            .apply()
    }

    private fun formatPayload(payload: JSONObject): String {
        val source = sourceLabel(payload.optString("source", ""))
        val status = statusLabel(payload.optString("status", ""))
        val cloudflare = payload.optString("cloudflare_status", "")
        val github = payload.optString("github_status", "")
        val cloudflareCount = payload.optInt("cloudflare_upload_count", 0)
        val githubCount = payload.optInt("github_upload_count", 0)
        val parts = ArrayList<String>()
        if (cloudflare.isNotBlank()) {
            parts.add(String.format(Locale.ROOT, "CF %s %d条", statusLabel(cloudflare), cloudflareCount))
        }
        if (github.isNotBlank()) {
            parts.add(String.format(Locale.ROOT, "GitHub %s %d条", statusLabel(github), githubCount))
        }
        val providerText = if (parts.isEmpty()) "" else "；" + parts.joinToString("，")
        return "最近上传：$source $status$providerText"
    }

    private fun sourceLabel(source: String): String {
        return when (source.trim()) {
            "manual_push" -> "手动推送"
            "post_probe_push" -> "测速后自动上传"
            "scheduled_probe" -> "定时任务自动上传"
            "scheduled_pipeline" -> "定时工作流自动上传"
            else -> "上传任务"
        }
    }

    private fun statusLabel(status: String): String {
        return when (status.trim()) {
            "completed" -> "完成"
            "failed" -> "失败"
            "partial" -> "部分完成"
            "unsupported" -> "暂不支持"
            else -> "跳过"
        }
    }
}
