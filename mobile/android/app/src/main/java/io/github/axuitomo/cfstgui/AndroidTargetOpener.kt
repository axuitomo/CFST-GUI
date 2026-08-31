package io.github.axuitomo.cfstgui

import android.content.ActivityNotFoundException
import android.content.Context
import android.content.Intent
import android.net.Uri
import android.provider.DocumentsContract
import android.util.Log
import java.util.Locale
import java.io.File
import androidx.core.content.FileProvider

object AndroidTargetOpener {
    @JvmStatic
    fun directoryChooserIntent(uri: Uri): Intent {
        val viewIntent = Intent(Intent.ACTION_VIEW).apply {
            setDataAndType(uri, DocumentsContract.Document.MIME_TYPE_DIR)
            addFlags(Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_GRANT_READ_URI_PERMISSION or Intent.FLAG_GRANT_PREFIX_URI_PERMISSION)
        }
        return Intent.createChooser(viewIntent, "选择工具打开目录").apply {
            addFlags(Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_GRANT_READ_URI_PERMISSION or Intent.FLAG_GRANT_PREFIX_URI_PERMISSION)
        }
    }
    const val EXPORT_DIRECTORY_PERMISSION_LOST_MESSAGE = "Android 未持有所选导出目录的持久化权限，请重新选择导出目录。"
    const val EXPORT_DIRECTORY_OPEN_ERROR_MESSAGE = "系统无法打开该导出目录，请安装或启用文件管理器后重试。"

    fun interface IntentStarter {
        fun tryStart(intent: Intent): Boolean
    }

    @JvmStatic
    @JvmOverloads
    fun openTargetPath(context: Context, targetPath: String?, starter: IntentStarter = ContextIntentStarter(context)) {
        val normalized = targetPath?.trim().orEmpty()
        if (normalized.isEmpty()) {
            throw IllegalArgumentException("缺少可打开的目标路径。")
        }
        val uri = Uri.parse(normalized)
        when (uri.scheme?.trim()?.lowercase(Locale.ROOT).orEmpty()) {
            "content" -> {
                if (DocumentsContract.isTreeUri(uri)) {
                    openTreeUri(context, uri, starter)
                    return
                }
                openContentUri(context, uri, starter)
                return
            }
            "http", "https" -> {
                startExternalIntent(starter, AndroidOpenIntents.linkIntent(uri), "没有可用的应用可以打开该链接。")
                return
            }
            else -> {
                val localPath = if (uri.scheme.equals("file", ignoreCase = true)) uri.path.orEmpty() else normalized
                openLocalDirectory(context, File(localPath), starter)
                return
            }
        }
    }

    @JvmStatic
    fun hasPersistedUriPermission(context: Context, uri: Uri?): Boolean {
        if (uri == null) {
            return false
        }
        for (permission in context.contentResolver.persistedUriPermissions) {
            if (uri == permission.uri && permission.isReadPermission && permission.isWritePermission) {
                return true
            }
        }
        return false
    }

    @JvmStatic
    fun requireExportTreeUriPermission(hasPermission: Boolean) {
        if (!hasPermission) {
            throw IllegalStateException(EXPORT_DIRECTORY_PERMISSION_LOST_MESSAGE)
        }
    }

    private fun openLocalDirectory(context: Context, directory: File, starter: IntentStarter) {
        if (!directory.isDirectory) {
            throw IllegalStateException("应用数据目录不存在：${directory.absolutePath}")
        }
        val uri = FileProvider.getUriForFile(context, context.packageName + ".fileprovider", directory)
        startExternalIntent(starter, directoryChooserIntent(uri), "没有可用的文件管理工具可以打开应用数据目录。")
    }

    private fun openContentUri(context: Context, uri: Uri, starter: IntentStarter) {
        val mimeType = mimeTypeForUri(context, uri)
        if (starter.tryStart(AndroidOpenIntents.viewFileIntent(uri, mimeType))) {
            return
        }
        startExternalIntent(
            starter,
            AndroidOpenIntents.shareFileChooserIntent(uri, mimeType),
            "没有可用的应用可以查看或分享该文件。",
        )
    }

    private fun openTreeUri(context: Context, treeUri: Uri, starter: IntentStarter) {
        requireExportTreeUriPermission(hasPersistedUriPermission(context, treeUri))
        for (intent in AndroidDirectoryOpenIntents.openDirectoryIntents(treeUri)) {
            if (starter.tryStart(intent)) {
                return
            }
        }
        throw IllegalStateException(EXPORT_DIRECTORY_OPEN_ERROR_MESSAGE)
    }

    private fun mimeTypeForUri(context: Context, uri: Uri): String {
        val mimeType = context.contentResolver.getType(uri)
        if (!mimeType.isNullOrBlank()) {
            return mimeType
        }
        return AndroidOpenIntents.mimeTypeForName(AndroidPrivateFiles.queryDisplayName(context, uri))
    }

    private fun startExternalIntent(starter: IntentStarter, intent: Intent, errorMessage: String) {
        if (!starter.tryStart(intent)) {
            throw IllegalStateException(errorMessage)
        }
    }

    private class ContextIntentStarter(private val context: Context) : IntentStarter {
        override fun tryStart(intent: Intent): Boolean {
            return try {
                context.startActivity(intent)
                true
            } catch (error: ActivityNotFoundException) {
                Log.e(TAG, "Failed to start external Android intent.", error)
                false
            } catch (error: SecurityException) {
                Log.e(TAG, "Failed to start external Android intent.", error)
                false
            }
        }
    }

    private const val TAG = "AndroidTargetOpener"
}
