package io.github.axuitomo.cfstgui

import android.content.Context

object AndroidExportFlow {
    fun interface ExportAction {
        fun export(payloadJSON: String): String
    }

    @JvmStatic
    fun exportConfig(context: Context, payloadJSON: String, action: ExportAction): String {
        var response = action.export(payloadJSON)
        val targetURI = AndroidPayloads.extractTargetURI(payloadJSON)
        if (targetURI.isNotEmpty()) {
            response = AndroidExportResponses.writeConfigExportToURI(context, response, targetURI)
        }
        return AndroidPluginCommands.finalizeServiceResponse(context, response)
    }

    @JvmStatic
    fun exportConfigArchive(context: Context, payloadJSON: String, action: ExportAction): String {
        var response = action.export(payloadJSON)
        val targetURI = AndroidPayloads.extractTargetURI(payloadJSON)
        if (targetURI.isNotEmpty()) {
            response = AndroidExportResponses.writeConfigArchiveToURI(context, response, targetURI)
        }
        return AndroidPluginCommands.finalizeServiceResponse(context, response)
    }

    @JvmStatic
    fun exportResultsCSV(context: Context, payloadJSON: String, action: ExportAction): String {
        var response = action.export(payloadJSON)
        val targetURI = AndroidPayloads.extractTargetURI(payloadJSON)
        if (targetURI.isNotEmpty()) {
            response = AndroidExportResponses.writeCSVExportToURI(context, response, targetURI)
        }
        return AndroidPluginCommands.finalizeServiceResponse(context, response)
    }

    @JvmStatic
    fun exportDebugLog(context: Context, payloadJSON: String, action: ExportAction): String {
        var response = action.export(payloadJSON)
        val targetURI = AndroidPayloads.extractTargetURI(payloadJSON)
        if (targetURI.isNotEmpty()) {
            response = AndroidExportResponses.writeDebugLogExportToURI(context, response, targetURI)
        }
        return AndroidPluginCommands.finalizeServiceResponse(context, response)
    }

    @JvmStatic
    fun exportDiagnosticBundle(context: Context, payloadJSON: String, action: ExportAction): String {
        var response = action.export(payloadJSON)
        val targetURI = AndroidPayloads.extractTargetURI(payloadJSON)
        if (targetURI.isNotEmpty()) {
            response = AndroidExportResponses.writeDiagnosticBundleToURI(context, response, targetURI)
        }
        return AndroidPluginCommands.finalizeServiceResponse(context, response)
    }
}
