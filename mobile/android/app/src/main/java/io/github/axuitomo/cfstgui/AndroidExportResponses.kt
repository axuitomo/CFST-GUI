package io.github.axuitomo.cfstgui

import android.content.Context
import android.util.Base64
import java.nio.charset.StandardCharsets
import org.json.JSONArray
import org.json.JSONObject

object AndroidExportResponses {
    @JvmStatic
    fun writeConfigExportToURI(context: Context, responseJSON: String, targetURI: String?): String {
        return writeTextExport(
            context,
            responseJSON,
            targetURI,
            "content",
            "cfst-gui-config.json",
            "配置导出内容为空，未写入系统选择的目标。",
            "Android 配置导出到系统文件失败：",
            null,
        )
    }

    @JvmStatic
    fun writeConfigArchiveToURI(context: Context, responseJSON: String, targetURI: String?): String {
        return writeBase64Export(
            context,
            responseJSON,
            targetURI,
            "cfst-gui-config.zip",
            "配置压缩包内容为空，未写入系统选择的目标。",
            "Android 配置压缩包导出到系统文件失败：",
            null,
        )
    }

    @JvmStatic
    fun writeDiagnosticPackageExportToURI(context: Context, responseJSON: String, targetURI: String?): String {
        return writeBase64Export(
            context,
            responseJSON,
            targetURI,
            "cfst-diagnostics.zip",
            "诊断包内容为空，未写入系统选择的目标。",
            "Android 诊断包导出到系统文件失败：",
            FailedCommand("DIAGNOSTIC_PACKAGE_WRITE_FAILED"),
        )
    }

    @JvmStatic
    fun writeCSVExportToURI(context: Context, responseJSON: String, targetURI: String?): String {
        return writeBase64Export(
            context,
            responseJSON,
            targetURI,
            "result.csv",
            "CSV 导出内容为空，未写入系统选择的目标。",
            "Android CSV 导出到系统文件失败：",
            FailedCommand("RESULTS_CSV_EXPORT_WRITE_FAILED"),
        )
    }

    @JvmStatic
    fun writeDebugLogExportToURI(context: Context, responseJSON: String, targetURI: String?): String {
        return writeBase64Export(
            context,
            responseJSON,
            targetURI,
            "cfip-log.txt",
            "调试日志内容为空，未写入系统选择的目标。",
            "Android 调试日志导出到系统文件失败：",
            FailedCommand("DEBUG_LOG_EXPORT_WRITE_FAILED"),
        )
    }

    private fun writeTextExport(
        context: Context,
        responseJSON: String,
        targetURI: String?,
        contentField: String,
        fallbackFileName: String,
        emptyMessage: String,
        failurePrefix: String,
        failedCommand: FailedCommand?,
    ): String {
        return try {
            val command = JSONObject(responseJSON)
            val data = command.optJSONObject("data") ?: return responseJSON
            val content = data.optString(contentField, "")
            if (content.isEmpty()) {
                return exportWriteFailed(command, data, emptyMessage, failedCommand)
            }
            val writtenURI = AndroidStorageBridge.writeBytesToSafTarget(
                context,
                targetURI,
                data.optString("file_name", fallbackFileName),
                content.toByteArray(StandardCharsets.UTF_8),
                true,
            )
            markWritten(data, writtenURI, contentField)
            command.toString()
        } catch (error: Exception) {
            writeFailureResponse(responseJSON, failurePrefix + error.message, failedCommand)
        }
    }

    private fun writeBase64Export(
        context: Context,
        responseJSON: String,
        targetURI: String?,
        fallbackFileName: String,
        emptyMessage: String,
        failurePrefix: String,
        failedCommand: FailedCommand?,
    ): String {
        return try {
            val command = JSONObject(responseJSON)
            val data = command.optJSONObject("data") ?: return responseJSON
            val contentBase64 = data.optString("content_base64", "")
            if (contentBase64.isEmpty()) {
                return exportWriteFailed(command, data, emptyMessage, failedCommand)
            }
            val content = Base64.decode(contentBase64, Base64.DEFAULT)
            val writtenURI = AndroidStorageBridge.writeBytesToSafTarget(
                context,
                targetURI,
                data.optString("file_name", fallbackFileName),
                content,
                true,
            )
            markWritten(data, writtenURI, "content_base64")
            command.toString()
        } catch (error: Exception) {
            writeFailureResponse(responseJSON, failurePrefix + error.message, failedCommand)
        }
    }

    private fun writeFailureResponse(responseJSON: String, message: String, failedCommand: FailedCommand?): String {
        return try {
            val command = JSONObject(responseJSON)
            val data = command.optJSONObject("data")
            exportWriteFailed(command, data, message, failedCommand)
        } catch (_: Exception) {
            responseJSON
        }
    }

    private fun markWritten(data: JSONObject, writtenURI: String, contentField: String) {
        data.put("target_uri", writtenURI)
        data.put("path", writtenURI)
        data.remove(contentField)
    }

    private fun exportWriteFailed(
        command: JSONObject,
        data: JSONObject?,
        message: String,
        failedCommand: FailedCommand?,
    ): String {
        appendWarning(command, data, message)
        if (failedCommand != null) {
            command.put("code", failedCommand.code)
            command.put("message", message)
            command.put("ok", false)
        }
        return command.toString()
    }

    private fun appendWarning(command: JSONObject, data: JSONObject?, warning: String) {
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

    private class FailedCommand(val code: String)
}
