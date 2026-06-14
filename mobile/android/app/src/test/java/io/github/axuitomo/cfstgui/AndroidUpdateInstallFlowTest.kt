package io.github.axuitomo.cfstgui

import android.content.Context
import android.net.Uri
import com.getcapacitor.JSObject
import org.json.JSONObject
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RuntimeEnvironment
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [35])
class AndroidUpdateInstallFlowTest {
    @Test
    fun unavailableUpdateReturnsCompatibleCommandWithoutSideEffects() {
        val downloader = RecordingDownloader()
        val installer = RecordingInstaller()
        val cleanup = RecordingCleanup()

        val command = JSONObject(
            AndroidUpdateInstallFlow.commandForDownloadAndInstall(
                RuntimeEnvironment.getApplication(),
                "1.8.2",
                { payload(false, "", "", "") },
                downloader,
                installer,
                cleanup,
            ).toString(),
        )

        assertEquals("UPDATE_NOT_AVAILABLE", command.getString("code"))
        assertTrue(command.getBoolean("ok"))
        assertFalse(command.getJSONObject("data").getBoolean("update_available"))
        assertEquals(0, downloader.calls)
        assertEquals(0, installer.calls)
        assertEquals(0, cleanup.calls)
    }

    @Test
    fun availableUpdateDownloadsInstallsAndSchedulesCleanup() {
        val context = RuntimeEnvironment.getApplication()
        val sha256 = "1e10ba560383b17472b4cf72fef8f9e76c66815a3e6ae8c5a9b0c5e696b0bdf8"
        val downloader = RecordingDownloader()
        val installer = RecordingInstaller()
        val cleanup = RecordingCleanup()

        val command = JSONObject(
            AndroidUpdateInstallFlow.commandForDownloadAndInstall(
                context,
                "1.8.2",
                { payload(true, "cfst-gui-android-release.apk", "https://example.test/app.apk", sha256) },
                downloader,
                installer,
                cleanup,
            ).toString(),
        )
        val data = command.getJSONObject("data")

        assertEquals("UPDATE_INSTALL_READY", command.getString("code"))
        assertTrue(command.getBoolean("ok"))
        assertEquals("APK 已下载，请在系统安装确认中继续。", command.getString("message"))
        assertEquals("android_install_confirmation", data.getString("next_action"))
        assertTrue(data.getBoolean("install_started"))
        assertEquals("Download/CFST-GUI/cfst-gui-android-release.apk", data.getString("downloaded_path"))
        assertEquals("https://example.test/app.apk", downloader.rawURL)
        assertEquals(sha256, downloader.expectedSHA256)
        assertEquals("1.8.2", downloader.appVersion)
        assertEquals("cfst-gui-android-release.apk", downloader.fileName)
        assertEquals(Uri.parse("content://downloads/my_downloads/42"), installer.uri)
        assertEquals(42L, cleanup.updatePackage.downloadId)
    }

    private fun payload(updateAvailable: Boolean, assetName: String, downloadURL: String, sha256: String): JSObject {
        val payload = JSObject()
        payload.put("update_available", updateAvailable)
        payload.put("asset_name", assetName)
        payload.put("download_url", downloadURL)
        payload.put("sha256", sha256)
        return payload
    }

    private class RecordingDownloader : AndroidUpdateInstallFlow.ApkDownloader {
        var calls = 0
        var rawURL: String? = null
        var fileName: String? = null
        var expectedSHA256: String? = null
        var appVersion: String? = null

        override fun download(context: Context, rawURL: String?, fileName: String, expectedSHA256: String?, appVersion: String?): AndroidUpdateDownloads.DownloadedUpdatePackage {
            calls++
            this.rawURL = rawURL
            this.fileName = fileName
            this.expectedSHA256 = expectedSHA256
            this.appVersion = appVersion
            return AndroidUpdateDownloads.DownloadedUpdatePackage(
                42L,
                Uri.parse("content://downloads/my_downloads/42"),
                fileName,
                AndroidUpdateInstaller.displayDownloadPath(fileName),
            )
        }
    }

    private class RecordingInstaller : AndroidUpdateInstallFlow.ApkInstaller {
        var calls = 0
        lateinit var uri: Uri

        override fun startInstall(context: Context, uri: Uri) {
            calls++
            this.uri = uri
        }
    }

    private class RecordingCleanup : AndroidUpdateInstallFlow.CleanupScheduler {
        var calls = 0
        lateinit var updatePackage: AndroidUpdateDownloads.DownloadedUpdatePackage

        override fun schedule(context: Context, updatePackage: AndroidUpdateDownloads.DownloadedUpdatePackage) {
            calls++
            this.updatePackage = updatePackage
        }
    }
}
