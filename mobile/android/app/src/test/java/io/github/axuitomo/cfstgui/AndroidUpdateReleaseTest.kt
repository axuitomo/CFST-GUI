package io.github.axuitomo.cfstgui

import java.io.ByteArrayOutputStream
import java.io.InputStream
import java.net.ServerSocket
import java.nio.charset.StandardCharsets
import java.util.concurrent.CountDownLatch
import java.util.concurrent.TimeUnit
import org.json.JSONArray
import org.json.JSONObject
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertThrows
import org.junit.Assert.assertTrue
import org.junit.Test

class AndroidUpdateReleaseTest {
    @Test
    fun checkPayloadMarksNewerReleaseAvailable() {
        val release = JSONObject()
            .put("tag_name", "v1.9.0")
            .put("name", "CFST GUI 1.9.0")
            .put("html_url", "https://example.test/release")

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
        val release = JSONObject().put("tag_name", "v1.8.2")

        val payload = AndroidUpdateRelease.checkForUpdatesPayload("1.8.2", release)

        assertFalse(payload.getBoolean("update_available", true) == true)
        assertEquals(AndroidUpdateRelease.RELEASE_PAGE_URL, payload.getString("release_url"))
    }

    @Test
    fun applyAndroidAssetFallsBackToUniversalApk() {
        val release = JSONObject().put(
            "assets",
            JSONArray()
                .put(
                    JSONObject()
                        .put("name", "cfst-gui-android-release.apk")
                        .put("browser_download_url", "https://example.test/cfst-gui-android-release.apk"),
                ),
        )
        val payload = AndroidUpdateRelease.checkForUpdatesPayload("1.8.2", JSONObject().put("tag_name", "v1.9.0"))

        AndroidUpdateRelease.applyAndroidAsset(release, payload, "1.8.2")

        assertEquals("cfst-gui-android-release.apk", payload.getString("asset_name"))
        assertEquals("https://example.test/cfst-gui-android-release.apk", payload.getString("download_url"))
        assertEquals("", payload.getString("sha256"))
    }

    @Test
    fun applyAndroidAssetPrefersManifestSelection() {
        val manifest = JSONObject().put(
            "assets",
            JSONArray()
                .put(
                    JSONObject()
                        .put("platform", "android")
                        .put("abi", "universal")
                        .put("name", "cfst-gui-android-universal-release.apk")
                        .put("sha256", "abc123"),
                ),
        )
        LocalHttpServer.start(200, manifest.toString()).use { server ->
            val release = JSONObject().put(
                "assets",
                JSONArray()
                    .put(
                        JSONObject()
                            .put("name", "cfst-gui-update-manifest.json")
                            .put("browser_download_url", server.url()),
                    )
                    .put(
                        JSONObject()
                            .put("name", "cfst-gui-android-release.apk")
                            .put("browser_download_url", "https://example.test/fallback.apk"),
                    )
                    .put(
                        JSONObject()
                            .put("name", "cfst-gui-android-universal-release.apk")
                            .put("browser_download_url", "https://example.test/universal.apk"),
                    ),
            )
            val payload = AndroidUpdateRelease.checkForUpdatesPayload("1.8.2", JSONObject().put("tag_name", "v1.9.0"))

            AndroidUpdateRelease.applyAndroidAsset(release, payload, "1.8.2")

            assertEquals("cfst-gui-android-universal-release.apk", payload.getString("asset_name"))
            assertEquals("https://example.test/universal.apk", payload.getString("download_url"))
            assertEquals("abc123", payload.getString("sha256"))
            assertTrue(server.await())
        }
    }

    @Test
    fun applyAndroidAssetRejectsMissingApk() {
        val release = JSONObject().put("assets", JSONArray())
        val payload = com.getcapacitor.JSObject()

        val error = assertThrows(IllegalStateException::class.java) {
            AndroidUpdateRelease.applyAndroidAsset(release, payload, "1.8.2")
        }

        assertEquals("GitHub Release 缺少 Android APK 资产。", error.message)
    }

    private class LocalHttpServer private constructor(
        private val socket: ServerSocket,
        status: Int,
        body: String,
    ) : AutoCloseable {
        private val handled = CountDownLatch(1)
        private val thread = Thread({ handle(status, body) }, "cfst-test-update-release-server").apply {
            isDaemon = true
            start()
        }

        fun url(): String {
            return "http://127.0.0.1:${socket.localPort}/manifest"
        }

        fun await(): Boolean {
            return handled.await(5, TimeUnit.SECONDS)
        }

        override fun close() {
            socket.close()
            thread.join(1000)
        }

        private fun handle(status: Int, body: String) {
            try {
                socket.accept().use { client ->
                    readRequest(client.getInputStream())
                    val content = body.toByteArray(StandardCharsets.UTF_8)
                    val reason = if (status in 200..299) "OK" else "Error"
                    val headers = "HTTP/1.1 $status $reason\r\n" +
                        "Content-Length: ${content.size}\r\n" +
                        "Connection: close\r\n" +
                        "\r\n"
                    val output = client.getOutputStream()
                    output.write(headers.toByteArray(StandardCharsets.UTF_8))
                    output.write(content)
                    output.flush()
                }
            } catch (_: Exception) {
                // Test assertions cover whether the server handled the expected request.
            } finally {
                handled.countDown()
            }
        }

        companion object {
            fun start(status: Int, body: String): LocalHttpServer {
                return LocalHttpServer(ServerSocket(0), status, body)
            }

            private fun readRequest(input: InputStream) {
                val buffer = ByteArrayOutputStream()
                while (true) {
                    val value = input.read()
                    if (value < 0) {
                        return
                    }
                    buffer.write(value)
                    val text = buffer.toString(StandardCharsets.ISO_8859_1.name())
                    if (text.endsWith("\r\n\r\n")) {
                        return
                    }
                }
            }
        }
    }
}
