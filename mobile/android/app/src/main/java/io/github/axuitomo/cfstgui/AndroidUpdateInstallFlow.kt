package io.github.axuitomo.cfstgui

import android.content.Context
import android.net.Uri
import com.getcapacitor.JSObject

object AndroidUpdateInstallFlow {
    fun interface UpdatePayloadReader {
        fun updateInstallPayload(appVersion: String?): JSObject
    }

    fun interface ApkDownloader {
        fun download(context: Context, rawURL: String?, fileName: String, expectedSHA256: String?, appVersion: String?): AndroidUpdateDownloads.DownloadedUpdatePackage
    }

    fun interface ApkInstaller {
        fun startInstall(context: Context, uri: Uri)
    }

    fun interface CleanupScheduler {
        fun schedule(context: Context, updatePackage: AndroidUpdateDownloads.DownloadedUpdatePackage)
    }

    @JvmStatic
    fun commandForDownloadAndInstall(context: Context, appVersion: String?): JSObject {
        return commandForDownloadAndInstall(
            context,
            appVersion,
            UpdatePayloadReader { version -> AndroidUpdateRelease.updateInstallPayload(version) },
            ApkDownloader { downloadContext, rawURL, fileName, expectedSHA256, version ->
                AndroidUpdateDownloads.downloadUpdatePackage(downloadContext, rawURL, fileName, expectedSHA256, version)
            },
            ApkInstaller { targetContext, uri -> AndroidUpdateInstaller.startInstall(targetContext, uri) },
            CleanupScheduler { cleanupContext, updatePackage -> AndroidUpdateInstaller.schedulePackageCleanup(cleanupContext, updatePackage) },
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
        val fileName = AndroidUpdateInstaller.safePackageFileName(assetName)
        val expectedSHA256 = info.getString("sha256", "").orEmpty()
        val updatePackage = downloader.download(context, info.getString("download_url", "").orEmpty(), fileName, expectedSHA256, appVersion)
        AndroidUpdateInstaller.recordDownloadedPackage(context, updatePackage)
        installer.startInstall(context, updatePackage.uri)
        cleanupScheduler.schedule(context, updatePackage)
        info.put("downloaded_path", updatePackage.displayPath)
        info.put("install_started", true)
        info.put("next_action", "android_install_confirmation")
        return AndroidPluginCommands.command("UPDATE_INSTALL_READY", info, "APK 已下载，请在系统安装确认中继续。", true)
    }
}
