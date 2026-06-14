package io.github.axuitomo.cfstgui

import android.app.DownloadManager
import android.content.Context
import android.database.Cursor
import android.net.Uri
import android.os.Environment
import android.os.SystemClock
import java.io.ByteArrayOutputStream
import java.io.InputStream
import java.io.OutputStream
import java.net.HttpURLConnection
import java.net.Proxy
import java.net.URI
import java.net.URL
import java.nio.charset.StandardCharsets

object AndroidUpdateDownloads {
    private val githubDownloadProxyPrefixes = arrayOf(
        "https://ghproxy.vip/",
        "https://gh.3w.pm/",
        "https://gh.ddlc.top/",
    )
    private const val updateMetadataTimeoutMs = 8000
    private const val updateDownloadTimeoutMs = 10 * 60 * 1000L
    private const val updateDownloadPollMs = 500L
    private const val APK_MIME_TYPE = "application/vnd.android.package-archive"

    data class DownloadedUpdatePackage(
        val downloadId: Long,
        val uri: Uri,
        val fileName: String,
        val displayPath: String,
    )

    fun interface CandidateDownloader {
        fun download(candidateURL: String, fileName: String, displayPath: String, appVersion: String?): DownloadedUpdatePackage
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
        val displayPath = AndroidUpdateInstaller.displayDownloadPath(fileName)
        return downloadUpdatePackage(
            rawURL,
            fileName,
            displayPath,
            expectedSHA256,
            appVersion,
            CandidateDownloader { candidateURL, candidateFileName, candidateDisplayPath, version ->
                downloadCandidateWithDownloadManager(context, candidateURL, candidateFileName, candidateDisplayPath, version)
            },
            DownloadVerifier { updatePackage, expected ->
                if (!expected?.trim().isNullOrEmpty()) {
                    AndroidUpdateIntegrity.verifySHA256(context.contentResolver, updatePackage.uri, expected)
                }
            },
            DownloadRemover { updatePackage ->
                removeDownload(context, updatePackage.downloadId)
            },
        )
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
        val errors = ArrayList<Exception>()
        for (candidate in candidates) {
            var updatePackage: DownloadedUpdatePackage? = null
            try {
                updatePackage = downloader.download(candidate, fileName, displayPath, appVersion)
                if (!expectedSHA256?.trim().isNullOrEmpty()) {
                    verifier.verify(updatePackage, expectedSHA256)
                }
                return updatePackage
            } catch (error: Exception) {
                if (updatePackage != null) {
                    remover.remove(updatePackage)
                }
                errors.add(error)
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

    @JvmStatic
    fun downloadRequest(candidateURL: String, fileName: String, appVersion: String?): DownloadManager.Request {
        return DownloadManager.Request(Uri.parse(candidateURL)).apply {
            setTitle(fileName)
            setDescription("CFST-GUI Android 更新包")
            setMimeType(APK_MIME_TYPE)
            setNotificationVisibility(DownloadManager.Request.VISIBILITY_VISIBLE_NOTIFY_COMPLETED)
            setAllowedOverMetered(true)
            setAllowedOverRoaming(true)
            addRequestHeader("User-Agent", userAgent(appVersion))
            setDestinationInExternalPublicDir(
                Environment.DIRECTORY_DOWNLOADS,
                AndroidUpdateInstaller.relativeDownloadPath(fileName),
            )
        }
    }

    private fun downloadCandidateWithDownloadManager(context: Context, candidateURL: String, fileName: String, displayPath: String, appVersion: String?): DownloadedUpdatePackage {
        val manager = context.getSystemService(Context.DOWNLOAD_SERVICE) as? DownloadManager
            ?: throw IllegalStateException("系统下载管理器不可用。")
        val downloadId = try {
            manager.enqueue(downloadRequest(candidateURL, fileName, appVersion))
        } catch (error: SecurityException) {
            throw IllegalStateException("系统下载管理器无法写入 Download/CFST-GUI，请检查系统下载组件或存储策略。", error)
        } catch (error: IllegalArgumentException) {
            throw IllegalStateException("更新包下载地址无效：$candidateURL", error)
        }
        try {
            val uri = waitForDownload(manager, downloadId)
            return DownloadedUpdatePackage(downloadId, uri, fileName, displayPath)
        } catch (error: Exception) {
            manager.remove(downloadId)
            throw error
        }
    }

    private fun waitForDownload(manager: DownloadManager, downloadId: Long): Uri {
        val deadline = SystemClock.elapsedRealtime() + updateDownloadTimeoutMs
        while (SystemClock.elapsedRealtime() < deadline) {
            val status = queryDownloadStatus(manager, downloadId)
            when (status.state) {
                DownloadState.SUCCESS -> {
                    return manager.getUriForDownloadedFile(downloadId)
                        ?: throw IllegalStateException("下载完成但系统未返回更新包 URI。")
                }
                DownloadState.FAILED -> throw IllegalStateException("下载管理器下载 APK 失败：" + status.reason)
                DownloadState.PENDING -> Thread.sleep(updateDownloadPollMs)
            }
        }
        throw IllegalStateException("下载 APK 超时。")
    }

    private fun queryDownloadStatus(manager: DownloadManager, downloadId: Long): DownloadStatus {
        val cursor = manager.query(DownloadManager.Query().setFilterById(downloadId)) ?: return DownloadStatus(DownloadState.PENDING, "等待系统下载管理器响应")
        cursor.use {
            if (!it.moveToFirst()) {
                return DownloadStatus(DownloadState.PENDING, "等待系统下载管理器创建任务")
            }
            return when (it.getIntColumn(DownloadManager.COLUMN_STATUS)) {
                DownloadManager.STATUS_SUCCESSFUL -> DownloadStatus(DownloadState.SUCCESS, "完成")
                DownloadManager.STATUS_FAILED -> DownloadStatus(DownloadState.FAILED, it.getIntColumn(DownloadManager.COLUMN_REASON).toString())
                else -> DownloadStatus(DownloadState.PENDING, "下载中")
            }
        }
    }

    private fun Cursor.getIntColumn(columnName: String): Int {
        return getInt(getColumnIndexOrThrow(columnName))
    }

    private enum class DownloadState {
        PENDING,
        SUCCESS,
        FAILED,
    }

    private data class DownloadStatus(val state: DownloadState, val reason: String)

    private fun removeDownload(context: Context, downloadId: Long): Int {
        val manager = context.getSystemService(Context.DOWNLOAD_SERVICE) as? DownloadManager ?: return 0
        return manager.remove(downloadId)
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
