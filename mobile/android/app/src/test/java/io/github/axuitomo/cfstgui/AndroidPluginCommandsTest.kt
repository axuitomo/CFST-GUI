package io.github.axuitomo.cfstgui

import com.getcapacitor.JSObject
import org.json.JSONObject
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RuntimeEnvironment
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [35])
class AndroidPluginCommandsTest {
    @Test
    fun commandKeepsMobileSchemaEnvelope() {
        val data = JSObject()
        data.put("value", 42)

        val command = JSONObject(AndroidPluginCommands.commandJSON("TEST_OK", data, "done", true))

        assertEquals("TEST_OK", command.getString("code"))
        assertTrue(command.getBoolean("ok"))
        assertEquals("done", command.getString("message"))
        assertEquals("cfst-gui-mobile-v1", command.getString("schema_version"))
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
}
