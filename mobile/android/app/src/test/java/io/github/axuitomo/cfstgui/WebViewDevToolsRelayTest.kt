package io.github.axuitomo.cfstgui

import java.io.ByteArrayInputStream
import java.io.EOFException
import java.nio.charset.StandardCharsets
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class WebViewDevToolsRelayTest {
    @Test
    fun stripsOnlySupportedDevToolsOriginsFromWebSocketRequests() {
        for (origin in listOf("devtools://devtools", "chrome-devtools://")) {
            val request = webSocketRequest(origin)
            val relayed = WebViewDevToolsRelay.stripDevToolsOrigin(request).toString(StandardCharsets.ISO_8859_1)

            assertFalse(relayed.contains("Origin: $origin"))
            assertTrue(relayed.contains("Sec-WebSocket-Key: test-key"))
            assertTrue(WebViewDevToolsRelay.isWebSocketUpgrade(request))
        }
    }

    @Test
    fun preservesOtherOrigins() {
        val request = webSocketRequest("https://untrusted.example")
        val relayed = WebViewDevToolsRelay.stripDevToolsOrigin(request).toString(StandardCharsets.ISO_8859_1)

        assertTrue(relayed.contains("Origin: https://untrusted.example"))
    }

    @Test
    fun readsRequestHeadersUntilTerminatingBlankLine() {
        val stream = ByteArrayInputStream(
            (
                "GET /devtools/page/target HTTP/1.1\r\n" +
                    "Host: 127.0.0.1\r\n" +
                    "\r\n" +
                    "ignored-body"
            ).toByteArray(StandardCharsets.ISO_8859_1),
        )

        val headers = WebViewDevToolsRelay.readHeaders(stream).toString(StandardCharsets.ISO_8859_1)

        assertEquals("GET /devtools/page/target HTTP/1.1\r\nHost: 127.0.0.1\r\n\r\n", headers)
    }

    @Test(expected = EOFException::class)
    fun rejectsTruncatedRequestHeaders() {
        val stream = ByteArrayInputStream("GET / HTTP/1.1\r\n".toByteArray(StandardCharsets.ISO_8859_1))

        WebViewDevToolsRelay.readHeaders(stream)
    }

    private fun webSocketRequest(origin: String): ByteArray {
        return (
            "GET /devtools/page/target HTTP/1.1\r\n" +
                "Host: 127.0.0.1\r\n" +
                "Origin: $origin\r\n" +
                "Upgrade: websocket\r\n" +
                "Sec-WebSocket-Key: test-key\r\n\r\n"
        ).toByteArray(StandardCharsets.ISO_8859_1)
    }
}
