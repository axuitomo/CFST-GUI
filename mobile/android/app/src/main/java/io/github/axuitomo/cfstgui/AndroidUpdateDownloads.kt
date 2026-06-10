package io.github.axuitomo.cfstgui

import java.io.ByteArrayOutputStream
import java.io.File
import java.io.FileOutputStream
import java.io.InputStream
import java.io.OutputStream
import java.net.HttpURLConnection
import java.net.URI
import java.net.URL
import java.nio.charset.StandardCharsets
import java.util.concurrent.ExecutionException
import java.util.concurrent.ExecutorCompletionService
import java.util.concurrent.Executors
import java.util.concurrent.Future
import java.util.concurrent.Callable
import java.util.concurrent.atomic.AtomicBoolean

object AndroidUpdateDownloads {
    private val githubDownloadProxyPrefixes = arrayOf(
        "https://ghproxy.vip/",
        "https://gh.3w.pm/",
        "https://gh.ddlc.top/",
    )
    private const val updateMetadataTimeoutMs = 8000
    private const val updateDownloadTimeoutMs = 8000

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
                connection = URL(candidate).openConnection() as HttpURLConnection
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
    fun downloadURLToFile(rawURL: String?, target: File, expectedSHA256: String?, appVersion: String?) {
        val candidates = githubDownloadCandidates(rawURL)
        if (candidates.isEmpty()) {
            throw IllegalStateException("下载 APK 缺少有效地址。")
        }
        val downloadExecutor = Executors.newFixedThreadPool(candidates.size)
        val completion = ExecutorCompletionService<File>(downloadExecutor)
        val futures = ArrayList<Future<File>>()
        val winnerSelected = AtomicBoolean(false)
        for (index in candidates.indices) {
            val candidate = candidates[index]
            val part = File(target.absolutePath + ".$index.part")
            futures.add(
                completion.submit(Callable {
                    downloadCandidateURLToFile(candidate, part, expectedSHA256, appVersion)
                    if (!winnerSelected.compareAndSet(false, true)) {
                        part.delete()
                        throw IllegalStateException("已有更快的更新源完成下载。")
                    }
                    part
                }),
            )
        }
        val errors = ArrayList<Exception>()
        try {
            for (index in candidates.indices) {
                try {
                    val part = completion.take().get()
                    for (future in futures) {
                        future.cancel(true)
                    }
                    if (target.exists() && !target.delete()) {
                        throw IllegalStateException("删除旧更新包失败：" + target.absolutePath)
                    }
                    if (!part.renameTo(target)) {
                        throw IllegalStateException("保存更新包失败：" + target.absolutePath)
                    }
                    return
                } catch (error: ExecutionException) {
                    val cause = error.cause
                    errors.add(if (cause is Exception) cause else Exception(cause))
                }
            }
        } finally {
            downloadExecutor.shutdownNow()
            for (index in candidates.indices) {
                val part = File(target.absolutePath + ".$index.part")
                if (part.exists()) {
                    part.delete()
                }
            }
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

    private fun downloadCandidateURLToFile(candidate: String, target: File, expectedSHA256: String?, appVersion: String?) {
        var connection: HttpURLConnection? = null
        try {
            connection = URL(candidate).openConnection() as HttpURLConnection
            connection.connectTimeout = updateDownloadTimeoutMs
            connection.readTimeout = updateDownloadTimeoutMs
            connection.setRequestProperty("User-Agent", userAgent(appVersion))
            val status = connection.responseCode
            if (status !in 200..299) {
                throw IllegalStateException("下载 APK 返回 HTTP $status ($candidate)")
            }
            connection.inputStream.use { input ->
                FileOutputStream(target).use { output ->
                    copy(input, output)
                }
            }
            if (!expectedSHA256?.trim().isNullOrEmpty()) {
                AndroidUpdateIntegrity.verifySHA256(target, expectedSHA256)
            }
        } catch (error: Exception) {
            if (target.exists()) {
                target.delete()
            }
            throw error
        } finally {
            connection?.disconnect()
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

    private fun copy(input: InputStream, output: OutputStream) {
        val buffer = ByteArray(8192)
        while (true) {
            val read = input.read(buffer)
            if (read < 0) {
                return
            }
            if (read > 0) {
                output.write(buffer, 0, read)
            }
        }
    }
}
