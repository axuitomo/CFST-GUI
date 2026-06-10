package io.github.axuitomo.cfstgui

import android.content.Context
import android.content.Intent
import android.net.Uri
import android.util.Log
import androidx.core.content.FileProvider
import java.io.File

object AndroidUpdateInstaller {
    private const val APK_CLEANUP_DELAY_MS = 10 * 60 * 1000L
    private const val APK_MIME_TYPE = "application/vnd.android.package-archive"
    private const val TAG = "AndroidUpdateInstaller"

    @JvmStatic
    fun updateDirectory(context: Context): File = File(context.filesDir, "updates")

    @JvmStatic
    fun ensureUpdateDirectory(context: Context): File {
        val updateDir = updateDirectory(context)
        if (!updateDir.exists() && !updateDir.mkdirs()) {
            throw IllegalStateException("创建更新目录失败：" + updateDir.absolutePath)
        }
        return updateDir
    }

    @JvmStatic
    fun installIntent(context: Context, apk: File): Intent {
        val uri = FileProvider.getUriForFile(context, fileProviderAuthority(context), apk)
        return installIntentForUri(uri)
    }

    @JvmStatic
    fun fileProviderAuthority(context: Context): String = context.packageName + ".fileprovider"

    @JvmStatic
    fun installIntentForUri(uri: Uri): Intent {
        return Intent(Intent.ACTION_VIEW).apply {
            setDataAndType(uri, APK_MIME_TYPE)
            addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION or Intent.FLAG_ACTIVITY_NEW_TASK)
        }
    }

    @JvmStatic
    fun startInstall(context: Context, apk: File) {
        context.startActivity(installIntent(context, apk))
    }

    @JvmStatic
    fun schedulePackageCleanup(apk: File) {
        val cleanupThread = Thread({
            try {
                Thread.sleep(APK_CLEANUP_DELAY_MS)
                AndroidUpdatePackages.cleanup(apk.parentFile)
            } catch (interrupted: InterruptedException) {
                Thread.currentThread().interrupt()
            } catch (error: Exception) {
                Log.e(TAG, "Failed to clean Android update APK.", error)
            }
        }, "CFST update APK cleanup")
        cleanupThread.isDaemon = true
        cleanupThread.start()
    }
}
