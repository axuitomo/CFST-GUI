package io.github.axuitomo.cfstgui

import com.getcapacitor.JSObject
import org.json.JSONObject
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class AndroidRuntimeStatusTest {
    @Test
    fun payloadIncludesSnapshotRuntimeAndBatteryStatus() {
        val battery = JSObject()
        battery.put("ignoring_battery_optimizations", true)

        val payload = AndroidRuntimeStatus.payloadFromSnapshots(
            "{\"ok\":true,\"data\":{\"task_id\":\"task-1\",\"session_state\":\"paused_runtime\",\"runtime_attached\":true,\"resume_capable\":true}}",
            "{\"ok\":true,\"data\":{\"worker\":\"ready\"}}",
            true,
            battery,
        )
        val data = JSONObject(payload.toString())

        assertTrue(data.getBoolean("foreground_service_running"))
        assertTrue(data.getBoolean("has_task_snapshot"))
        assertTrue(data.getBoolean("runtime_attached"))
        assertTrue(data.getBoolean("resume_capable"))
        assertEquals("paused_runtime", data.getString("session_state"))
        assertEquals("task-1", data.getString("task_id"))
        assertEquals("task-1", data.getJSONObject("task_snapshot").getString("task_id"))
        assertEquals("ready", data.getJSONObject("runtime").getString("worker"))
        assertTrue(data.getJSONObject("battery").getBoolean("ignoring_battery_optimizations"))
    }

    @Test
    fun payloadUsesEmptyTaskFieldsWhenSnapshotCommandFailed() {
        val payload = AndroidRuntimeStatus.payloadFromSnapshots(
            "{\"ok\":false,\"data\":{\"task_id\":\"ignored\"}}",
            "{\"ok\":true}",
            false,
            JSObject(),
        )
        val data = JSONObject(payload.toString())

        assertFalse(data.getBoolean("foreground_service_running"))
        assertFalse(data.getBoolean("has_task_snapshot"))
        assertFalse(data.getBoolean("runtime_attached"))
        assertFalse(data.getBoolean("resume_capable"))
        assertEquals("", data.getString("session_state"))
        assertEquals("", data.getString("task_id"))
        assertFalse(data.has("task_snapshot"))
        assertFalse(data.has("runtime"))
    }
}
