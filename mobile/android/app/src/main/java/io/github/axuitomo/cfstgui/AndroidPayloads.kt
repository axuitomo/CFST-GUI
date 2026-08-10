package io.github.axuitomo.cfstgui

import org.json.JSONObject

object AndroidPayloads {
    @JvmStatic
    fun firstNonEmpty(vararg values: String?): String {
        for (value in values) {
            if (!value.isNullOrBlank()) {
                return value
            }
        }
        return ""
    }

    @JvmStatic
    fun extractExportTargetURI(payloadJSON: String?): String {
        return try {
            val payload = JSONObject(payloadJSON.orEmpty())
            val exportConfig = payload.optJSONObject("config")?.optJSONObject("export") ?: return ""
            firstNonEmpty(exportConfig.optString("target_uri", ""), exportConfig.optString("targetUri", "")).trim()
        } catch (_: Exception) {
            ""
        }
    }

    @JvmStatic
    fun extractTargetURI(payloadJSON: String?): String {
        return try {
            val payload = JSONObject(payloadJSON.orEmpty())
            var value = firstNonEmpty(payload.optString("target_uri", ""), payload.optString("targetUri", ""))
            if (value.isBlank()) {
                val config = payload.optJSONObject("config")
                    ?: payload.optJSONObject("config_snapshot")
                    ?: payload.optJSONObject("configSnapshot")
                val exportConfig = config?.optJSONObject("export")
                if (exportConfig != null) {
                    value = firstNonEmpty(exportConfig.optString("target_uri", ""), exportConfig.optString("targetUri", ""))
                }
            }
            value.trim()
        } catch (_: Exception) {
            ""
        }
    }

    @JvmStatic
    fun withAndroidExportURI(payloadJSON: String?, exportURI: String?): String {
        val normalizedExportURI = exportURI?.trim().orEmpty()
        if (normalizedExportURI.isEmpty()) {
            return payloadJSON.orEmpty()
        }
        return try {
            val payload = JSONObject(payloadJSON.orEmpty())
            payload.put("android_export_uri", normalizedExportURI)
            payload.toString()
        } catch (_: Exception) {
            payloadJSON.orEmpty()
        }
    }

}
