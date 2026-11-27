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

    @JvmStatic
    fun withTempFilePath(payloadJSON: String?, path: String?): String {
        val normalized = path?.trim().orEmpty()
        if (normalized.isEmpty()) return payloadJSON.orEmpty()
        return try {
            JSONObject(payloadJSON.orEmpty()).put("temp_file_path", normalized).toString()
        } catch (_: Exception) {
            payloadJSON.orEmpty()
        }
    }

    @JvmStatic
    fun formatProbeSpeedContent(payload: JSONObject?): String {
        if (payload == null) {
            return "正在采集测速样本。"
        }
        val ip = firstNonEmpty(payload.optString("ip", ""), "当前 IP")
        val currentReady = optionalBoolean(payload, "current_ready", "currentReady")
        val averageReady = optionalBoolean(payload, "average_ready", "averageReady")
        val current = payload.optDouble("current_speed_mb_s", payload.optDouble("currentSpeedMbS", 0.0))
        val average = payload.optDouble("average_speed_mb_s", payload.optDouble("averageSpeedMbS", 0.0))
        val currentText = if (currentReady == true && current.isFinite()) {
            String.format(java.util.Locale.ROOT, "%.2f MB/s", current)
        } else {
            "-"
        }
        val averageText = if (averageReady == true && average.isFinite()) {
            String.format(java.util.Locale.ROOT, "%.2f MB/s", average)
        } else {
            "-"
        }
        if (currentReady != true && averageReady != true) {
            return "$ip 正在测速中。"
        }
        return String.format(java.util.Locale.ROOT, "%s 当前 %s，均速 %s。", ip, currentText, averageText)
    }

    @JvmStatic
    fun optionalBoolean(payload: JSONObject, vararg keys: String): Boolean? {
        for (key in keys) {
            if (payload.has(key) && !payload.isNull(key)) {
                return payload.optBoolean(key)
            }
        }
        return null
    }

}
