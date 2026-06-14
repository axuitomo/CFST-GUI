package io.github.axuitomo.cfstgui

import android.app.DownloadManager
import android.content.Intent
import android.net.Uri
import org.junit.Assert.assertEquals
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RuntimeEnvironment
import org.robolectric.RobolectricTestRunner
import org.robolectric.Shadows
import org.robolectric.annotation.Config

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [35])
class UpdatePackageCleanupReceiverTest {
    @Test
    fun packageReplacementRemovesRecordedDownloadManagerPackages() {
        val context = RuntimeEnvironment.getApplication()
        val manager = context.getSystemService(android.content.Context.DOWNLOAD_SERVICE) as DownloadManager
        val shadowManager = Shadows.shadowOf(manager)
        val downloadId = manager.enqueue(DownloadManager.Request(Uri.parse("https://example.test/app.apk")))
        AndroidUpdateInstaller.recordDownloadedPackage(context, updatePackage(downloadId))

        UpdatePackageCleanupReceiver().onReceive(context, Intent(Intent.ACTION_MY_PACKAGE_REPLACED))

        assertEquals(0, shadowManager.requestCount)
    }

    @Test
    fun unrelatedBroadcastDoesNotCleanUpdatePackages() {
        val context = RuntimeEnvironment.getApplication()
        val manager = context.getSystemService(android.content.Context.DOWNLOAD_SERVICE) as DownloadManager
        val shadowManager = Shadows.shadowOf(manager)
        val downloadId = manager.enqueue(DownloadManager.Request(Uri.parse("https://example.test/app.apk")))
        AndroidUpdateInstaller.recordDownloadedPackage(context, updatePackage(downloadId))

        UpdatePackageCleanupReceiver().onReceive(context, Intent(Intent.ACTION_PACKAGE_REPLACED))

        assertEquals(1, shadowManager.requestCount)
    }

    private fun updatePackage(downloadId: Long): AndroidUpdateDownloads.DownloadedUpdatePackage {
        return AndroidUpdateDownloads.DownloadedUpdatePackage(
            downloadId,
            Uri.parse("content://downloads/my_downloads/$downloadId"),
            "cfst-gui-android-release.apk",
            "Download/CFST-GUI/cfst-gui-android-release.apk",
        )
    }
}
