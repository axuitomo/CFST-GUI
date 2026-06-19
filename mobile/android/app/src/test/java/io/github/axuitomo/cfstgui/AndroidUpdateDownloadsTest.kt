package io.github.axuitomo.cfstgui

import android.net.Uri
import java.io.ByteArrayOutputStream
import java.io.File
import java.io.InputStream
import java.net.ServerSocket
import java.nio.charset.StandardCharsets
import java.security.MessageDigest
import java.util.Collections
import java.util.concurrent.CountDownLatch
import java.util.concurrent.TimeUnit
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertThrows
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.RuntimeEnvironment
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
    fun fastestVerifiedGithubCandidateWins() {
        val rawURL = "https://github.com/axuitomo/CFST-GUI/releases/download/v1/app.apk"
        val candidates = AndroidUpdateDownloads.githubDownloadCandidates(rawURL)
        val downloader = RacingCandidateDownloader(
            expectedStarts = candidates.size,
            delays = mapOf(
                candidates[0] to 250L,
                candidates[1] to 40L,
                candidates[2] to 220L,
                candidates[3] to 200L,
            ),
        )
        val verifier = RecordingVerifier()
        val remover = RecordingRemover()

        val updatePackage = AndroidUpdateDownloads.downloadUpdatePackage(
            rawURL,
            "app.apk",
            "应用内更新/app.apk",
            "abc123",
            "1.8.2",
            downloader,
            verifier,
            remover,
        )

        assertEquals(candidates.toSet(), downloader.calls.toSet())
        assertEquals(downloader.fileFor(candidates[1]), updatePackage.file)
        assertEquals(listOf(downloader.fileFor(candidates[1])), verifier.verifiedFiles)
        assertTrue(remover.removedFiles.isEmpty())
    }

    @Test
    fun sha256FailedCandidateIsRemovedWhileOtherCandidatesContinue() {
        val rawURL = "https://github.com/axuitomo/CFST-GUI/releases/download/v1/app.apk"
        val candidates = AndroidUpdateDownloads.githubDownloadCandidates(rawURL)
        val downloader = RacingCandidateDownloader(
            expectedStarts = candidates.size,
            delays = mapOf(
                candidates[0] to 30L,
                candidates[1] to 80L,
                candidates[2] to 220L,
                candidates[3] to 200L,
            ),
        )
        val verifier = RecordingVerifier(failingFiles = setOf(downloader.fileFor(candidates[0])))
        val remover = RecordingRemover()

        val updatePackage = AndroidUpdateDownloads.downloadUpdatePackage(
            rawURL,
            "app.apk",
            "应用内更新/app.apk",
            "abc123",
            "1.8.2",
            downloader,
            verifier,
            remover,
        )

        assertEquals(downloader.fileFor(candidates[1]), updatePackage.file)
        assertTrue(verifier.verifiedFiles.contains(downloader.fileFor(candidates[0])))
        assertTrue(verifier.verifiedFiles.contains(downloader.fileFor(candidates[1])))
        assertEquals(listOf(downloader.fileFor(candidates[0])), remover.removedFiles)
    }

    @Test
    fun allCandidateFailuresAreAggregatedAndRemoved() {
        val rawURL = "https://github.com/axuitomo/CFST-GUI/releases/download/v1/app.apk"
        val candidates = AndroidUpdateDownloads.githubDownloadCandidates(rawURL)
        val downloader = RacingCandidateDownloader(expectedStarts = candidates.size)
        val verifier = RecordingVerifier(failingFiles = candidates.map { downloader.fileFor(it) }.toSet())
        val remover = RecordingRemover()

        val error = assertThrows(IllegalStateException::class.java) {
            AndroidUpdateDownloads.downloadUpdatePackage(
                rawURL,
                "app.apk",
                "应用内更新/app.apk",
                "deadbeef",
                "1.8.2",
                downloader,
                verifier,
                remover,
            )
        }

        assertTrue(error.message.orEmpty().contains("下载 APK 失败"))
        assertEquals(candidates.map { downloader.fileFor(it) }.toSet(), remover.removedFiles.toSet())
    }

    @Test
    fun nonGithubURLDownloadsOnceIntoPrivateUpdateDirectory() {
        val context = RuntimeEnvironment.getApplication()
        val body = "apk-body"
        val expectedSHA256 = sha256(body)

        LocalHttpServer.start(200, body).use { server ->
            val updatePackage = AndroidUpdateDownloads.downloadUpdatePackage(
                context,
                server.url(),
                "cfst-gui-android-release.apk",
                expectedSHA256,
                "1.8.2",
            )

            assertTrue(server.await())
            assertEquals(File(context.filesDir, "update_downloads/cfst-gui-android-release.apk"), updatePackage.file)
            assertEquals("content", updatePackage.uri.scheme)
            assertEquals("应用内更新/cfst-gui-android-release.apk", updatePackage.displayPath)
            assertFalse(AndroidUpdateInstaller.updateDownloadDirectory(context).listFiles().orEmpty().any { it.name.endsWith(".part") })
        }
    }

    @Test
    fun failedPrivateDownloadRemovesFinalAndPartFiles() {
        val context = RuntimeEnvironment.getApplication()

        LocalHttpServer.start(200, "wrong-body").use { server ->
            assertThrows(IllegalStateException::class.java) {
                AndroidUpdateDownloads.downloadUpdatePackage(
                    context,
                    server.url(),
                    "cfst-gui-android-release.apk",
                    "deadbeef",
                    "1.8.2",
                )
            }

            assertTrue(server.await())
            val files = AndroidUpdateInstaller.updateDownloadDirectory(context).listFiles().orEmpty()
            assertTrue(files.isEmpty())
        }
    }

    @Test
    fun joinsErrorMessages() {
        assertEquals(
            "first；second",
            AndroidUpdateDownloads.joinErrorMessages(listOf(IllegalStateException(" first "), IllegalArgumentException("second"))),
        )
    }

    private class RacingCandidateDownloader(
        private val expectedStarts: Int = 0,
        private val delays: Map<String, Long> = emptyMap(),
        private val failingURLs: Set<String> = emptySet(),
    ) : AndroidUpdateDownloads.CandidateDownloader {
        val calls = Collections.synchronizedList(mutableListOf<String>())
        private val started = CountDownLatch(expectedStarts)

        override fun download(candidateURL: String, fileName: String, displayPath: String, appVersion: String?): AndroidUpdateDownloads.DownloadedUpdatePackage {
            calls.add(candidateURL)
            if (expectedStarts > 0) {
                started.countDown()
                if (!started.await(5, TimeUnit.SECONDS)) {
                    throw IllegalStateException("candidates did not start together")
                }
            }
            Thread.sleep(delays[candidateURL] ?: 0L)
            if (failingURLs.contains(candidateURL)) {
                throw IllegalStateException("failed $candidateURL")
            }
            val file = fileFor(candidateURL)
            return AndroidUpdateDownloads.DownloadedUpdatePackage(
                file,
                Uri.fromFile(file),
                fileName,
                displayPath,
            )
        }

        fun fileFor(candidateURL: String): File {
            return File("build/tmp/android-update-test/${candidateURL.hashCode()}.part")
        }
    }

    private class RecordingVerifier(
        private val failingFiles: Set<File> = emptySet(),
    ) : AndroidUpdateDownloads.DownloadVerifier {
        val verifiedFiles = Collections.synchronizedList(mutableListOf<File>())

        override fun verify(updatePackage: AndroidUpdateDownloads.DownloadedUpdatePackage, expectedSHA256: String?) {
            verifiedFiles.add(updatePackage.file)
            if (failingFiles.contains(updatePackage.file)) {
                throw IllegalStateException("SHA failed")
            }
        }
    }

    private class RecordingRemover : AndroidUpdateDownloads.DownloadRemover {
        val removedFiles = Collections.synchronizedList(mutableListOf<File>())

        override fun remove(updatePackage: AndroidUpdateDownloads.DownloadedUpdatePackage) {
            removedFiles.add(updatePackage.file)
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

    private companion object {
        fun sha256(value: String): String {
            val digest = MessageDigest.getInstance("SHA-256").digest(value.toByteArray(StandardCharsets.UTF_8))
            return AndroidUpdateIntegrity.bytesToHex(digest)
        }
    }
}
