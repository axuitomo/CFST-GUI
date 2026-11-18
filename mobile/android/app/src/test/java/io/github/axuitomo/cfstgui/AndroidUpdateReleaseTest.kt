package io.github.axuitomo.cfstgui

import android.os.Build
import com.getcapacitor.JSObject
import org.json.JSONArray
import org.json.JSONObject
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertThrows
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.util.ReflectionHelpers

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [35])
class AndroidUpdateReleaseTest {
    private val hash = "a".repeat(64)

    @Test
    fun checkPayloadMarksNewerReleaseAvailable() {
        val release = JSONObject().put("tag_name", "v1.9.0").put("name", "CFST GUI 1.9.0")
        val payload = AndroidUpdateRelease.checkForUpdatesPayload("1.8.2", release)
        assertEquals("1.8.2", payload.getString("current_version"))
        assertEquals("1.9.0", payload.getString("latest_version"))
        assertEquals("android_apk", payload.getString("install_mode"))
        assertEquals("android", payload.getString("platform"))
        assertTrue(payload.getBoolean("update_available", false) == true)
        assertEquals("", payload.getString("asset_name"))
        assertEquals("", payload.getString("download_url"))
    }

    @Test
    fun checkPayloadKeepsCurrentReleaseUnavailable() {
        val payload = AndroidUpdateRelease.checkForUpdatesPayload("1.8.2", JSONObject().put("tag_name", "v1.8.2"))
        assertFalse(payload.getBoolean("update_available", true) == true)
        assertEquals(AndroidUpdateRelease.RELEASE_PAGE_URL, payload.getString("release_url"))
    }

    @Test
    fun checkPayloadRejectsMissingReleaseTag() {
        val error = assertThrows(IllegalStateException::class.java) {
            AndroidUpdateRelease.checkForUpdatesPayload("1.8.2", JSONObject())
        }
        assertEquals("GitHub Release 缺少版本号（tag_name）。", error.message)
    }

    @Test
    fun applyAndroidAssetFallsBackToOfficialUniversalApkDigest() {
        val name = "cfst-gui-android-release.apk"
        val payload = JSObject()
        AndroidUpdateRelease.applyAndroidAsset(releaseAsset(name, "sha256:" + hash.uppercase()), payload, "1.8.2")
        assertEquals(name, payload.getString("asset_name"))
        assertEquals(hash, payload.getString("sha256"))
    }

    @Test
    fun applyAndroidAssetSupportsCurrentArm64ReleaseWithoutManifest() {
        ReflectionHelpers.setStaticField(Build::class.java, "SUPPORTED_ABIS", arrayOf("arm64-v8a"))
        val name = "cfst-gui-android-arm64-v8a-release.apk"
        val payload = JSObject()
        AndroidUpdateRelease.applyAndroidAsset(releaseAsset(name, "sha256:$hash"), payload, "1.8.2")
        assertEquals(name, payload.getString("asset_name"))
        assertEquals(hash, payload.getString("sha256"))
    }

    @Test
    fun applyAndroidAssetRejectsAbsentOrInvalidOfficialDigest() {
        for (digest in listOf("", "sha512:$hash", "sha256:short", "sha256:" + "z".repeat(64))) {
            assertThrows(IllegalStateException::class.java) {
                AndroidUpdateRelease.applyAndroidAsset(
                    releaseAsset("cfst-gui-android-release.apk", digest), JSObject(), "1.8.2",
                )
            }
        }
    }

    @Test
    fun manifestHashIsRequiredOrUsesMatchingOfficialApkDigest() {
        val name = "cfst-gui-android-release.apk"
        val release = releaseAsset(name, "sha256:$hash")
        release.getJSONArray("assets").put(
            JSONObject().put("name", "cfst-gui-update-manifest.json")
                .put("browser_download_url", "https://github.com/axuitomo/CFST-GUI/releases/download/v1/manifest.json"),
        )
        for (manifestHash in listOf(hash, "", "invalid")) {
            val manifest = JSONObject().put(
                "assets",
                JSONArray().put(
                    JSONObject().put("platform", "android").put("abi", "universal")
                        .put("name", name).put("sha256", manifestHash),
                ),
            )
            val payload = JSObject()
            val apply = {
                AndroidUpdateRelease.applyAndroidAsset(release, payload, "1.8.2") { url, _ ->
                    assertTrue(AndroidUpdateDownloads.isOfficialMetadataURL(requireNotNull(url)))
                    manifest.toString()
                }
            }
            if (manifestHash == "invalid") {
                assertThrows(IllegalStateException::class.java) { apply() }
            } else {
                apply()
                assertEquals(name, payload.getString("asset_name"))
                assertEquals(hash, payload.getString("sha256"))
            }
        }
    }

    @Test
    fun applyAndroidAssetRejectsMissingApk() {
        val error = assertThrows(IllegalStateException::class.java) {
            AndroidUpdateRelease.applyAndroidAsset(JSONObject().put("assets", JSONArray()), JSObject(), "1.8.2")
        }
        assertEquals("GitHub Release 缺少 Android APK 资产。", error.message)
    }

    private fun releaseAsset(name: String, digest: String): JSONObject = JSONObject().put(
        "assets",
        JSONArray().put(
            JSONObject().put("name", name)
                .put("browser_download_url", "https://github.com/axuitomo/CFST-GUI/releases/download/v1/$name")
                .put("digest", digest),
        ),
    )
}
