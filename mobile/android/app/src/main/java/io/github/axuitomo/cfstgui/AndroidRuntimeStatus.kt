package io.github.axuitomo.cfstgui

import com.getcapacitor.JSObject
import mobileapi.Service
import org.json.JSONObject

object AndroidRuntimeStatus {
    @JvmStatic
    fun payload(service: Service, foregroundServiceRunning: Boolean, battery: JSObject, keepAlive: JSObject? = null): JSObject {
        return payloadFromSnapshots(
            service.loadTaskSnapshot("{}"),
            service.loadTaskSnapshot("{\"runtime_status_only\":true}"),
            foregroundServiceRunning,
            battery,
            keepAlive,
        )
    }

    @JvmStatic
    fun payloadFromSnapshots(
        snapshotJSON: String,
        runtimeJSON: String,
        foregroundServiceRunning: Boolean,
        battery: JSObject,
        keepAlive: JSObject? = null,
    ): JSObject {
        val data = JSObject()
        val snapshotCommand = JSONObject(snapshotJSON)
        val snapshot = if (snapshotCommand.optBoolean("ok", false)) {
            snapshotCommand.optJSONObject("data")
        } else {
            null
        }
        val hasTaskSnapshot = snapshot != null
        val taskId = snapshot?.optString("task_id", "").orEmpty()
        val sessionState = snapshot?.optString("session_state", "").orEmpty()
        val runtimeAttached = snapshot?.optBoolean("runtime_attached", false) ?: false
        val resumeCapable = snapshot?.optBoolean("resume_capable", false) ?: false

        data.put("foreground_service_running", foregroundServiceRunning)
        data.put("has_task_snapshot", hasTaskSnapshot)
        data.put("resume_capable", resumeCapable)
        data.put("runtime_attached", runtimeAttached)
        data.put("session_state", sessionState)
        data.put("task_id", taskId)
        if (snapshot != null) {
            data.put("task_snapshot", snapshot)
        }

        val runtime = JSONObject(runtimeJSON).optJSONObject("data")
        if (runtime != null) {
            data.put("runtime", runtime)
        }
        data.put("battery", battery)
        if (keepAlive != null) {
            data.put("keep_alive", keepAlive)
        }
        return data
    }
}
