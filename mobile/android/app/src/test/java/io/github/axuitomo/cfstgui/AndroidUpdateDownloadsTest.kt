package io.github.axuitomo.cfstgui

import java.io.ByteArrayOutputStream
import java.io.File
import java.io.InputStream
import java.net.ServerSocket
import java.nio.charset.StandardCharsets
import java.nio.file.Files
import java.util.concurrent.CountDownLatch
import java.util.concurrent.TimeUnit
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertThrows
import org.junit.Assert.assertTrue
import org.junit.Test

class AndroidUpdateDownloadsTest {
    @Test
    fun createsGithubProxyCandidatesForGithubURLs() {
        val candidates = AndroidUpdateDownloads.githubDownloadCandidates(" https://github.com/axuitomo/CFST-GUI/releases/download/v1/app.apk ")

        assertEquals("https://ghproxy.vip/https://github.com/axuitomo/CFST-GUI/releases/download/v1/app.apk", candidates[0])
        assertEquals("https://gh.3w.pm/https://github.com/axuitomo/CFST-GUI/releases/download/v1/app.apk", candidates[1])
        assertEquals("https://gh.ddlc.top/https://github.com/axuitomo/CFST-GUI/releases/download/v1/app.apk", candidates[2])
        assertEquals("https://github.com/axuitomo/CFST-GUI/releases/download/v1/app.apk", candidates[3])
    }

    @Test
    fun keepsNonGithubURLAsSingleCandidate() {
        val candidates = AndroidUpdateDownloads.githubDownloadCandidates("https://example.test/app.apk")

        assertEquals(1, candidates.size)
        assertEquals("https://example.test/app.apk", candidates[0])
        assertTrue(AndroidUpdateDownloads.githubDownloadCandidates("").isEmpty())
    }

    @Test
    fun readsHTTPText() {
        LocalHttpServer.start(200, "ok-json").use { server ->
            assertEquals("ok-json", AndroidUpdateDownloads.readURL(server.url(), "1.8.2"))
            assertTrue(server.await())
        }
    }

    @Test
    fun downloadsHTTPBodyToTargetFile() {
        val root = Files.createTempDirectory("cfst-download-ok").toFile()
        try {
            LocalHttpServer.start(200, "apk-body").use { server ->
                val target = File(root, "app.apk")

                AndroidUpdateDownloads.downloadURLToFile(server.url(), target, "", "1.8.2")

                assertEquals("apk-body", String(Files.readAllBytes(target.toPath()), StandardCharsets.UTF_8))
                assertFalse(File(target.absolutePath + ".0.part").exists())
                assertTrue(server.await())
            }
        } finally {
            deleteRecursively(root)
        }
    }

    @Test
    fun deletesPartialDownloadWhenSHA256Fails() {
        val root = Files.createTempDirectory("cfst-download-sha").toFile()
        try {
            LocalHttpServer.start(200, "apk-body").use { server ->
                val target = File(root, "app.apk")

                assertThrows(IllegalStateException::class.java) {
                    AndroidUpdateDownloads.downloadURLToFile(server.url(), target, "deadbeef", "1.8.2")
                }

                assertFalse(target.exists())
                assertFalse(File(target.absolutePath + ".0.part").exists())
                assertTrue(server.await())
            }
        } finally {
            deleteRecursively(root)
        }
    }

    @Test
    fun joinsErrorMessages() {
        assertEquals(
            "first；second",
            AndroidUpdateDownloads.joinErrorMessages(listOf(IllegalStateException(" first "), IllegalArgumentException("second"))),
        )
    }

    private fun deleteRecursively(file: File?) {
        if (file == null || !file.exists()) {
            return
        }
        if (file.isDirectory) {
            file.listFiles()?.forEach { child -> deleteRecursively(child) }
        }
        Files.deleteIfExists(file.toPath())
    }

    private class LocalHttpServer private constructor(
        private val socket: ServerSocket,
        status: Int,
        body: String,
    ) : AutoCloseable {
        private val handled = CountDownLatch(1)
        private val thread = Thread({ handle(status, body) }, "cfst-test-http-server").apply {
            isDaemon = true
            start()
        }

        fun url(): String {
            return "http://127.0.0.1:${socket.localPort}/asset"
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
