package io.github.axuitomo.cfstgui

import com.getcapacitor.JSObject
import org.json.JSONObject
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.RuntimeEnvironment
import org.robolectric.annotation.Config

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [35])
class AndroidPluginCommandsTest {
    @Test
    fun coloDictionaryCommandsUseDedicatedExecutor() {
        assertTrue(AndroidPluginCommands.usesDedicatedExecutor("colo.update"))
        assertTrue(AndroidPluginCommands.usesDedicatedExecutor("colo.process"))
        assertFalse(AndroidPluginCommands.usesDedicatedExecutor("config.save"))
        assertFalse(AndroidPluginCommands.usesDedicatedExecutor("colo.status"))
    }

    @Test
    fun commandUsesSharedSchemaEnvelope() {
        val data = JSObject()
        data.put("value", 42)

        val command = JSONObject(AndroidPluginCommands.commandJSON("TEST_OK", data, "done", true))

        assertEquals("TEST_OK", command.getString("code"))
        assertTrue(command.getBoolean("ok"))
        assertEquals("done", command.getString("message"))
        assertEquals("cfst-gui-command-v2", command.getString("schema_version"))
        assertTrue(command.isNull("task_id"))
        assertEquals(0, command.getJSONArray("warnings").length())
        assertEquals(42, command.getJSONObject("data").getInt("value"))
    }

    @Test
    fun finalizesServiceResponseWithStorageStatus() {
        val context = RuntimeEnvironment.getApplication()
        val response = "{\"ok\":true,\"data\":{\"answer\":42},\"warnings\":[]}"

        val command = JSONObject(AndroidPluginCommands.finalizeServiceResponse(context, response))
        val data = command.getJSONObject("data")
        val storage = data.getJSONObject("storage")

        assertEquals(42, data.getInt("answer"))
        assertEquals("private", storage.getString("backend"))
        assertTrue(storage.getBoolean("permission_ok"))
        assertEquals(0, command.getJSONArray("warnings").length())
    }

    @Test
    fun appendWarningAddsTopLevelAndDataWarnings() {
        val data = JSONObject()
        val command = JSONObject().put("data", data)

        AndroidPluginCommands.appendWarning(command, data, "careful")

        assertEquals("careful", command.getJSONArray("warnings").getString(0))
        assertEquals("careful", data.getJSONArray("warnings").getString(0))
    }

    @Test
    fun normalizeJSONStripsBomAndEscapesControlCharactersWithoutLosingText() {
        val normalized = AndroidPluginCommands.normalizeJSON("\uFEFF{\"message\":\"bad\u0000text\"}")

        assertTrue(normalized.contains("\\u0000"))
        assertEquals("bad\u0000text", JSONObject(normalized).getString("message"))
    }

    @Test
    fun normalizeJSONEscapesRawControlCharactersInsideStrings() {
        val normalized = AndroidPluginCommands.normalizeJSON("{\"message\":\"line1\nline2\tend\"}")

        assertEquals("{\"message\":\"line1\\u000Aline2\\u0009end\"}", normalized)
        assertEquals("line1\nline2\tend", JSONObject(normalized).getString("message"))
    }

    @Test
    fun normalizeJSONKeepsGoEscapedQuotesAndBackslashes() {
        val goPayload = "{\"code\":\"JSON_ENCODE_FAILED\",\"message\":\"bad \\\"quote\\\" and \\\\ backslash\"}"

        val normalized = AndroidPluginCommands.normalizeJSON(goPayload)

        assertEquals(goPayload, normalized)
        assertEquals("bad \"quote\" and \\ backslash", JSONObject(normalized).getString("message"))
    }

    @Test
    fun normalizeJSONKeepsStructuralWhitespace() {
        val raw = "{\n  \"message\": \"ok\"\n}"

        assertEquals(raw, AndroidPluginCommands.normalizeJSON(raw))
    }

    @Test
    fun finalizeServiceResponseKeepsEscapedErrorMessage() {
        val context = RuntimeEnvironment.getApplication()
        val response = "{\"ok\":false,\"code\":\"COMMAND_PAYLOAD_INVALID\"," +
            "\"data\":{\"answer\":42},\"message\":\"invalid character '\\\\n' in string literal\"," +
            "\"schema_version\":\"cfst-gui-command-v2\",\"task_id\":null,\"warnings\":[]}"

        val command = JSONObject(AndroidPluginCommands.finalizeServiceResponse(context, response))

        assertFalse(command.getBoolean("ok"))
        assertEquals("invalid character '\\n' in string literal", command.getString("message"))
    }
}
