package io.github.axuitomo.cfstgui;

import android.content.Context;
import android.util.Log;
import java.util.concurrent.CopyOnWriteArrayList;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import mobileapi.EventSink;
import mobileapi.Mobileapi;
import mobileapi.Service;
import org.json.JSONArray;
import org.json.JSONObject;

final class CfstRuntime {
    private static final String TAG = "CfstRuntime";
    private static final ExecutorService EXECUTOR = Executors.newSingleThreadExecutor();
    private static final Object LOCK = new Object();
    private static final CopyOnWriteArrayList<ProbeEventListener> AUXILIARY_LISTENERS = new CopyOnWriteArrayList<>();
    private static Service service;
    private static ProbeEventListener pluginListener;
    private static int lastEventSeq;

    interface ProbeEventListener {
        void onProbeEvent(String eventJSON);
    }

    private static final EventSink BRIDGE_SINK = new EventSink() {
        @Override
        public void onProbeEvent(String eventJSON) {
            dispatchProbeEvent(eventJSON);
        }
    };

    private CfstRuntime() {}

    static Service service() {
        synchronized (LOCK) {
            if (service == null) {
                service = Mobileapi.newService();
                service.setEventSink(BRIDGE_SINK);
            }
            return service;
        }
    }

    static void ensureInitialized(Context context, String runtimeDir) {
        synchronized (LOCK) {
            Service current = service();
            current.setEventSink(BRIDGE_SINK);
            try {
                current.init(runtimeDir);
            } catch (Exception error) {
                Log.e(TAG, "Failed to initialize runtime", error);
            }
        }
    }

    static ExecutorService executor() {
        return EXECUTOR;
    }

    static void setPluginListener(ProbeEventListener listener) {
        synchronized (LOCK) {
            pluginListener = listener;
        }
    }

    static void registerAuxiliaryListener(ProbeEventListener listener) {
        if (listener == null) {
            return;
        }
        AUXILIARY_LISTENERS.addIfAbsent(listener);
    }

    static void unregisterAuxiliaryListener(ProbeEventListener listener) {
        if (listener == null) {
            return;
        }
        AUXILIARY_LISTENERS.remove(listener);
    }

    static void emitSyntheticProbeEvent(String taskId, String event, JSONObject payload) {
        try {
            JSONObject envelope = new JSONObject();
            envelope.put("event", event);
            envelope.put("payload", payload == null ? new JSONObject() : payload);
            envelope.put("schema_version", "cfst-gui-mobile-v1");
            envelope.put("seq", nextEventSeq());
            envelope.put("task_id", taskId == null ? "" : taskId.trim());
            envelope.put("ts", CfstPlugin.nowRFC3339UTC());
            dispatchProbeEvent(envelope.toString());
        } catch (Exception error) {
            Log.e(TAG, "Failed to emit synthetic probe event", error);
        }
    }

    static boolean hasRunningOrPausedTask() {
        try {
            JSONObject command = new JSONObject(service().loadTaskSnapshot("{}"));
            if (!command.optBoolean("ok", false)) {
                return false;
            }
            JSONObject data = command.optJSONObject("data");
            if (data == null) {
                return false;
            }
            String sessionState = data.optString("session_state", "").trim();
            return "active_runtime".equals(sessionState) || "paused_runtime".equals(sessionState);
        } catch (Exception error) {
            Log.e(TAG, "Failed to inspect runtime task state", error);
            return false;
        }
    }

    private static void dispatchProbeEvent(String eventJSON) {
        updateLastEventSeq(eventJSON);
        ProbeEventListener currentPluginListener;
        synchronized (LOCK) {
            currentPluginListener = pluginListener;
        }
        if (currentPluginListener != null) {
            try {
                currentPluginListener.onProbeEvent(eventJSON);
            } catch (Exception error) {
                Log.e(TAG, "Plugin listener failed", error);
            }
        }
        for (ProbeEventListener listener : AUXILIARY_LISTENERS) {
            try {
                listener.onProbeEvent(eventJSON);
            } catch (Exception error) {
                Log.e(TAG, "Auxiliary probe listener failed", error);
            }
        }
    }

    private static void updateLastEventSeq(String eventJSON) {
        try {
            JSONObject event = new JSONObject(eventJSON);
            int seq = event.optInt("seq", 0);
            synchronized (LOCK) {
                if (seq > lastEventSeq) {
                    lastEventSeq = seq;
                }
            }
        } catch (Exception ignored) {
            // Keep synthetic sequence monotonic even when the payload is malformed.
        }
    }

    private static int nextEventSeq() {
        synchronized (LOCK) {
            lastEventSeq += 1;
            return lastEventSeq;
        }
    }
}
