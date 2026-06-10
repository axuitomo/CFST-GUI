package io.github.axuitomo.cfstgui

import android.content.Context
import android.util.Log
import java.util.concurrent.CopyOnWriteArrayList
import java.util.concurrent.ExecutorService
import java.util.concurrent.Executors
import mobileapi.EventSink
import mobileapi.Mobileapi
import mobileapi.Service
import org.json.JSONObject

object CfstRuntime {
    private const val TAG = "CfstRuntime"
    private val executor: ExecutorService = Executors.newSingleThreadExecutor()
    private val lock = Any()
    private val auxiliaryListeners = CopyOnWriteArrayList<ProbeEventListener>()
    private var serviceInstance: Service? = null
    private var pluginListener: ProbeEventListener? = null
    private var lastEventSeq = 0

    fun interface ProbeEventListener {
        fun onProbeEvent(eventJSON: String)
    }

    private val bridgeSink = EventSink { eventJSON ->
        dispatchProbeEvent(eventJSON)
    }

    @JvmStatic
    fun service(): Service = synchronized(lock) {
        var current = serviceInstance
        if (current == null) {
            current = Mobileapi.newService()
            current.setEventSink(bridgeSink)
            serviceInstance = current
        }
        current
    }

    @JvmStatic
    fun ensureInitialized(context: Context, runtimeDir: String) {
        synchronized(lock) {
            val current = service()
            current.setEventSink(bridgeSink)
            try {
                current.init(runtimeDir)
            } catch (error: Exception) {
                Log.e(TAG, "Failed to initialize runtime", error)
            }
        }
    }

    @JvmStatic
    fun executor(): ExecutorService = executor

    @JvmStatic
    fun setPluginListener(listener: ProbeEventListener?) {
        synchronized(lock) {
            pluginListener = listener
        }
    }

    @JvmStatic
    fun registerAuxiliaryListener(listener: ProbeEventListener?) {
        if (listener == null) {
            return
        }
        auxiliaryListeners.addIfAbsent(listener)
    }

    @JvmStatic
    fun unregisterAuxiliaryListener(listener: ProbeEventListener?) {
        if (listener == null) {
            return
        }
        auxiliaryListeners.remove(listener)
    }

    @JvmStatic
    fun emitSyntheticProbeEvent(taskId: String?, event: String, payload: JSONObject?) {
        try {
            val envelope = JSONObject()
            envelope.put("event", event)
            envelope.put("payload", payload ?: JSONObject())
            envelope.put("schema_version", "cfst-gui-mobile-v1")
            envelope.put("seq", nextEventSeq())
            envelope.put("task_id", taskId?.trim().orEmpty())
            envelope.put("ts", CfstPlugin.nowRFC3339UTC())
            dispatchProbeEvent(envelope.toString())
        } catch (error: Exception) {
            Log.e(TAG, "Failed to emit synthetic probe event", error)
        }
    }

    @JvmStatic
    fun hasRunningOrPausedTask(): Boolean {
        return try {
            val command = JSONObject(service().loadTaskSnapshot("{}"))
            if (!command.optBoolean("ok", false)) {
                return false
            }
            val data = command.optJSONObject("data") ?: return false
            val sessionState = data.optString("session_state", "").trim()
            sessionState == "active_runtime" || sessionState == "paused_runtime"
        } catch (error: Exception) {
            Log.e(TAG, "Failed to inspect runtime task state", error)
            false
        }
    }

    private fun dispatchProbeEvent(eventJSON: String) {
        updateLastEventSeq(eventJSON)
        val currentPluginListener = synchronized(lock) {
            pluginListener
        }
        if (currentPluginListener != null) {
            try {
                currentPluginListener.onProbeEvent(eventJSON)
            } catch (error: Exception) {
                Log.e(TAG, "Plugin listener failed", error)
            }
        }
        for (listener in auxiliaryListeners) {
            try {
                listener.onProbeEvent(eventJSON)
            } catch (error: Exception) {
                Log.e(TAG, "Auxiliary probe listener failed", error)
            }
        }
    }

    private fun updateLastEventSeq(eventJSON: String) {
        try {
            val event = JSONObject(eventJSON)
            val seq = event.optInt("seq", 0)
            synchronized(lock) {
                if (seq > lastEventSeq) {
                    lastEventSeq = seq
                }
            }
        } catch (_: Exception) {
            // Keep synthetic sequence monotonic even when the payload is malformed.
        }
    }

    private fun nextEventSeq(): Int = synchronized(lock) {
        lastEventSeq += 1
        lastEventSeq
    }
}
