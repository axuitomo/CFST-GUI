package io.github.axuitomo.cfstgui

import android.content.Context
import android.net.Uri
import java.io.ByteArrayOutputStream
import java.io.File
import java.io.FileOutputStream
import java.io.InputStream
import java.io.OutputStream
import java.net.HttpURLConnection
import java.net.Proxy
import java.net.URI
import java.net.URL
import java.nio.charset.StandardCharsets
import java.nio.file.AtomicMoveNotSupportedException
import java.nio.file.Files
import java.nio.file.StandardCopyOption
import java.util.Collections
import java.util.UUID
import java.util.concurrent.CancellationException
import java.util.concurrent.Callable
import java.util.concurrent.ConcurrentHashMap
import java.util.concurrent.ExecutionException
import java.util.concurrent.ExecutorCompletionService
import java.util.concurrent.ExecutorService
import java.util.concurrent.Executors
import java.util.concurrent.Future
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicBoolean

object AndroidUpdateDownloads {
    private val githubDownloadProxyPrefixes = arrayOf(
        "https://ghproxy.vip/",
        "https://gh.3w.pm/",
        "https://gh.ddlc.top/",
    )
    private const val updateMetadataTimeoutMs = 8000
    private const val updateDownloadTimeoutMs = 10 * 60 * 1000L
    private const val APK_MIME_TYPE = "application/vnd.android.package-archive"

    data class DownloadedUpdatePackage(
        val file: File,
        val uri: Uri,
        val fileName: String,
        val displayPath: String,
    )

    fun interface CandidateDownloader {
        fun download(candidateURL: String, fileName: String, displayPath: String, appVersion: String?): DownloadedUpdatePackage
    }

    private interface CancellableCandidateDownloader : CandidateDownloader {
        fun cancelActiveDownloads()
    }

    fun interface DownloadVerifier {
        fun verify(updatePackage: DownloadedUpdatePackage, expectedSHA256: String?)
    }

    fun interface DownloadRemover {
        fun remove(updatePackage: DownloadedUpdatePackage)
    }

    @JvmStatic
    fun githubDownloadCandidates(rawURL: String?): List<String> {
        val value = rawURL?.trim().orEmpty()
        if (value.isEmpty()) {
            return ArrayList()
        }
        return try {
            val uri = URI(value)
            val host = uri.host
            if (host == null || !host.equals("github.com", ignoreCase = true)) {
                return arrayListOf(value)
            }
            val candidates = ArrayList<String>()
            for (prefix in githubDownloadProxyPrefixes) {
                candidates.add(prefix + value)
            }
            candidates.add(value)
            uniqueURLs(candidates)
        } catch (_: Exception) {
            arrayListOf(value)
        }
    }

    @JvmStatic
    fun readURL(rawURL: String?, appVersion: String?): String {
        val candidates = githubDownloadCandidates(rawURL)
        if (candidates.isEmpty()) {
            throw IllegalStateException("缺少有效读取地址。")
        }
        var lastError: Exception? = null
        for (candidate in candidates) {
            var connection: HttpURLConnection? = null
            try {
                connection = URL(candidate).openConnection(Proxy.NO_PROXY) as HttpURLConnection
                connection.connectTimeout = updateMetadataTimeoutMs
                connection.readTimeout = updateMetadataTimeoutMs
                connection.setRequestProperty("Accept", "application/vnd.github+json")
                connection.setRequestProperty("User-Agent", userAgent(appVersion))
                val status = connection.responseCode
                val input = if (status in 200..299) connection.inputStream else connection.errorStream
                input.use { body ->
                    ByteArrayOutputStream().use { output ->
                        if (body != null) {
                            copy(body, output)
                        }
                        val text = output.toString(StandardCharsets.UTF_8.name())
                        if (status !in 200..299) {
                            throw IllegalStateException("HTTP $status：$text ($candidate)")
                        }
                        return text
                    }
                }
            } catch (error: Exception) {
                lastError = error
            } finally {
                connection?.disconnect()
            }
        }
        throw lastError ?: IllegalStateException("读取远程内容失败。")
    }

    @JvmStatic
    fun downloadUpdatePackage(context: Context, rawURL: String?, fileName: String, expectedSHA256: String?, appVersion: String?): DownloadedUpdatePackage {
        val safeFileName = AndroidUpdateInstaller.safePackageFileName(fileName)
        val displayPath = AndroidUpdateInstaller.displayDownloadPath(safeFileName)
        val finalFile = AndroidUpdateInstaller.updatePackageFile(context, safeFileName)
        AndroidUpdateInstaller.prepareUpdateDirectory(context)
        AndroidUpdateInstaller.deleteUpdatePartFiles(context)
        val downloader = HttpCandidateDownloader(context, downloadDeadlineNanos())
        try {
            val downloadedPart = downloadUpdatePackage(
                rawURL,
                safeFileName,
                displayPath,
                expectedSHA256,
                appVersion,
                downloader,
                DownloadVerifier { updatePackage, expected ->
                    if (!expected?.trim().isNullOrEmpty()) {
                        AndroidUpdateIntegrity.verifySHA256(updatePackage.file, expected)
                    }
                },
                DownloadRemover { updatePackage ->
                    deleteFile(updatePackage.file)
                },
            )
            moveReplacing(downloadedPart.file, finalFile)
            val uri = AndroidUpdateInstaller.contentUriForFile(context, finalFile)
            return DownloadedUpdatePackage(finalFile, uri, safeFileName, displayPath)
        } catch (error: Exception) {
            deleteFile(finalFile)
            throw error
        } finally {
            AndroidUpdateInstaller.deleteUpdatePartFiles(context)
        }
    }

    @JvmStatic
    fun downloadUpdatePackage(
        rawURL: String?,
        fileName: String,
        displayPath: String,
        expectedSHA256: String?,
        appVersion: String?,
        downloader: CandidateDownloader,
        verifier: DownloadVerifier,
        remover: DownloadRemover,
    ): DownloadedUpdatePackage {
        val candidates = githubDownloadCandidates(rawURL)
        if (candidates.isEmpty()) {
            throw IllegalStateException("下载 APK 缺少有效地址。")
        }
        if (candidates.size == 1) {
            return downloadAndVerifyCandidate(candidates[0], fileName, displayPath, expectedSHA256, appVersion, downloader, verifier, remover)
        }
        val executor = Executors.newFixedThreadPool(candidates.size)
        val completion = ExecutorCompletionService<DownloadedUpdatePackage>(executor)
        val futures = ArrayList<Future<DownloadedUpdatePackage>>()
        val winnerClaimed = AtomicBoolean(false)
        for (candidate in candidates) {
            futures.add(completion.submit(Callable {
                downloadAndVerifyCandidate(candidate, fileName, displayPath, expectedSHA256, appVersion, downloader, verifier, remover, winnerClaimed)
            }))
        }
        val errors = ArrayList<Exception>()
        var winnerPackage: DownloadedUpdatePackage? = null
        var interruptedError: InterruptedException? = null
        try {
            var completed = 0
            while (completed < candidates.size && winnerPackage == null && interruptedError == null) {
                try {
                    winnerPackage = completion.take().get()
                    cancelRemaining(futures, downloader)
                } catch (error: InterruptedException) {
                    Thread.currentThread().interrupt()
                    interruptedError = error
                    cancelRemaining(futures, downloader)
                } catch (_: CancellationException) {
                    // Another candidate already won.
                } catch (error: ExecutionException) {
                    val cause = error.cause
                    errors.add(if (cause is Exception) cause else error)
                }
                completed++
            }
        } finally {
            executor.shutdownNow()
            awaitDownloadTasks(executor)
        }
        if (interruptedError != null) {
            throw IllegalStateException("下载 APK 已取消。", interruptedError)
        }
        if (winnerPackage != null) {
            return winnerPackage
        }
        throw if (errors.isEmpty()) {
            IllegalStateException("下载 APK 失败。")
        } else {
            IllegalStateException("下载 APK 失败：" + joinErrorMessages(errors))
        }
    }

    @JvmStatic
    fun joinErrorMessages(errors: List<Exception>?): String {
        if (errors == null) {
            return ""
        }
        val builder = StringBuilder()
        for (error in errors) {
            val message = error.message?.trim().orEmpty()
            if (message.isEmpty()) {
                continue
            }
            if (builder.isNotEmpty()) {
                builder.append("；")
            }
            builder.append(message)
        }
        return builder.toString()
    }

    @JvmStatic
    fun partFileName(fileName: String): String = "$fileName.${UUID.randomUUID()}.part"

    private class HttpCandidateDownloader(
        private val context: Context,
        private val deadlineNanos: Long,
    ) : CancellableCandidateDownloader {
        private val activeConnections = Collections.newSetFromMap(ConcurrentHashMap<HttpURLConnection, Boolean>())

        override fun download(candidateURL: String, fileName: String, displayPath: String, appVersion: String?): DownloadedUpdatePackage {
            return downloadCandidateToPartFile(context, candidateURL, fileName, displayPath, appVersion, activeConnections, deadlineNanos)
        }

        override fun cancelActiveDownloads() {
            for (connection in activeConnections) {
                connection.disconnect()
            }
        }
    }

    private fun downloadAndVerifyCandidate(
        candidateURL: String,
        fileName: String,
        displayPath: String,
        expectedSHA256: String?,
        appVersion: String?,
        downloader: CandidateDownloader,
        verifier: DownloadVerifier,
        remover: DownloadRemover,
        winnerClaimed: AtomicBoolean? = null,
    ): DownloadedUpdatePackage {
        var updatePackage: DownloadedUpdatePackage? = null
        try {
            updatePackage = downloader.download(candidateURL, fileName, displayPath, appVersion)
            if (!expectedSHA256?.trim().isNullOrEmpty()) {
                verifier.verify(updatePackage, expectedSHA256)
            }
            if (winnerClaimed != null && !winnerClaimed.compareAndSet(false, true)) {
                remover.remove(updatePackage)
                updatePackage = null
                throw CancellationException("另一个更新下载候选已胜出。")
            }
            return updatePackage
        } catch (error: Exception) {
            if (updatePackage != null) {
                remover.remove(updatePackage)
            }
            throw error
        }
    }

    private fun downloadCandidateToPartFile(
        context: Context,
        candidateURL: String,
        fileName: String,
        displayPath: String,
        appVersion: String?,
        activeConnections: MutableSet<HttpURLConnection>,
        deadlineNanos: Long,
    ): DownloadedUpdatePackage {
        val updateDir = AndroidUpdateInstaller.prepareUpdateDirectory(context)
        val partFile = File(updateDir, partFileName(fileName))
        var connection: HttpURLConnection? = null
        val timeoutExecutor = Executors.newSingleThreadScheduledExecutor()
        var timeoutTask: Future<*>? = null
        try {
            throwIfInterrupted()
            throwIfDeadlineExpired(deadlineNanos)
            connection = URL(candidateURL).openConnection(Proxy.NO_PROXY) as HttpURLConnection
            val remainingTimeoutMs = remainingDeadlineMillis(deadlineNanos)
            connection.connectTimeout = remainingTimeoutMs
            connection.readTimeout = remainingTimeoutMs
            connection.setRequestProperty("Accept", APK_MIME_TYPE)
            connection.setRequestProperty("User-Agent", userAgent(appVersion))
            activeConnections.add(connection)
            timeoutTask = timeoutExecutor.schedule({ connection.disconnect() }, remainingTimeoutMs.toLong(), TimeUnit.MILLISECONDS)
            val status = connection.responseCode
            throwIfDeadlineExpired(deadlineNanos)
            if (status !in 200..299) {
                val text = connection.errorStream.useText()
                throw IllegalStateException("HTTP $status：$text ($candidateURL)")
            }
            connection.inputStream.use { input ->
                FileOutputStream(partFile).use { output ->
                    copyWithDeadline(input, output, deadlineNanos)
                }
            }
            throwIfInterrupted()
            throwIfDeadlineExpired(deadlineNanos)
            return DownloadedUpdatePackage(partFile, Uri.fromFile(partFile), fileName, displayPath)
        } catch (error: Exception) {
            deleteFile(partFile)
            if (isDeadlineExpired(deadlineNanos)) {
                throw IllegalStateException("下载 APK 超时。", error)
            }
            throw error
        } finally {
            timeoutTask?.cancel(true)
            timeoutExecutor.shutdownNow()
            if (connection != null) {
                activeConnections.remove(connection)
                connection.disconnect()
            }
        }
    }

    private fun awaitDownloadTasks(executor: ExecutorService) {
        try {
            executor.awaitTermination(5, TimeUnit.SECONDS)
        } catch (error: InterruptedException) {
            Thread.currentThread().interrupt()
        }
    }

    private fun cancelRemaining(futures: List<Future<DownloadedUpdatePackage>>, downloader: CandidateDownloader) {
        if (downloader is CancellableCandidateDownloader) {
            downloader.cancelActiveDownloads()
        }
        for (future in futures) {
            if (!future.isDone) {
                future.cancel(true)
            }
        }
    }

    private fun moveReplacing(source: File, target: File) {
        target.parentFile?.mkdirs()
        try {
            Files.move(source.toPath(), target.toPath(), StandardCopyOption.REPLACE_EXISTING, StandardCopyOption.ATOMIC_MOVE)
        } catch (_: AtomicMoveNotSupportedException) {
            Files.move(source.toPath(), target.toPath(), StandardCopyOption.REPLACE_EXISTING)
        }
    }

    private fun deleteFile(file: File): Int {
        return if (file.isFile && file.delete()) 1 else 0
    }

    private fun InputStream?.useText(): String {
        if (this == null) {
            return ""
        }
        return use { input ->
            ByteArrayOutputStream().use { output ->
                copy(input, output)
                output.toString(StandardCharsets.UTF_8.name())
            }
        }
    }

    private fun uniqueURLs(values: List<String>): List<String> {
        val result = ArrayList<String>()
        for (value in values) {
            val normalized = value.trim()
            if (normalized.isEmpty() || result.contains(normalized)) {
                continue
            }
            result.add(normalized)
        }
        return result
    }

    private fun userAgent(appVersion: String?): String = "CFST-GUI/" + appVersion.orEmpty().ifEmpty { "1.0" }

    private fun downloadDeadlineNanos(): Long = System.nanoTime() + TimeUnit.MILLISECONDS.toNanos(updateDownloadTimeoutMs)

    private fun remainingDeadlineMillis(deadlineNanos: Long): Int {
        val remainingNanos = deadlineNanos - System.nanoTime()
        if (remainingNanos <= 0L) {
            throw IllegalStateException("下载 APK 超时。")
        }
        val remainingMillis = TimeUnit.NANOSECONDS.toMillis(remainingNanos).coerceAtLeast(1L)
        return remainingMillis.coerceAtMost(Int.MAX_VALUE.toLong()).toInt()
    }

    private fun isDeadlineExpired(deadlineNanos: Long): Boolean = System.nanoTime() >= deadlineNanos

    private fun throwIfDeadlineExpired(deadlineNanos: Long) {
        if (isDeadlineExpired(deadlineNanos)) {
            throw IllegalStateException("下载 APK 超时。")
        }
    }

    private fun copy(input: InputStream, output: OutputStream) {
        val buffer = ByteArray(8192)
        while (true) {
            throwIfInterrupted()
            val read = input.read(buffer)
            if (read < 0) {
                return
            }
            if (read > 0) {
                output.write(buffer, 0, read)
            }
        }
    }

    private fun copyWithDeadline(input: InputStream, output: OutputStream, deadlineNanos: Long) {
        val buffer = ByteArray(8192)
        while (true) {
            throwIfInterrupted()
            throwIfDeadlineExpired(deadlineNanos)
            val read = input.read(buffer)
            if (read < 0) {
                return
            }
            if (read > 0) {
                output.write(buffer, 0, read)
            }
        }
    }

    private fun throwIfInterrupted() {
        if (Thread.currentThread().isInterrupted) {
            throw InterruptedException("下载 APK 已取消。")
        }
    }
}
