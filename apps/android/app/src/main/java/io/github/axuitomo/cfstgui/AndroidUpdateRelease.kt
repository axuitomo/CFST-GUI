package io.github.axuitomo.cfstgui

import android.os.Build
import com.getcapacitor.JSObject
import org.json.JSONArray
import org.json.JSONObject

object AndroidUpdateRelease {
    const val RELEASE_PAGE_URL = "https://github.com/axuitomo/CFST-GUI/releases/latest"
    private const val LATEST_RELEASE_API = "https://api.github.com/repos/axuitomo/CFST-GUI/releases/latest"

    @JvmStatic
    fun checkForUpdatesPayload(currentVersion: String?): JSObject {
        return checkForUpdatesPayload(currentVersion, readLatestRelease(currentVersion))
    }

    @JvmStatic
    fun updateInstallPayload(currentVersion: String?): JSObject {
        val release = readLatestRelease(currentVersion)
        val data = checkForUpdatesPayload(currentVersion, release)
        if (data.getBoolean("update_available", false) != true) {
            return data
        }
        applyAndroidAsset(release, data, currentVersion)
        if (data.getString("download_url", "").orEmpty().trim().isEmpty()) {
            throw IllegalStateException("Android 更新资产缺少下载地址。")
        }
        return data
    }

    @JvmStatic
    fun checkForUpdatesPayload(currentVersion: String?, release: JSONObject): JSObject {
        val latestVersion = AndroidUpdateIntegrity.normalizeVersion(release.optString("tag_name", ""))
        check(latestVersion.isNotEmpty()) { "GitHub Release 缺少版本号（tag_name）。" }
        val normalizedCurrentVersion = currentVersion?.trim().orEmpty().ifEmpty { "1.0" }
        val data = JSObject()
        data.put("current_version", normalizedCurrentVersion)
        data.put("install_mode", "android_apk")
        data.put("platform", "android")
        data.put("latest_version", latestVersion)
        data.put("release_name", release.optString("name", ""))
        data.put("release_url", release.optString("html_url", RELEASE_PAGE_URL))
        data.put("update_available", AndroidUpdateIntegrity.compareVersions(latestVersion, normalizedCurrentVersion) > 0)
        data.put("asset_name", "")
        data.put("download_url", "")
        data.put("sha256", "")
        return data
    }

    @JvmStatic
    fun applyAndroidAsset(
        release: JSONObject,
        data: JSObject,
        currentVersion: String?,
        readMetadata: (String?, String?) -> String = { url, version -> AndroidUpdateDownloads.readURL(url, version) },
    ) {
        val assets = release.optJSONArray("assets") ?: throw IllegalStateException("GitHub Release 缺少 assets。")
        val manifestAsset = AndroidUpdateAssets.findReleaseAsset(assets, "cfst-gui-update-manifest.json")
        if (manifestAsset != null && applyManifestAsset(manifestAsset, assets, data, currentVersion, readMetadata)) {
            return
        }
        val arm64 = if (Build.SUPPORTED_ABIS.orEmpty().contains("arm64-v8a")) {
            AndroidUpdateAssets.findReleaseAsset(assets, "cfst-gui-android-arm64-v8a-release.apk")
        } else {
            null
        }
        val apk = arm64 ?: AndroidUpdateAssets.findReleaseAsset(assets, "cfst-gui-android-release.apk")
            ?: throw IllegalStateException("GitHub Release 缺少 Android APK 资产。")
        data.put("asset_name", apk.optString("name", "cfst-gui-android-release.apk"))
        data.put("download_url", apk.optString("browser_download_url", ""))
        data.put("sha256", officialAssetSHA256(apk))
        if (data.getString("download_url", "").orEmpty().trim().isEmpty()) {
            throw IllegalStateException("GitHub Release 的 Android APK 下载地址为空。")
        }
    }

    private fun readLatestRelease(currentVersion: String?): JSONObject {
        return JSONObject(AndroidUpdateDownloads.readURL(LATEST_RELEASE_API, currentVersion))
    }

    private fun applyManifestAsset(
        manifestAsset: JSONObject,
        releaseAssets: JSONArray,
        data: JSObject,
        currentVersion: String?,
        readMetadata: (String?, String?) -> String,
    ): Boolean {
        val manifest = JSONObject(readMetadata(manifestAsset.optString("browser_download_url", ""), currentVersion))
        val manifestAssets = manifest.optJSONArray("assets") ?: return false
        val selected = AndroidUpdateAssets.selectManifestAsset(manifestAssets, releaseAssets) ?: return false
        val name = selected.optString("name", "cfst-gui-android-release.apk")
        data.put("asset_name", name)
        data.put(
            "download_url",
            AndroidPayloads.firstNonEmpty(
                selected.optString("download_url", ""),
                AndroidUpdateAssets.releaseAssetDownloadURL(releaseAssets, name),
            ),
        )
        val hash = selected.optString("sha256", "").trim()
        data.put(
            "sha256",
            if (hash.isNotEmpty()) {
                AndroidUpdateIntegrity.requireSHA256(hash)
            } else {
                officialAssetSHA256(AndroidUpdateAssets.findReleaseAsset(releaseAssets, name))
            },
        )
        return true
    }

    private fun officialAssetSHA256(asset: JSONObject?): String {
        val digest = asset?.optString("digest", "").orEmpty().trim()
        check(digest.startsWith("sha256:", ignoreCase = true)) { "GitHub Release 缺少可信的 APK SHA256。" }
        return AndroidUpdateIntegrity.requireSHA256(digest.substringAfter(':'))
    }
}
