package io.github.axuitomo.cfstgui

import android.content.Context
import android.net.Uri
import org.json.JSONObject

object AndroidPayloads {
    private val RESULT_URI_KEYS = arrayOf(
        "path",
        "source_path",
        "sourcePath",
        "target_path",
        "targetPath",
        "export_path",
        "exportPath",
    )

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
    fun withPrivateResultFilePath(context: Context, payloadJSON: String?): String {
        val sourcePayload = payloadJSON.orEmpty()
        val payload = JSONObject(sourcePayload)
        val resultURI = extractResultFileURI(payload)
        if (resultURI.isEmpty()) {
            return sourcePayload
        }
        val uri = Uri.parse(resultURI)
        val copied = AndroidPrivateFiles.copyResultUriToPrivateFile(
            context,
            uri,
            AndroidPrivateFiles.queryDisplayName(context, uri),
        )
        payload.put("path", copied.absolutePath)
        payload.put("source_uri", resultURI)
        payload.put("sourceUri", resultURI)
        payload.remove("export_path")
        payload.remove("exportPath")
        payload.remove("source_path")
        payload.remove("sourcePath")
        payload.remove("target_path")
        payload.remove("targetPath")
        return payload.toString()
    }

    @JvmStatic
    fun extractResultFileURI(payload: JSONObject): String {
        for (key in RESULT_URI_KEYS) {
            val value = payload.optString(key, "")
            if (isContentDocumentURI(value)) {
                return value.trim()
            }
        }
        val config = payload.optJSONObject("config")
            ?: payload.optJSONObject("config_snapshot")
            ?: payload.optJSONObject("configSnapshot")
        val exportConfig = config?.optJSONObject("export")
        if (exportConfig != null) {
            val value = firstNonEmpty(exportConfig.optString("target_uri", ""), exportConfig.optString("targetUri", ""))
            if (isContentDocumentURI(value)) {
                return value.trim()
            }
        }
        return ""
    }

    private fun isContentDocumentURI(value: String?): Boolean {
        val normalized = value?.trim().orEmpty()
        return normalized.startsWith("content://") && !AndroidStorageBridge.isTreeURIString(normalized)
    }
}
