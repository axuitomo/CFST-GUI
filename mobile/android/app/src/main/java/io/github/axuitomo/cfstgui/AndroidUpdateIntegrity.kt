package io.github.axuitomo.cfstgui

import android.content.ContentResolver
import android.net.Uri
import java.io.File
import java.io.FileInputStream
import java.io.InputStream
import java.security.MessageDigest
import java.util.Locale

object AndroidUpdateIntegrity {
    @JvmStatic
    fun normalizeVersion(value: String?): String {
        var normalized = value?.trim().orEmpty()
        if (normalized.startsWith("v") || normalized.startsWith("V")) {
            normalized = normalized.substring(1)
        }
        return normalized
    }

    @JvmStatic
    fun compareVersions(left: String?, right: String?): Int {
        val leftParts = versionParts(left)
        val rightParts = versionParts(right)
        val count = maxOf(leftParts.size, rightParts.size)
        for (index in 0 until count) {
            val leftPart = leftParts.getOrElse(index) { 0 }
            val rightPart = rightParts.getOrElse(index) { 0 }
            if (leftPart > rightPart) {
                return 1
            }
            if (leftPart < rightPart) {
                return -1
            }
        }
        return 0
    }

    @JvmStatic
    fun verifySHA256(file: File, expected: String?) {
        verifySHA256(FileInputStream(file), expected)
    }

    @JvmStatic
    fun verifySHA256(contentResolver: ContentResolver, uri: Uri, expected: String?) {
        val input = contentResolver.openInputStream(uri)
            ?: throw IllegalStateException("无法读取下载完成的更新包：$uri")
        verifySHA256(input, expected)
    }

    @JvmStatic
    fun verifySHA256(input: InputStream, expected: String?) {
        val digest = MessageDigest.getInstance("SHA-256")
        input.use { body ->
            val buffer = ByteArray(8192)
            while (true) {
                val read = body.read(buffer)
                if (read < 0) {
                    break
                }
                if (read > 0) {
                    digest.update(buffer, 0, read)
                }
            }
        }
        val actual = bytesToHex(digest.digest())
        if (!actual.equals(expected?.trim().orEmpty(), ignoreCase = true)) {
            throw IllegalStateException("SHA256 校验失败：期望 $expected，实际 $actual")
        }
    }

    @JvmStatic
    fun bytesToHex(bytes: ByteArray): String {
        val builder = StringBuilder(bytes.size * 2)
        for (byte in bytes) {
            builder.append(String.format(Locale.ROOT, "%02x", byte.toInt() and 0xff))
        }
        return builder.toString()
    }

    private fun versionParts(value: String?): IntArray {
        val normalized = normalizeVersion(value).split("[-+]".toRegex())[0]
        val parts = normalized.split(".")
        val result = IntArray(parts.size)
        for (index in parts.indices) {
            val digits = parts[index].replace("[^0-9].*$".toRegex(), "")
            result[index] = if (digits.isEmpty()) 0 else digits.toInt()
        }
        return result
    }
}
