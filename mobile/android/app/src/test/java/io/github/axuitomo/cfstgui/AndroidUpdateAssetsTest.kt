package io.github.axuitomo.cfstgui

import org.json.JSONArray
import org.json.JSONObject
import org.junit.Assert.assertEquals
import org.junit.Test

class AndroidUpdateAssetsTest {
    @Test
    fun releaseAssetDownloadURLFindsNamedAsset() {
        val releaseAssets = JSONArray()
            .put(JSONObject().put("name", "cfst-gui-android-release.apk").put("browser_download_url", "https://example.test/universal.apk"))
            .put(JSONObject().put("name", "cfst-gui-android-arm64-v8a-release.apk").put("browser_download_url", "https://example.test/arm64.apk"))

        assertEquals(
            "https://example.test/arm64.apk",
            AndroidUpdateAssets.releaseAssetDownloadURL(releaseAssets, "cfst-gui-android-arm64-v8a-release.apk"),
        )
    }

    @Test
    fun selectManifestAssetFallsBackToUniversalAndroidAsset() {
        val manifestAssets = JSONArray()
            .put(JSONObject().put("platform", "linux").put("name", "cfst-gui-linux.tar.gz"))
            .put(JSONObject().put("platform", "android").put("abi", "universal").put("name", "cfst-gui-android-release.apk"))
        val releaseAssets = JSONArray()
            .put(JSONObject().put("name", "cfst-gui-android-release.apk").put("browser_download_url", "https://example.test/universal.apk"))

        val selected = requireNotNull(AndroidUpdateAssets.selectManifestAsset(manifestAssets, releaseAssets, arrayOf("x86_64")))

        assertEquals("cfst-gui-android-release.apk", selected.optString("name"))
    }

    @Test
    fun selectManifestAssetAcceptsManifestDownloadURLWithoutReleaseAsset() {
        val manifestAssets = JSONArray()
            .put(
                JSONObject()
                    .put("platform", "android")
                    .put("abi", "arm64-v8a")
                    .put("name", "cfst-gui-android-arm64-v8a-release.apk")
                    .put("download_url", "https://example.test/arm64.apk"),
            )

        val selected = requireNotNull(AndroidUpdateAssets.selectManifestAsset(manifestAssets, JSONArray(), arrayOf("arm64-v8a")))

        assertEquals("cfst-gui-android-arm64-v8a-release.apk", selected.optString("name"))
    }
}
