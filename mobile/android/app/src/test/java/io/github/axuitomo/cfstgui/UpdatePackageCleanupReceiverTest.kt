package io.github.axuitomo.cfstgui

import android.app.DownloadManager
import android.content.Context
import android.content.Intent
import android.net.Uri
import java.io.File
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.RuntimeEnvironment
import org.robolectric.Shadows
import org.robolectric.annotation.Config

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [35])
class UpdatePackageCleanupReceiverTest {
    @Test
    fun packageReplacementRemovesRecordedPrivateUpdatePackagesAndParts() {
        val context = RuntimeEnvironment.getApplication()
        val apk = File(AndroidUpdateInstaller.updateDownloadDirectory(context), "cfst-gui-android-release.apk")
        val part = File(AndroidUpdateInstaller.updateDownloadDirectory(context), "cfst-gui-android-release.apk.1.part")
        AndroidUpdateInstaller.prepareUpdateDirectory(context)
        apk.writeText("apk")
        part.writeText("part")
        AndroidUpdateInstaller.recordDownloadedPackage(context, updatePackage(context, apk))

        UpdatePackageCleanupReceiver().onReceive(context, Intent(Intent.ACTION_MY_PACKAGE_REPLACED))

        assertFalse(apk.exists())
        assertFalse(part.exists())
    }

    @Test
    fun packageReplacementPerformsOneTimeLegacyDownloadManagerCleanup() {
        val context = RuntimeEnvironment.getApplication()
        val manager = context.getSystemService(Context.DOWNLOAD_SERVICE) as DownloadManager
        val shadowManager = Shadows.shadowOf(manager)
        val downloadId = manager.enqueue(DownloadManager.Request(Uri.parse("https://example.test/app.apk")))
        context.getSharedPreferences("cfst_android_update_downloads", Context.MODE_PRIVATE)
            .edit()
            .putStringSet("download_ids", setOf(downloadId.toString()))
            .apply()

        UpdatePackageCleanupReceiver().onReceive(context, Intent(Intent.ACTION_MY_PACKAGE_REPLACED))

        assertEquals(0, shadowManager.requestCount)
        assertTrue(
            context.getSharedPreferences("cfst_android_update_downloads", Context.MODE_PRIVATE)
                .getStringSet("download_ids", emptySet())
                .orEmpty()
                .isEmpty(),
        )
    }

    @Test
    fun unrelatedBroadcastDoesNotCleanUpdatePackages() {
        val context = RuntimeEnvironment.getApplication()
        val apk = File(AndroidUpdateInstaller.updateDownloadDirectory(context), "cfst-gui-android-release.apk")
        AndroidUpdateInstaller.prepareUpdateDirectory(context)
        apk.writeText("apk")
        AndroidUpdateInstaller.recordDownloadedPackage(context, updatePackage(context, apk))

        UpdatePackageCleanupReceiver().onReceive(context, Intent(Intent.ACTION_PACKAGE_REPLACED))

        assertTrue(apk.exists())
    }

    private fun updatePackage(context: Context, file: File): AndroidUpdateDownloads.DownloadedUpdatePackage {
        return AndroidUpdateDownloads.DownloadedUpdatePackage(
            file,
            Uri.parse("content://${context.packageName}.fileprovider/update_downloads/${file.name}"),
            file.name,
            AndroidUpdateInstaller.displayDownloadPath(file.name),
        )
    }
}
