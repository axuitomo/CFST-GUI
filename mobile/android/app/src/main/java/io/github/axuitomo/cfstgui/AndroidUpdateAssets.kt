package io.github.axuitomo.cfstgui

import android.os.Build
import java.util.Locale
import org.json.JSONArray
import org.json.JSONObject

object AndroidUpdateAssets {
    @JvmStatic
    fun selectManifestAsset(manifestAssets: JSONArray, releaseAssets: JSONArray): JSONObject? {
        return selectManifestAsset(manifestAssets, releaseAssets, Build.SUPPORTED_ABIS ?: emptyArray())
    }

    @JvmStatic
    fun selectManifestAsset(manifestAssets: JSONArray, releaseAssets: JSONArray, supportedABIs: Array<String>): JSONObject? {
        for (abi in supportedABIs) {
            val matched = findManifestAssetByABI(manifestAssets, releaseAssets, abi)
            if (matched != null) {
                return matched
            }
        }
        var universal: JSONObject? = null
        for (index in 0 until manifestAssets.length()) {
            val asset = manifestAssets.optJSONObject(index)
            if (asset == null || !isAndroidManifestAsset(asset)) {
                continue
            }
            val abi = asset.optString("abi", "").trim()
            if (abi.isEmpty() || abi.equals("universal", ignoreCase = true)) {
                universal = asset
                break
            }
        }
        if (universal != null) {
            return universal
        }
        for (index in 0 until manifestAssets.length()) {
            val asset = manifestAssets.optJSONObject(index)
            if (asset != null && isAndroidManifestAsset(asset)) {
                return asset
            }
        }
        return null
    }

    @JvmStatic
    fun findReleaseAsset(assets: JSONArray, name: String): JSONObject? {
        for (index in 0 until assets.length()) {
            val asset = assets.optJSONObject(index)
            if (asset != null && name == asset.optString("name", "")) {
                return asset
            }
        }
        return null
    }

    @JvmStatic
    fun releaseAssetDownloadURL(assets: JSONArray, name: String): String =
        findReleaseAsset(assets, name)?.optString("browser_download_url", "").orEmpty()

    private fun findManifestAssetByABI(
        manifestAssets: JSONArray,
        releaseAssets: JSONArray,
        deviceABI: String?,
    ): JSONObject? {
        val normalizedABI = normalizeAndroidABI(deviceABI)
        if (normalizedABI.isEmpty()) {
            return null
        }
        for (index in 0 until manifestAssets.length()) {
            val asset = manifestAssets.optJSONObject(index)
            if (asset == null || !isAndroidManifestAsset(asset)) {
                continue
            }
            val manifestABI = normalizeAndroidABI(asset.optString("abi", ""))
            if (normalizedABI != manifestABI) {
                continue
            }
            val name = asset.optString("name", "")
            if ((name.isNotEmpty() && releaseAssetDownloadURL(releaseAssets, name).trim().isNotEmpty()) ||
                asset.optString("download_url", "").trim().isNotEmpty()
            ) {
                return asset
            }
        }
        return null
    }

    private fun isAndroidManifestAsset(asset: JSONObject): Boolean {
        val platform = asset.optString("platform", "")
        val goos = asset.optString("goos", "")
        return platform.equals("android", ignoreCase = true) || goos.equals("android", ignoreCase = true)
    }

    private fun normalizeAndroidABI(value: String?): String {
        return when (val normalized = value?.trim()?.lowercase(Locale.ROOT).orEmpty()) {
            "arm64", "arm64-v8a", "aarch64" -> "arm64-v8a"
            "arm", "armeabi", "armeabi-v7a", "armv7", "armv7a" -> "armeabi-v7a"
            "universal" -> "universal"
            else -> normalized
        }
    }
}
