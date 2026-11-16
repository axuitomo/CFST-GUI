package io.github.axuitomo.cfstgui

import org.json.JSONException
import org.junit.Assert.assertEquals
import org.junit.Assert.assertSame
import org.junit.Assert.assertThrows
import org.junit.Test

class CfstRuntimeInitializationTest {
    @Test
    fun acceptsSuccessfulGoInitialization() {
        initializeRuntimeChecked { """{"ok":true,"code":"MOBILE_INIT_OK"}""" }
    }

    @Test
    fun propagatesReportedFailureForPluginFallback() {
        val error = assertThrows(IllegalStateException::class.java) {
            initializeRuntimeChecked {
                """{"ok":false,"code":"MOBILE_INIT_FAILED","message":"directory unavailable"}"""
            }
        }
        assertEquals("directory unavailable", error.message)
    }

    @Test
    fun requiresBooleanSuccess() {
        for (response in listOf("""{"code":"MOBILE_INIT_OK"}""", """{"ok":"true"}""", """{"ok":1}""")) {
            assertThrows(IllegalStateException::class.java) {
                initializeRuntimeChecked { response }
            }
        }
    }

    @Test
    fun suppliesFallbackMessageForEmptyFailure() {
        val error = assertThrows(IllegalStateException::class.java) {
            initializeRuntimeChecked { """{"ok":false,"message":""}""" }
        }
        assertEquals("Android 运行时初始化失败。", error.message)
    }

    @Test
    fun propagatesMalformedGoResponse() {
        assertThrows(JSONException::class.java) {
            initializeRuntimeChecked { "not json" }
        }
    }

    @Test
    fun propagatesNativeInitializationException() {
        val failure = IllegalStateException("native initialization unavailable")
        val thrown = assertThrows(IllegalStateException::class.java) {
            initializeRuntimeChecked { throw failure }
        }
        assertSame(failure, thrown)
    }
}
