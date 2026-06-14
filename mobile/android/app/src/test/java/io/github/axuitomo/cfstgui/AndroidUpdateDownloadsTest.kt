package io.github.axuitomo.cfstgui

import java.io.ByteArrayOutputStream
import java.io.InputStream
import java.net.ServerSocket
import java.nio.charset.StandardCharsets
import java.util.concurrent.CountDownLatch
import java.util.concurrent.TimeUnit
import android.net.Uri
import org.junit.Assert.assertEquals
import org.junit.Assert.assertThrows
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [35])
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
    fun downloadsFirstSuccessfulCandidatePackage() {
        val downloader = RecordingCandidateDownloader(failUntilCall = 1)
        val verifier = RecordingVerifier()
        val remover = RecordingRemover()

        val updatePackage = AndroidUpdateDownloads.downloadUpdatePackage(
            "https://github.com/axuitomo/CFST-GUI/releases/download/v1/app.apk",
            "app.apk",
            "Download/CFST-GUI/app.apk",
            "",
            "1.8.2",
            downloader,
            verifier,
            remover,
        )

        assertEquals(2, downloader.calls.size)
        assertEquals("https://ghproxy.vip/https://github.com/axuitomo/CFST-GUI/releases/download/v1/app.apk", downloader.calls[0])
        assertEquals("https://gh.3w.pm/https://github.com/axuitomo/CFST-GUI/releases/download/v1/app.apk", downloader.calls[1])
        assertEquals(101L, updatePackage.downloadId)
        assertEquals("app.apk", updatePackage.fileName)
        assertEquals("Download/CFST-GUI/app.apk", updatePackage.displayPath)
        assertTrue(remover.removedIds.isEmpty())
    }

    @Test
    fun removesDownloadedPackageWhenSHA256Fails() {
        val downloader = RecordingCandidateDownloader()
        val verifier = RecordingVerifier(fail = true)
        val remover = RecordingRemover()

        assertThrows(IllegalStateException::class.java) {
            AndroidUpdateDownloads.downloadUpdatePackage(
                "https://example.test/app.apk",
                "app.apk",
                "Download/CFST-GUI/app.apk",
                "deadbeef",
                "1.8.2",
                downloader,
                verifier,
                remover,
            )
        }

        assertEquals(1, downloader.calls.size)
        assertEquals(listOf(100L), verifier.verifiedIds)
        assertEquals(listOf(100L), remover.removedIds)
    }

    @Test
    fun removesFailedCandidatePackageBeforeTryingNextCandidate() {
        val downloader = RecordingCandidateDownloader()
        val verifier = RecordingVerifier(failingIds = setOf(100L))
        val remover = RecordingRemover()

        val updatePackage = AndroidUpdateDownloads.downloadUpdatePackage(
            "https://github.com/axuitomo/CFST-GUI/releases/download/v1/app.apk",
            "app.apk",
            "Download/CFST-GUI/app.apk",
            "deadbeef",
            "1.8.2",
            downloader,
            verifier,
            remover,
        )

        assertEquals(2, downloader.calls.size)
        assertEquals(101L, updatePackage.downloadId)
        assertEquals(listOf(100L, 101L), verifier.verifiedIds)
        assertEquals(listOf(100L), remover.removedIds)
    }

    @Test
    fun joinsErrorMessages() {
        assertEquals(
            "first；second",
            AndroidUpdateDownloads.joinErrorMessages(listOf(IllegalStateException(" first "), IllegalArgumentException("second"))),
        )
    }

    private class RecordingCandidateDownloader(
        private val failUntilCall: Int = 0,
    ) : AndroidUpdateDownloads.CandidateDownloader {
        val calls = mutableListOf<String>()

        override fun download(candidateURL: String, fileName: String, displayPath: String, appVersion: String?): AndroidUpdateDownloads.DownloadedUpdatePackage {
            calls.add(candidateURL)
            if (calls.size <= failUntilCall) {
                throw IllegalStateException("failed $candidateURL")
            }
            val downloadId = 99L + calls.size
            return AndroidUpdateDownloads.DownloadedUpdatePackage(
                downloadId,
                Uri.parse("content://downloads/my_downloads/$downloadId"),
                fileName,
                displayPath,
            )
        }
    }

    private class RecordingVerifier(
        private val fail: Boolean = false,
        private val failingIds: Set<Long> = emptySet(),
    ) : AndroidUpdateDownloads.DownloadVerifier {
        val verifiedIds = mutableListOf<Long>()

        override fun verify(updatePackage: AndroidUpdateDownloads.DownloadedUpdatePackage, expectedSHA256: String?) {
            verifiedIds.add(updatePackage.downloadId)
            if (fail || failingIds.contains(updatePackage.downloadId)) {
                throw IllegalStateException("SHA failed")
            }
        }
    }

    private class RecordingRemover : AndroidUpdateDownloads.DownloadRemover {
        val removedIds = mutableListOf<Long>()

        override fun remove(updatePackage: AndroidUpdateDownloads.DownloadedUpdatePackage) {
            removedIds.add(updatePackage.downloadId)
        }
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
