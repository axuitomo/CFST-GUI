package io.github.axuitomo.cfstgui

import android.net.Uri
import java.io.ByteArrayInputStream
import java.nio.charset.StandardCharsets
import java.nio.file.Files
import java.nio.file.Path
import org.json.JSONObject
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RuntimeEnvironment
import org.robolectric.RobolectricTestRunner
import org.robolectric.Shadows
import org.robolectric.annotation.Config

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [35])
class AndroidTaskBridgeTest {
    @Test
    fun cancelProbeFinalizesStorageState() {
        val action = RecordingAction("{\"ok\":true,\"code\":\"PROBE_CANCELLED\",\"data\":{\"task_id\":\"t1\"},\"warnings\":[]}")

        val command = JSONObject(AndroidTaskBridge.cancelProbe(RuntimeEnvironment.getApplication(), "{\"task_id\":\"t1\"}", action))

        assertEquals("{\"task_id\":\"t1\"}", action.payload)
        assertEquals("PROBE_CANCELLED", command.getString("code"))
        assertEquals("t1", command.getJSONObject("data").getString("task_id"))
        assertEquals("private", command.getJSONObject("data").getJSONObject("storage").getString("backend"))
    }

    @Test
    fun resumeProbeFinalizesStorageState() {
        val action = RecordingAction("{\"ok\":true,\"code\":\"PROBE_RESUMED\",\"data\":{\"task_id\":\"t2\"},\"warnings\":[]}")

        val command = JSONObject(AndroidTaskBridge.resumeProbe(RuntimeEnvironment.getApplication(), "{\"task_id\":\"t2\"}", action))

        assertEquals("PROBE_RESUMED", command.getString("code"))
        assertEquals("t2", command.getJSONObject("data").getString("task_id"))
        assertEquals("private", command.getJSONObject("data").getJSONObject("storage").getString("backend"))
    }

    @Test
    fun listResultFileCopiesContentUriIntoPrivateFileBeforeCallingService() {
        val context = RuntimeEnvironment.getApplication()
        val source = Uri.parse("content://example.test/result.csv")
        Shadows.shadowOf(context.contentResolver).registerInputStream(
            source,
            ByteArrayInputStream("ip,ms".toByteArray(StandardCharsets.UTF_8)),
        )
        val action = RecordingAction("{\"ok\":true,\"code\":\"RESULT_FILE_READY\",\"data\":{\"rows\":1},\"warnings\":[]}")

        val command = JSONObject(AndroidTaskBridge.listResultFile(context, "{\"path\":\"content://example.test/result.csv\"}", action))
        val payload = JSONObject(action.payload)

        assertEquals("RESULT_FILE_READY", command.getString("code"))
        assertTrue(payload.getString("path").contains("/result-files/"))
        assertEquals("content://example.test/result.csv", payload.getString("source_uri"))
        assertEquals("content://example.test/result.csv", payload.getString("sourceUri"))
        assertFalse(payload.has("export_path"))
        assertEquals("ip,ms", String(Files.readAllBytes(Path.of(payload.getString("path"))), StandardCharsets.UTF_8))
        assertEquals("private", command.getJSONObject("data").getJSONObject("storage").getString("backend"))
    }

    private class RecordingAction(private val response: String) : AndroidTaskBridge.PayloadAction {
        var payload = ""

        override fun call(payloadJSON: String): String {
            payload = payloadJSON
            return response
        }
    }
}
