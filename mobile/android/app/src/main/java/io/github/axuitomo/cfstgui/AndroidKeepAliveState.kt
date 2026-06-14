package io.github.axuitomo.cfstgui

import android.content.Context
import android.util.Log
import com.getcapacitor.JSObject

object AndroidKeepAliveState {
    private const val PREFS_NAME = "cfst_android_keep_alive"
    private const val KEY_ENABLED = "enabled"
    private const val TAG = "AndroidKeepAliveState"

    @JvmStatic
    fun enabled(context: Context): Boolean {
        return context.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE).getBoolean(KEY_ENABLED, true)
    }

    @JvmStatic
    fun setEnabled(context: Context, enabled: Boolean): JSObject {
        context.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE)
            .edit()
            .putBoolean(KEY_ENABLED, enabled)
            .apply()
        if (enabled) {
            startIfAllowed(context)
        } else {
            stop(context)
        }
        return statusPayload(context)
    }

    @JvmStatic
    fun startIfAllowed(context: Context): Boolean {
        if (!enabled(context) || !AndroidNotificationPermissions.granted(context)) {
            return false
        }
        return try {
            context.startForegroundService(AndroidKeepAliveForegroundService.startIntent(context))
            true
        } catch (error: SecurityException) {
            Log.e(TAG, "Android keep-alive service could not be started.", error)
            false
        } catch (error: IllegalStateException) {
            Log.e(TAG, "Android keep-alive service could not be started.", error)
            false
        }
    }

    @JvmStatic
    fun stop(context: Context): Boolean {
        return context.stopService(AndroidKeepAliveForegroundService.startIntent(context))
    }

    @JvmStatic
    fun statusPayload(context: Context): JSObject {
        val enabled = enabled(context)
        val notificationGranted = AndroidNotificationPermissions.granted(context)
        val running = AndroidKeepAliveForegroundService.isRunning()
        val data = JSObject()
        data.put("supported", true)
        data.put("enabled", enabled)
        data.put("running", running)
        data.put("notification_permission_granted", notificationGranted)
        data.put("message", statusMessage(enabled, running, notificationGranted))
        return data
    }

    private fun statusMessage(enabled: Boolean, running: Boolean, notificationGranted: Boolean): String {
        if (!enabled) {
            return "通知栏保活已关闭；定时任务仍可运行，但更容易受系统后台策略影响。"
        }
        if (!notificationGranted) {
            return "通知栏保活已启用，等待通知权限后自动显示常驻通知。"
        }
        return if (running) {
            "通知栏保活运行中；这能提升后台任务稳定性，但仍受厂商省电策略影响。"
        } else {
            "通知栏保活已启用，正在等待系统启动常驻服务。"
        }
    }
}
