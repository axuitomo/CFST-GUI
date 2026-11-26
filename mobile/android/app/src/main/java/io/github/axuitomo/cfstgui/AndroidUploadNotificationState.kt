package io.github.axuitomo.cfstgui

import android.content.Context
import androidx.core.content.edit
import org.json.JSONObject
import java.util.Locale

object AndroidUploadNotificationState {
    private const val PREFS_NAME = "cfst_android_upload_notification"
    private const val KEY_LAST_PAYLOAD = "last_payload"
    private const val MAX_PROVIDER_LABEL_LENGTH = 24
    private const val PROVIDER_LABEL_ELLIPSIS = "…"

    @JvmStatic
    fun recordFromEvent(context: Context, eventJSON: String?): Boolean {
        return try {
            val event = JSONObject(eventJSON ?: "{}")
            if (event.optString("event", "") != "upload.notification") {
                return false
            }
            val payload = event.optJSONObject("payload") ?: return false
            context.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE).edit {
                putString(KEY_LAST_PAYLOAD, payload.toString())
            }
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
        context.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE).edit {
            clear()
        }
    }

    private fun formatPayload(payload: JSONObject): String {
        val source = sourceLabel(payload.optString("source", ""))
        val status = statusLabel(payload.optString("status", ""))
        val parts = providerParts(payload)
        val providerText = if (parts.isEmpty()) "" else "；" + parts.joinToString("，")
        return "最近上传：$source $status$providerText"
    }

    private fun providerParts(payload: JSONObject): List<String> {
        val parts = ArrayList<String>()
        val providers = payload.optJSONArray("providers")
        if (providers != null) {
            for (index in 0 until providers.length()) {
                val item = providers.optJSONObject(index)
                val providerStatus = item?.optString("status", "").orEmpty()
                if (item == null || providerStatus.isBlank()) {
                    continue
                }
                parts.add(
                    String.format(
                        Locale.ROOT,
                        "%s %s %d条",
                        providerLabel(item),
                        statusLabel(providerStatus),
                        item.optInt("upload_count", 0),
                    ),
                )
            }
        }
        if (parts.isNotEmpty()) {
            return parts
        }
        // 旧负载只有 Cloudflare/GitHub 字段，继续兼容已保存的通知。
        val cloudflare = payload.optString("cloudflare_status", "")
        val github = payload.optString("github_status", "")
        if (cloudflare.isNotBlank()) {
            parts.add(
                String.format(
                    Locale.ROOT,
                    "CF %s %d条",
                    statusLabel(cloudflare),
                    payload.optInt("cloudflare_upload_count", 0),
                ),
            )
        }
        if (github.isNotBlank()) {
            parts.add(
                String.format(
                    Locale.ROOT,
                    "GitHub %s %d条",
                    statusLabel(github),
                    payload.optInt("github_upload_count", 0),
                ),
            )
        }
        return parts
    }

    private fun providerLabel(item: JSONObject): String {
        val label = item.optString("label", "").trim()
        if (label.isNotBlank()) {
            return if (label.length > MAX_PROVIDER_LABEL_LENGTH) {
                label.take(MAX_PROVIDER_LABEL_LENGTH - 1) + PROVIDER_LABEL_ELLIPSIS
            } else {
                label
            }
        }
        return when (item.optString("provider", "").trim()) {
            "dnshe" -> "DNSHE"
            "aliyun" -> "阿里云"
            "dnspod" -> "DNSPod"
            "github" -> "GitHub"
            else -> "CF"
        }
    }

    private fun sourceLabel(source: String): String {
        return when (source.trim()) {
            "manual_push" -> "手动推送"
            "post_probe_push" -> "测速后自动上传"
            "scheduled_probe" -> "定时任务自动上传"
            else -> "上传任务"
        }
    }

    private fun statusLabel(status: String): String {
        return when (status.trim()) {
            "completed" -> "完成"
            "failed" -> "失败"
            "partial" -> "部分完成"
            "aborted" -> "熔断/终止"
            "unsupported" -> "暂不支持"
            else -> "跳过"
        }
    }
}
