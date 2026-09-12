package io.github.axuitomo.cfstgui

import android.net.LocalSocket
import android.net.LocalSocketAddress
import android.os.Process
import android.util.Log
import java.io.ByteArrayOutputStream
import java.io.EOFException
import java.io.IOException
import java.io.InputStream
import java.net.InetAddress
import java.net.ServerSocket
import java.net.Socket
import java.nio.charset.StandardCharsets
import java.util.concurrent.atomic.AtomicBoolean

object WebViewDevToolsRelay {
    const val PORT = 9223

    private const val TAG = "WebViewCDP"
    private const val MAX_HEADER_BYTES = 16 * 1024
    private const val HEADER_TIMEOUT_MILLIS = 10_000
    private val started = AtomicBoolean(false)
    private const val BACKLOG = 8
    private const val HEADER_TERMINATOR = 0x0D0A0D0A
    private const val BITS_PER_BYTE = 8

    fun start() {
        if (!started.compareAndSet(false, true)) {
            return
        }
        Thread(::serve, "webview-cdp-relay").apply {
            isDaemon = true
            start()
        }
    }

    internal fun stripDevToolsOrigin(request: ByteArray): ByteArray {
        return headerLines(request)
            .filterNot(::isAllowedDevToolsOrigin)
            .joinToString("\r\n")
            .toByteArray(StandardCharsets.ISO_8859_1)
    }

    internal fun isWebSocketUpgrade(request: ByteArray): Boolean {
        return headerLines(request).any { header ->
            header.substringBefore(':').trim().equals("Upgrade", ignoreCase = true) &&
                header.substringAfter(':', "").trim().equals("websocket", ignoreCase = true)
        }
    }

    private fun serve() {
        try {
            ServerSocket(PORT, BACKLOG, InetAddress.getByName("127.0.0.1")).use { server ->
                Log.i(TAG, "WebView CDP relay listening on 127.0.0.1:$PORT")
                while (true) {
                    relayAsync(server.accept())
                }
            }
        } catch (error: IOException) {
            started.set(false)
            Log.e(TAG, "Unable to start WebView CDP relay.", error)
        }
    }

    private fun relayAsync(client: Socket) {
        Thread({
            client.use { downstream ->
                try {
                    downstream.soTimeout = HEADER_TIMEOUT_MILLIS
                    forward(readHeaders(downstream.getInputStream()), downstream)
                } catch (error: IOException) {
                    Log.w(TAG, "WebView CDP relay connection failed.", error)
                }
            }
        }, "webview-cdp-client").apply {
            isDaemon = true
            start()
        }
    }

    private fun forward(request: ByteArray, downstream: Socket) {
        LocalSocket().use { upstream ->
            upstream.connect(
                LocalSocketAddress(
                    "webview_devtools_remote_${Process.myPid()}",
                    LocalSocketAddress.Namespace.ABSTRACT,
                ),
            )
            upstream.outputStream.apply {
                write(stripDevToolsOrigin(request))
                flush()
            }
            if (isWebSocketUpgrade(request)) {
                downstream.soTimeout = 0
                bridge(downstream, upstream)
            } else {
                upstream.inputStream.copyTo(downstream.getOutputStream())
                downstream.getOutputStream().flush()
            }
        }
    }

    private fun bridge(downstream: Socket, upstream: LocalSocket) {
        val downstreamToUpstream = Thread({
            try {
                downstream.getInputStream().copyTo(upstream.outputStream)
            } catch (_: IOException) {
                // The peer closing the DevTools connection is expected.
            } finally {
                runCatching { upstream.shutdownOutput() }
            }
        }, "webview-cdp-upstream").apply {
            isDaemon = true
            start()
        }
        try {
            upstream.inputStream.copyTo(downstream.getOutputStream())
            downstream.getOutputStream().flush()
        } finally {
            runCatching { downstream.shutdownOutput() }
            downstreamToUpstream.join()
        }
    }

    internal fun readHeaders(input: InputStream): ByteArray {
        val output = ByteArrayOutputStream()
        var window = 0
        while (output.size() < MAX_HEADER_BYTES) {
            val current = input.read()
            if (current < 0) {
                throw EOFException("CDP client closed before sending request headers.")
            }
            output.write(current)
            window = (window shl BITS_PER_BYTE) or current
            if (window == HEADER_TERMINATOR) {
                return output.toByteArray()
            }
        }
        throw IOException("CDP request headers exceed $MAX_HEADER_BYTES bytes.")
    }

    private fun headerLines(request: ByteArray): List<String> {
        return request.toString(StandardCharsets.ISO_8859_1).split("\r\n")
    }

    private fun isAllowedDevToolsOrigin(header: String): Boolean {
        if (!header.substringBefore(':').trim().equals("Origin", ignoreCase = true)) {
            return false
        }
        return when (header.substringAfter(':', "").trim()) {
            "devtools://devtools", "chrome-devtools://" -> true
            else -> false
        }
    }
}
