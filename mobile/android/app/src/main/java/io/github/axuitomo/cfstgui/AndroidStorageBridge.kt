package io.github.axuitomo.cfstgui

import android.content.Context
import android.net.Uri
import androidx.documentfile.provider.DocumentFile
import java.io.File
import java.io.FileInputStream
import java.io.InputStream
import java.io.OutputStream
import java.util.Locale
import org.json.JSONArray
import org.json.JSONObject

object AndroidStorageBridge {
    @JvmStatic
    fun ensureWritablePersistentExportTarget(context: Context, exportURI: String?) {
        val targetURI = exportURI?.trim().orEmpty()
        if (targetURI.isEmpty()) {
            throw IllegalStateException(persistentExportTargetError(targetURI))
        }
        if (!isTreeURIString(targetURI)) {
            throw IllegalStateException(persistentExportTargetError(targetURI))
        }
        val treeUri = Uri.parse(targetURI)
        if (!hasPersistedUriPermission(context, treeUri)) {
            throw IllegalStateException(persistentExportTargetError(targetURI))
        }
        val tree = DocumentFile.fromTreeUri(context, treeUri)
        if (tree == null || !tree.isDirectory || !tree.canWrite()) {
            throw IllegalStateException("Android SAF 导出目录不可写，请重新选择导出目录。")
        }
    }

    @JvmStatic
    fun copyProbeExportToURI(context: Context, responseJSON: String, exportURI: String?): String {
        return try {
            val command = JSONObject(responseJSON)
            val data = command.optJSONObject("data") ?: return responseJSON
            val outputFile = data.optString("outputFile", "")
            if (outputFile.isEmpty()) {
                return responseJSON
            }
            val source = File(outputFile)
            if (!source.exists()) {
                markAndroidExportFailed(data, exportURI, outputFile, "Android 导出文件不存在，无法写入系统选择的目标。")
                appendWarning(command, data, data.optString("android_export_error", "Android 导出文件不存在，无法写入系统选择的目标。"))
                return command.toString()
            }
            val writtenURI = writeFileToSafTarget(context, exportURI, source, false)
            markAndroidExportWritten(data, writtenURI, outputFile)
            data.put("outputFile", writtenURI)
            data.put("androidExportUri", writtenURI)
            data.put("export_path", writtenURI)
            command.toString()
        } catch (error: Exception) {
            try {
                val command = JSONObject(responseJSON)
                val data = command.optJSONObject("data")
                val sourcePath = data?.optString("outputFile", "").orEmpty()
                val message = androidExportFailureMessage(error)
                markAndroidExportFailed(data, exportURI, sourcePath, message)
                appendWarning(command, data, message)
                command.toString()
            } catch (_: Exception) {
                responseJSON
            }
        }
    }

    @JvmStatic
    @Throws(Exception::class)
    fun writeBytesToSafTarget(
        context: Context,
        targetURI: String?,
        targetFileName: String?,
        content: ByteArray?,
        allowOneShotDocumentURI: Boolean,
    ): String {
        val normalizedTargetURI = targetURI?.trim().orEmpty()
        if (normalizedTargetURI.isEmpty()) {
            throw IllegalArgumentException("缺少 Android SAF 导出目标。")
        }
        if (content == null) {
            throw IllegalArgumentException("Android SAF 导出内容为空。")
        }
        if (isTreeURIString(normalizedTargetURI)) {
            val writtenURI = writeBytesToTree(
                context,
                Uri.parse(normalizedTargetURI),
                safTargetFileName(targetFileName, "result.csv"),
                content,
            )
            return writtenURI.toString()
        }
        val documentUri = Uri.parse(normalizedTargetURI)
        if (!allowOneShotDocumentURI && !hasPersistedUriPermission(context, documentUri)) {
            throw IllegalStateException(persistentExportTargetError(normalizedTargetURI))
        }
        context.contentResolver.openOutputStream(documentUri, "wt").use { output ->
            if (output == null) {
                throw IllegalStateException("Android SAF 导出目标无法写入。")
            }
            output.write(content)
        }
        return normalizedTargetURI
    }

    @JvmStatic
    fun isTreeURIString(value: String?): Boolean {
        val normalized = value?.trim().orEmpty()
        return normalized.startsWith("content://") && normalized.contains("/tree/")
    }

    @JvmStatic
    fun persistentExportTargetError(targetURI: String?): String {
        if (targetURI == null || targetURI.trim().isEmpty()) {
            return "缺少 Android SAF 导出目录，请重新选择导出目录。"
        }
        if (isTreeURIString(targetURI)) {
            return "Android 未持有所选导出目录的持久化权限，请重新选择导出目录。"
        }
        return "Android 导出目标不是 SAF 目录，请重新选择导出目录。"
    }

    private fun hasPersistedUriPermission(context: Context, uri: Uri?): Boolean {
        if (uri == null) {
            return false
        }
        return context.contentResolver.persistedUriPermissions.any { permission ->
            uri == permission.uri && permission.isReadPermission && permission.isWritePermission
        }
    }

    @Throws(Exception::class)
    private fun writeFileToSafTarget(
        context: Context,
        targetURI: String?,
        source: File?,
        allowOneShotDocumentURI: Boolean,
    ): String {
        if (source == null || !source.exists()) {
            throw IllegalStateException("Android 导出文件不存在，无法写入系统选择的目标。")
        }
        val normalizedTargetURI = targetURI?.trim().orEmpty()
        if (normalizedTargetURI.isEmpty()) {
            throw IllegalArgumentException("缺少 Android SAF 导出目录，请重新选择导出目录。")
        }
        if (isTreeURIString(normalizedTargetURI)) {
            return writeFileToTree(context, Uri.parse(normalizedTargetURI), source).toString()
        }
        val documentUri = Uri.parse(normalizedTargetURI)
        if (!allowOneShotDocumentURI && !hasPersistedUriPermission(context, documentUri)) {
            throw IllegalStateException(persistentExportTargetError(normalizedTargetURI))
        }
        FileInputStream(source).use { input ->
            context.contentResolver.openOutputStream(documentUri, "wt").use { output ->
                if (output == null) {
                    throw IllegalStateException("Android SAF 导出目标无法写入。")
                }
                copy(input, output)
            }
        }
        return normalizedTargetURI
    }

    @Throws(Exception::class)
    private fun writeFileToTree(context: Context, treeUri: Uri, source: File): Uri {
        val target = ensureWritableTreeFile(context, treeUri, safTargetFileName(source.name, "result.csv"))
        FileInputStream(source).use { input ->
            context.contentResolver.openOutputStream(target.uri, "wt").use { output ->
                if (output == null) {
                    throw IllegalStateException("Android SAF 导出目录中的目标文件无法写入。")
                }
                copy(input, output)
            }
        }
        return target.uri
    }

    @Throws(Exception::class)
    private fun writeBytesToTree(context: Context, treeUri: Uri, targetFileName: String, content: ByteArray): Uri {
        val target = ensureWritableTreeFile(context, treeUri, targetFileName)
        context.contentResolver.openOutputStream(target.uri, "wt").use { output ->
            if (output == null) {
                throw IllegalStateException("Android SAF 导出目录中的目标文件无法写入。")
            }
            output.write(content)
        }
        return target.uri
    }

    private fun ensureWritableTreeFile(context: Context, treeUri: Uri?, fileName: String): DocumentFile {
        if (!hasPersistedUriPermission(context, treeUri)) {
            throw IllegalStateException(persistentExportTargetError(treeUri?.toString().orEmpty()))
        }
        val tree = openStorageTree(context, treeUri)
        if (!tree.canWrite()) {
            throw IllegalStateException("Android SAF 导出目录不可写，请重新选择导出目录。")
        }
        return ensureTreeFile(tree, fileName)
    }

    private fun openStorageTree(context: Context, treeUri: Uri?): DocumentFile {
        val tree = DocumentFile.fromTreeUri(context, treeUri ?: Uri.EMPTY)
        if (tree == null || !tree.isDirectory) {
            throw IllegalStateException("无法访问选择的导出目录。")
        }
        return tree
    }

    private fun ensureTreeFile(parent: DocumentFile, name: String): DocumentFile {
        var existing = parent.findFile(name)
        if (existing != null && existing.isFile) {
            return existing
        }
        if (existing != null && existing.delete()) {
            existing = null
        }
        val created = parent.createFile(mimeTypeForName(name), name)
        if (created == null) {
            throw IllegalStateException("无法创建文件：$name")
        }
        return created
    }

    private fun mimeTypeForName(name: String?): String {
        val lower = name?.lowercase(Locale.ROOT).orEmpty()
        return when {
            lower.endsWith(".csv") -> "text/csv"
            lower.endsWith(".json") -> "application/json"
            lower.endsWith(".txt") -> "text/plain"
            lower.endsWith(".zip") -> "application/zip"
            else -> "application/octet-stream"
        }
    }

    @JvmStatic
    fun safTargetFileName(name: String?, fallback: String?): String {
        var value = name?.trim().orEmpty()
        if (value.isEmpty()) {
            value = fallback?.trim().orEmpty()
        }
        value = value.replace('\\', '/')
        val separator = value.lastIndexOf('/')
        if (separator >= 0) {
            value = value.substring(separator + 1)
        }
        value = value.replace(Regex("[\\\\/:*?\"<>|]"), "_").trim()
        if (value == "." || value == "..") {
            value = ""
        }
        if (value.isEmpty()) {
            return "result.csv"
        }
        return value
    }

    private fun androidExportFailureMessage(error: Exception?): String {
        var message = error?.message.orEmpty()
        if (message.trim().isEmpty()) {
            message = "未知错误"
        }
        if (message.contains("请重新选择导出目录")) {
            return message
        }
        return "Android 导出到系统文件失败：$message"
    }

    @Throws(Exception::class)
    private fun copy(input: InputStream, output: OutputStream) {
        val buffer = ByteArray(8192)
        while (true) {
            val read = input.read(buffer)
            if (read < 0) {
                return
            }
            if (read > 0) {
                output.write(buffer, 0, read)
            }
        }
    }

    @Throws(Exception::class)
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

    @Throws(Exception::class)
    private fun markAndroidExportWritten(data: JSONObject?, exportURI: String?, sourcePath: String?) {
        if (data == null) {
            return
        }
        data.put("android_export_status", "written")
        data.put("androidExportStatus", "written")
        data.put("android_export_uri", exportURI.orEmpty())
        data.put("androidExportUri", exportURI.orEmpty())
        data.put("android_export_source_path", sourcePath.orEmpty())
        data.put("androidExportSourcePath", sourcePath.orEmpty())
        data.put("android_export_error", "")
        data.put("androidExportError", "")
    }

    @Throws(Exception::class)
    private fun markAndroidExportFailed(data: JSONObject?, exportURI: String?, sourcePath: String?, message: String?) {
        if (data == null) {
            return
        }
        data.put("android_export_status", "failed")
        data.put("androidExportStatus", "failed")
        data.put("android_export_uri", exportURI.orEmpty())
        data.put("androidExportUri", exportURI.orEmpty())
        data.put("android_export_source_path", sourcePath.orEmpty())
        data.put("androidExportSourcePath", sourcePath.orEmpty())
        data.put("android_export_error", message.orEmpty())
        data.put("androidExportError", message.orEmpty())
    }
}
