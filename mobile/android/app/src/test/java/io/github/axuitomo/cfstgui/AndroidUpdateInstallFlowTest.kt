package io.github.axuitomo.cfstgui

import android.content.Context
import com.getcapacitor.JSObject
import java.io.File
import java.nio.charset.StandardCharsets
import java.nio.file.Files
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
        val downloader = RecordingDownloader("")
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
        val content = "apk-bytes"
        val sha256 = "1e10ba560383b17472b4cf72fef8f9e76c66815a3e6ae8c5a9b0c5e696b0bdf8"
        val downloader = RecordingDownloader(content)
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
        assertTrue(data.getString("downloaded_path").endsWith("/updates/cfst-gui-android-release.apk"))
        assertEquals("https://example.test/app.apk", downloader.rawURL)
        assertEquals(sha256, downloader.expectedSHA256)
        assertEquals("1.8.2", downloader.appVersion)
        assertEquals(File(context.filesDir, "updates/cfst-gui-android-release.apk").absolutePath, downloader.target.absolutePath)
        assertEquals(downloader.target.absolutePath, installer.apk.absolutePath)
        assertEquals(downloader.target.absolutePath, cleanup.apk.absolutePath)
    }

    private fun payload(updateAvailable: Boolean, assetName: String, downloadURL: String, sha256: String): JSObject {
        val payload = JSObject()
        payload.put("update_available", updateAvailable)
        payload.put("asset_name", assetName)
        payload.put("download_url", downloadURL)
        payload.put("sha256", sha256)
        return payload
    }

    private class RecordingDownloader(private val content: String) : AndroidUpdateInstallFlow.ApkDownloader {
        var calls = 0
        var rawURL: String? = null
        lateinit var target: File
        var expectedSHA256: String? = null
        var appVersion: String? = null

        override fun download(rawURL: String?, target: File, expectedSHA256: String?, appVersion: String?) {
            try {
                calls++
                this.rawURL = rawURL
                this.target = target
                this.expectedSHA256 = expectedSHA256
                this.appVersion = appVersion
                Files.write(target.toPath(), content.toByteArray(StandardCharsets.UTF_8))
            } catch (error: Exception) {
                throw IllegalStateException(error)
            }
        }
    }

    private class RecordingInstaller : AndroidUpdateInstallFlow.ApkInstaller {
        var calls = 0
        lateinit var apk: File

        override fun startInstall(context: Context, apk: File) {
            calls++
            this.apk = apk
        }
    }

    private class RecordingCleanup : AndroidUpdateInstallFlow.CleanupScheduler {
        var calls = 0
        lateinit var apk: File

        override fun schedule(apk: File) {
            calls++
            this.apk = apk
        }
    }
}
