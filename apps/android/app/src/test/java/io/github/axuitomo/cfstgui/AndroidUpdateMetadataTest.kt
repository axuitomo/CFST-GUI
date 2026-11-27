package io.github.axuitomo.cfstgui

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertThrows
import org.junit.Assert.assertTrue
import org.junit.Test
import java.io.IOException
import java.io.InputStream
import java.net.HttpURLConnection
import java.net.URL

class AndroidUpdateMetadataTest {
    private val url = "https://github.com/axuitomo/CFST-GUI/releases/download/v1/manifest.json"

    @Test
    fun redirectsAreBoundedAndErrorsDoNotTryProxies() {
        for (status in listOf(302, 500)) {
            val calls = mutableListOf<String>()
            assertThrows(IllegalStateException::class.java) {
                AndroidUpdateDownloads.readURL(url, "fixture") { target ->
                    calls.add(target.toString())
                    FixtureConnection(target, "".byteInputStream(), status, url)
                }
            }
            assertEquals(if (status == 302) 6 else 1, calls.size)
            assertTrue(calls.all { it == url })
        }
    }

    @Test
    fun oversizedMetadataIsRejectedAndClosed() {
        val connection = FixtureConnection(URL(url), ByteArray(2 * 1024 * 1024 + 1).inputStream())
        assertThrows(IllegalStateException::class.java) {
            AndroidUpdateDownloads.readURL(url, "fixture") { connection }
        }
        assertTrue(connection.disconnected)
        assertFalse(connection.instanceFollowRedirects)
    }

    @Test
    fun cancellationIsCheckedBetweenMetadataReads() {
        val input = object : InputStream() {
            override fun read(): Int {
                Thread.currentThread().interrupt()
                return 1
            }

            override fun read(buffer: ByteArray, offset: Int, length: Int): Int = read()
        }
        val connection = FixtureConnection(URL(url), input)
        try {
            assertThrows(InterruptedException::class.java) {
                AndroidUpdateDownloads.readURL(url, "fixture") { connection }
            }
        } finally {
            Thread.interrupted()
        }
        assertTrue(connection.disconnected)
    }

    @Test(timeout = 12000)
    fun continuousSlowMetadataCannotExtendOverallDeadline() {
        lateinit var connection: FixtureConnection
        val input = object : InputStream() {
            override fun read(): Int {
                Thread.sleep(25)
                if (connection.disconnected) throw IOException("fixture disconnected at deadline")
                return 1
            }

            override fun read(buffer: ByteArray, offset: Int, length: Int): Int = read()
        }
        connection = FixtureConnection(URL(url), input)
        assertThrows(Exception::class.java) {
            AndroidUpdateDownloads.readURL(url, "fixture") { connection }
        }
        assertTrue(connection.disconnected)
    }

    private class FixtureConnection(
        url: URL,
        private val body: InputStream,
        private val status: Int = 200,
        private val location: String? = null,
    ) : HttpURLConnection(url) {
        @Volatile
        var disconnected = false

        override fun connect() = Unit

        override fun disconnect() {
            disconnected = true
        }

        override fun usingProxy() = false

        override fun getResponseCode(): Int = status

        override fun getHeaderField(name: String): String? = if (name == "Location") location else null

        override fun getInputStream(): InputStream = body
    }
}
