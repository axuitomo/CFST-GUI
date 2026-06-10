package io.github.axuitomo.cfstgui

import android.annotation.SuppressLint
import android.app.Activity
import android.content.Context
import android.content.Intent
import android.net.Uri
import com.getcapacitor.JSObject

object AndroidPathSelectionResults {
    @JvmStatic
    fun commandForResult(context: Context, mode: String?, resultCode: Int, resultData: Intent?): JSObject {
        val normalizedMode = AndroidPathSelection.normalizeMode(mode)
        val data = JSObject()
        data.put("mode", normalizedMode)

        val uri = resultData?.data
        if (resultCode != Activity.RESULT_OK || uri == null) {
            data.put("canceled", true)
            return AndroidPluginCommands.command("PATH_SELECTION_CANCELED", data, "已取消系统文件选择。", true)
        }

        val displayName = AndroidPrivateFiles.queryDisplayName(context, uri)
        data.put("canceled", false)
        data.put("display_name", displayName)
        data.put("uri", uri.toString())

        return when {
            AndroidPathSelection.isExportDirectoryMode(normalizedMode) -> {
                persistUriPermission(context, resultData, uri)
                data.put("target_uri", uri.toString())
                data.put("path", displayName.ifEmpty { uri.toString() })
                AndroidPluginCommands.command("PATH_SELECTED", data, "已选择导出目录。", true)
            }
            AndroidPathSelection.isExportFileMode(normalizedMode) ||
                AndroidPathSelection.isConfigExportMode(normalizedMode) ||
                AndroidPathSelection.isConfigArchiveExportMode(normalizedMode) -> {
                data.put("target_uri", uri.toString())
                data.put("path", displayName.ifEmpty { uri.toString() })
                AndroidPluginCommands.command("PATH_SELECTED", data, exportFileMessage(normalizedMode), true)
            }
            AndroidPathSelection.isConfigArchiveImportMode(normalizedMode) -> {
                data.put("content_base64", AndroidPrivateFiles.readUriBase64(context, uri))
                data.put("path", displayName.ifEmpty { uri.toString() })
                AndroidPluginCommands.command("PATH_SELECTED", data, "已读取配置压缩包。", true)
            }
            AndroidPathSelection.isConfigImportMode(normalizedMode) -> {
                data.put("content", AndroidPrivateFiles.readUriText(context, uri))
                data.put("path", displayName.ifEmpty { uri.toString() })
                AndroidPluginCommands.command("PATH_SELECTED", data, "已读取配置文件。", true)
            }
            else -> {
                val copied = AndroidPrivateFiles.copyImportUriToPrivateFile(context, uri, displayName)
                data.put("path", copied.absolutePath)
                AndroidPluginCommands.command("PATH_SELECTED", data, "已选择输入源文件。", true)
            }
        }
    }

    private fun exportFileMessage(mode: String): String {
        return if (AndroidPathSelection.isConfigExportMode(mode) || AndroidPathSelection.isConfigArchiveExportMode(mode)) {
            "已选择配置导出文件。"
        } else {
            "已选择导出文件。"
        }
    }

    @SuppressLint("WrongConstant")
    private fun persistUriPermission(context: Context, data: Intent, uri: Uri) {
        val flags = data.flags and (Intent.FLAG_GRANT_READ_URI_PERMISSION or Intent.FLAG_GRANT_WRITE_URI_PERMISSION)
        if (flags == 0) {
            return
        }
        try {
            context.contentResolver.takePersistableUriPermission(uri, flags)
        } catch (_: SecurityException) {
            // Some providers grant one-shot access only; copied imports do not need persisted access.
        }
    }
}
