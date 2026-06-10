package io.github.axuitomo.cfstgui

import android.content.Context
import com.getcapacitor.JSObject
import java.io.File

object AndroidUpdateInstallFlow {
    fun interface UpdatePayloadReader {
        fun updateInstallPayload(appVersion: String?): JSObject
    }

    fun interface ApkDownloader {
        fun download(rawURL: String?, target: File, expectedSHA256: String?, appVersion: String?)
    }

    fun interface ApkInstaller {
        fun startInstall(context: Context, apk: File)
    }

    fun interface CleanupScheduler {
        fun schedule(apk: File)
    }

    @JvmStatic
    fun commandForDownloadAndInstall(context: Context, appVersion: String?): JSObject {
        return commandForDownloadAndInstall(
            context,
            appVersion,
            UpdatePayloadReader { version -> AndroidUpdateRelease.updateInstallPayload(version) },
            ApkDownloader { rawURL, target, expectedSHA256, version ->
                AndroidUpdateDownloads.downloadURLToFile(rawURL, target, expectedSHA256, version)
            },
            ApkInstaller { targetContext, apk -> AndroidUpdateInstaller.startInstall(targetContext, apk) },
            CleanupScheduler { apk -> AndroidUpdateInstaller.schedulePackageCleanup(apk) },
        )
    }

    @JvmStatic
    fun commandForDownloadAndInstall(
        context: Context,
        appVersion: String?,
        payloadReader: UpdatePayloadReader,
        downloader: ApkDownloader,
        installer: ApkInstaller,
        cleanupScheduler: CleanupScheduler,
    ): JSObject {
        val info = payloadReader.updateInstallPayload(appVersion)
        if (info.getBoolean("update_available", false) != true) {
            return AndroidPluginCommands.command("UPDATE_NOT_AVAILABLE", info, "当前已是最新版本。", true)
        }

        val assetName = info.getString("asset_name", "cfst-gui-android-release.apk").orEmpty()
            .ifEmpty { "cfst-gui-android-release.apk" }
        val updateDir = AndroidUpdateInstaller.ensureUpdateDirectory(context)
        val apk = File(updateDir, assetName)
        val expectedSHA256 = info.getString("sha256", "").orEmpty()
        downloader.download(info.getString("download_url", "").orEmpty(), apk, expectedSHA256, appVersion)
        if (expectedSHA256.isNotEmpty()) {
            AndroidUpdateIntegrity.verifySHA256(apk, expectedSHA256)
        }
        installer.startInstall(context, apk)
        cleanupScheduler.schedule(apk)
        info.put("downloaded_path", apk.absolutePath)
        info.put("install_started", true)
        info.put("next_action", "android_install_confirmation")
        return AndroidPluginCommands.command("UPDATE_INSTALL_READY", info, "APK 已下载，请在系统安装确认中继续。", true)
    }
}
