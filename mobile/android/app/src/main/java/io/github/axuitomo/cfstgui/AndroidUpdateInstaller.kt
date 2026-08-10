package io.github.axuitomo.cfstgui

import android.app.DownloadManager
import android.content.Context
import android.content.Intent
import android.net.Uri
import android.util.Log
import androidx.core.content.FileProvider
import java.io.File
import java.util.Locale

object AndroidUpdateInstaller {
    private const val APK_CLEANUP_DELAY_MS = 10 * 60 * 1000L
    private const val APK_MIME_TYPE = "application/vnd.android.package-archive"
    private const val DEFAULT_APK_NAME = "cfst-gui-android-release.apk"
    private const val UPDATE_DOWNLOAD_DIRECTORY_NAME = "update_downloads"
    private const val DISPLAY_UPDATE_DIRECTORY = "应用内更新"
    private const val PREFS_NAME = "cfst_android_update_downloads"
    private const val KEY_DOWNLOAD_IDS = "download_ids"
    private const val KEY_FILE_PATHS = "file_paths"
    private const val TAG = "AndroidUpdateInstaller"

    @JvmStatic
    fun displayDownloadPath(fileName: String): String = "$DISPLAY_UPDATE_DIRECTORY/${safePackageFileName(fileName)}"

    @JvmStatic
    fun safePackageFileName(assetName: String?): String {
        val leafName = assetName
            ?.replace('\\', '/')
            ?.substringAfterLast('/')
            ?.trim()
            .orEmpty()
        val normalized = leafName.map { character ->
            if (character.isLetterOrDigit() || character == '.' || character == '_' || character == '-') {
                character
            } else {
                '_'
            }
        }.joinToString("").trim('.', '_', '-')
        val safeName = normalized.ifEmpty { DEFAULT_APK_NAME }
        return if (safeName.lowercase(Locale.ROOT).endsWith(".apk")) {
            safeName
        } else {
            "$safeName.apk"
        }
    }

    @JvmStatic
    fun updateDownloadDirectory(context: Context): File = File(context.filesDir, UPDATE_DOWNLOAD_DIRECTORY_NAME)

    @JvmStatic
    fun updatePackageFile(context: Context, fileName: String): File = File(updateDownloadDirectory(context), safePackageFileName(fileName))

    @JvmStatic
    fun prepareUpdateDirectory(context: Context): File {
        val updateDir = updateDownloadDirectory(context)
        if (!updateDir.isDirectory && !updateDir.mkdirs()) {
            throw IllegalStateException("无法创建应用内更新下载目录。")
        }
        return updateDir
    }

    @JvmStatic
    fun deleteUpdatePartFiles(context: Context): Int {
        val updateDir = updateDownloadDirectory(context)
        val files = updateDir.listFiles() ?: return 0
        var deleted = 0
        for (file in files) {
            val normalizedName = file.name.lowercase(Locale.ROOT)
            if (file.isFile && normalizedName.endsWith(".part") && file.delete()) {
                deleted++
            }
        }
        return deleted
    }

    @JvmStatic
    fun fileProviderAuthority(context: Context): String = context.packageName + ".fileprovider"

    @JvmStatic
    fun contentUriForFile(context: Context, file: File): Uri {
        return FileProvider.getUriForFile(context, fileProviderAuthority(context), file)
    }

    @JvmStatic
    fun installIntentForUri(uri: Uri): Intent {
        return Intent(Intent.ACTION_VIEW).apply {
            setDataAndType(uri, APK_MIME_TYPE)
            addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION or Intent.FLAG_ACTIVITY_NEW_TASK)
        }
    }

    @JvmStatic
    fun startInstall(context: Context, uri: Uri) {
        context.startActivity(installIntentForUri(uri))
    }

    @JvmStatic
    fun recordDownloadedPackage(context: Context, updatePackage: AndroidUpdateDownloads.DownloadedUpdatePackage) {
        val paths = downloadedPackagePaths(context).toMutableSet()
        paths.add(updatePackage.file.absolutePath)
        context.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE)
            .edit()
            .putStringSet(KEY_FILE_PATHS, paths)
            .apply()
    }

    @JvmStatic
    fun removeDownloadedPackage(context: Context, updatePackage: AndroidUpdateDownloads.DownloadedUpdatePackage): Int {
        val removed = deleteFile(updatePackage.file) + deleteUpdatePartFiles(context)
        forgetDownloadedPackage(context, updatePackage.file)
        return removed
    }

    @JvmStatic
    fun cleanupDownloadedPackages(context: Context): Int {
        val prefs = context.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE)
        var removed = 0
        for (path in downloadedPackagePaths(context)) {
            removed += deleteFile(File(path))
        }
        removed += deleteUpdateDirectoryFiles(context)
        removed += removeLegacyDownloadManagerIds(context)
        prefs.edit()
            .remove(KEY_FILE_PATHS)
            .remove(KEY_DOWNLOAD_IDS)
            .apply()
        return removed
    }

    @JvmStatic
    fun schedulePackageCleanup(context: Context, updatePackage: AndroidUpdateDownloads.DownloadedUpdatePackage) {
        val cleanupThread = Thread({
            try {
                Thread.sleep(APK_CLEANUP_DELAY_MS)
                removeDownloadedPackage(context, updatePackage)
            } catch (interrupted: InterruptedException) {
                Thread.currentThread().interrupt()
            } catch (error: Exception) {
                Log.e(TAG, "Failed to clean Android update APK.", error)
            }
        }, "CFST update APK cleanup")
        cleanupThread.isDaemon = true
        cleanupThread.start()
    }

    private fun downloadedPackagePaths(context: Context): Set<String> {
        return context.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE)
            .getStringSet(KEY_FILE_PATHS, emptySet())
            ?.toSet()
            .orEmpty()
    }

    private fun legacyDownloadedPackageIds(context: Context): Set<String> {
        return context.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE)
            .getStringSet(KEY_DOWNLOAD_IDS, emptySet())
            ?.toSet()
            .orEmpty()
    }

    private fun forgetDownloadedPackage(context: Context, file: File) {
        val paths = downloadedPackagePaths(context).toMutableSet()
        paths.remove(file.absolutePath)
        val editor = context.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE).edit()
        if (paths.isEmpty()) {
            editor.remove(KEY_FILE_PATHS)
        } else {
            editor.putStringSet(KEY_FILE_PATHS, paths)
        }
        editor.apply()
    }

    private fun deleteUpdateDirectoryFiles(context: Context): Int {
        val updateDir = updateDownloadDirectory(context)
        val files = updateDir.listFiles() ?: return 0
        var deleted = 0
        for (file in files) {
            val normalizedName = file.name.lowercase(Locale.ROOT)
            if (file.isFile && (normalizedName.endsWith(".apk") || normalizedName.endsWith(".part")) && file.delete()) {
                deleted++
            }
        }
        return deleted
    }

    private fun deleteFile(file: File): Int {
        return if (file.isFile && file.delete()) 1 else 0
    }

    private fun removeLegacyDownloadManagerIds(context: Context): Int {
        val ids = legacyDownloadedPackageIds(context).mapNotNull { value -> value.toLongOrNull() }
        if (ids.isEmpty()) {
            return 0
        }
        val manager = context.getSystemService(Context.DOWNLOAD_SERVICE) as? DownloadManager ?: return 0
        return manager.remove(*ids.toLongArray())
    }
}
