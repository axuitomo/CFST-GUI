package io.github.axuitomo.cfstgui

import android.content.Context
import android.database.Cursor
import android.net.Uri
import android.provider.OpenableColumns
import android.util.Base64
import java.io.ByteArrayOutputStream
import java.io.File
import java.io.FileOutputStream
import java.io.InputStream
import java.io.OutputStream

object AndroidPrivateFiles {
    @JvmStatic
    fun queryDisplayName(context: Context, uri: Uri): String {
        var cursor: Cursor? = null
        try {
            cursor = context.contentResolver.query(uri, null, null, null, null)
            if (cursor != null && cursor.moveToFirst()) {
                val index = cursor.getColumnIndex(OpenableColumns.DISPLAY_NAME)
                if (index >= 0) {
                    return cursor.getString(index).orEmpty()
                }
            }
        } catch (_: Exception) {
            // Fall back to URI path below.
        } finally {
            cursor?.close()
        }
        return uri.lastPathSegment.orEmpty()
    }

    @JvmStatic
    fun copyImportUriToPrivateFile(context: Context, uri: Uri, displayName: String?): File {
        val dir = File(context.filesDir, "imports")
        if (!dir.exists() && !dir.mkdirs()) {
            throw IllegalStateException("创建导入目录失败：" + dir.absolutePath)
        }
        val name = sanitizeFileName(displayName).ifEmpty { "source.txt" }
        val target = File(dir, System.currentTimeMillis().toString() + "-" + name)
        context.contentResolver.openInputStream(uri).use { input ->
            if (input == null) {
                throw IllegalStateException("无法读取选择的文件。")
            }
            FileOutputStream(target).use { output ->
                copy(input, output)
            }
        }
        return target
    }

    @JvmStatic
    fun copyResultUriToPrivateFile(context: Context, uri: Uri, displayName: String?): File {
        val dir = File(context.filesDir, "result-files")
        if (!dir.exists() && !dir.mkdirs()) {
            throw IllegalStateException("创建结果缓存目录失败：" + dir.absolutePath)
        }
        val name = sanitizeFileName(displayName).ifEmpty { "result.csv" }
        val target = File(dir, System.currentTimeMillis().toString() + "-" + name)
        context.contentResolver.openInputStream(uri).use { input ->
            if (input == null) {
                throw IllegalStateException("无法读取选择的结果文件。")
            }
            FileOutputStream(target).use { output ->
                copy(input, output)
            }
        }
        return target
    }

    @JvmStatic
    fun readUriText(context: Context, uri: Uri): String {
        context.contentResolver.openInputStream(uri).use { input ->
            if (input == null) {
                throw IllegalStateException("无法读取选择的配置文件。")
            }
            ByteArrayOutputStream().use { output ->
                copy(input, output)
                return output.toString(Charsets.UTF_8.name())
            }
        }
    }

    @JvmStatic
    fun readUriBase64(context: Context, uri: Uri): String {
        context.contentResolver.openInputStream(uri).use { input ->
            if (input == null) {
                throw IllegalStateException("无法读取选择的配置文件。")
            }
            ByteArrayOutputStream().use { output ->
                copy(input, output)
                return Base64.encodeToString(output.toByteArray(), Base64.NO_WRAP)
            }
        }
    }

    @JvmStatic
    fun sanitizeFileName(value: String?): String =
        value?.replace(Regex("[\\\\/:*?\"<>|]"), "_")?.trim().orEmpty()

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
}
