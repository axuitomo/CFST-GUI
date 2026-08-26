package io.github.axuitomo.cfstgui

import android.content.Context
import java.io.File
import org.json.JSONObject

object AndroidExportFlow {
    fun interface ExportAction { fun export(payloadJSON: String): String }

    @JvmStatic
    fun exportConfig(context: Context, payloadJSON: String, action: ExportAction): String {
        val response = action.export(payloadJSON)
        val targetURI = AndroidPayloads.extractTargetURI(payloadJSON)
        return AndroidPluginCommands.finalizeServiceResponse(context, if (targetURI.isNotEmpty()) AndroidExportResponses.writeConfigExportToURI(context, response, targetURI) else response)
    }

    @JvmStatic
    fun exportConfigArchive(context: Context, payloadJSON: String, action: ExportAction): String {
        val response = action.export(payloadJSON)
        val targetURI = AndroidPayloads.extractTargetURI(payloadJSON)
        return AndroidPluginCommands.finalizeServiceResponse(context, if (targetURI.isNotEmpty()) AndroidExportResponses.writeConfigArchiveToURI(context, response, targetURI) else response)
    }

    @JvmStatic
    fun exportResultsCSV(context: Context, payloadJSON: String, action: ExportAction): String {
        val response = action.export(payloadJSON)
        val targetURI = AndroidPayloads.extractTargetURI(payloadJSON)
        return AndroidPluginCommands.finalizeServiceResponse(context, if (targetURI.isNotEmpty()) AndroidExportResponses.writeCSVExportToURI(context, response, targetURI) else response)
    }

    @JvmStatic
    fun exportDebugLog(context: Context, payloadJSON: String, action: ExportAction): String {
        return exportLargeFileToSaf(context, payloadJSON, action, "cfip-log.txt") { response, target ->
            AndroidExportResponses.writeDebugLogExportToURI(context, response, target)
        }
    }

    @JvmStatic
    fun exportDiagnosticPackage(context: Context, payloadJSON: String, action: ExportAction): String {
        return exportLargeFileToSaf(context, payloadJSON, action, "cfst-diagnostics.zip") { response, target ->
            AndroidExportResponses.writeDiagnosticPackageExportToURI(context, response, target)
        }
    }

    private fun exportLargeFileToSaf(
        context: Context,
        payloadJSON: String,
        action: ExportAction,
        fallbackName: String,
        legacyWriter: (String, String) -> String,
    ): String {
        val targetURI = AndroidPayloads.extractTargetURI(payloadJSON)
        if (targetURI.isEmpty()) return AndroidPluginCommands.finalizeServiceResponse(context, action.export(payloadJSON))
        val exportDir = File(context.cacheDir, "export")
        exportDir.mkdirs()
        val tempFile = File(exportDir, "${System.currentTimeMillis()}-$fallbackName")
        val request = AndroidPayloads.withTempFilePath(payloadJSON, tempFile.absolutePath)
        return try {
            val response = action.export(request)
            val command = JSONObject(response)
            val data = command.optJSONObject("data")
            val tempPath = data?.optString("temp_file_path", "").orEmpty()
            if (tempPath.isBlank()) {
                AndroidPluginCommands.finalizeServiceResponse(context, legacyWriter(response, targetURI))
            } else {
                val writtenURI = AndroidStorageBridge.copyFileToSafTarget(context, targetURI, File(tempPath), true)
                data.put("target_uri", writtenURI)
                data.put("path", writtenURI)
                data.remove("temp_file_path")
                AndroidPluginCommands.finalizeServiceResponse(context, command.toString())
            }
        } finally {
            tempFile.delete()
        }
    }
}
