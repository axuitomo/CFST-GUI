package io.github.axuitomo.cfstgui

import android.Manifest
import android.app.Activity
import android.content.ActivityNotFoundException
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.net.Uri
import android.os.Build
import android.provider.Settings
import android.util.Log
import androidx.core.content.ContextCompat
import com.getcapacitor.JSObject
import com.getcapacitor.PermissionState

object AndroidNotificationPermissions {
    const val ALIAS = "notifications"
    private const val PREFS_NAME = "cfst_android_notification_permissions"
    private const val KEY_DENIED_REQUEST_COUNT = "denied_request_count"
    private const val TAG = "AndroidNotificationPerm"

    fun interface IntentStarter {
        fun tryStart(intent: Intent): Boolean
    }

    @JvmStatic
    fun supported(): Boolean = Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU

    @JvmStatic
    fun granted(context: Context): Boolean {
        return !supported() ||
            ContextCompat.checkSelfPermission(context, Manifest.permission.POST_NOTIFICATIONS) == PackageManager.PERMISSION_GRANTED
    }

    @JvmStatic
    fun requestAlreadyAttempted(context: Context): Boolean {
        return deniedRequestCount(context) > 0
    }

    @JvmStatic
    fun deniedRequestCount(context: Context): Int {
        return context.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE).getInt(KEY_DENIED_REQUEST_COUNT, 0)
    }

    @JvmStatic
    fun recordRequestResult(context: Context, granted: Boolean) {
        if (granted) {
            clearRequestHistory(context)
            return
        }
        val deniedCount = (deniedRequestCount(context) + 1).coerceAtMost(2)
        context.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE)
            .edit()
            .putInt(KEY_DENIED_REQUEST_COUNT, deniedCount)
            .apply()
    }

    @JvmStatic
    fun clearRequestHistory(context: Context) {
        context.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE)
            .edit()
            .remove(KEY_DENIED_REQUEST_COUNT)
            .apply()
    }

    @JvmStatic
    fun statusPayload(
        context: Context,
        permissionState: String?,
        activity: Activity?,
        deniedRequestCount: Int = deniedRequestCount(context),
    ): JSObject {
        val supported = supported()
        val granted = granted(context)
        val shouldShowRationale = supported && activity != null && activity.shouldShowRequestPermissionRationale(Manifest.permission.POST_NOTIFICATIONS)
        val openSettingsRecommended = supported && !granted && deniedRequestCount >= 2 && !shouldShowRationale
        val canRequest = supported && !granted && activity != null && !openSettingsRecommended
        val data = JSObject()
        data.put("supported", supported)
        data.put("granted", granted)
        data.put("state", if (supported) permissionState.orEmpty() else PermissionState.GRANTED.toString())
        data.put("should_show_rationale", shouldShowRationale)
        data.put("can_request", canRequest)
        data.put("open_settings_recommended", openSettingsRecommended)
        data.put("request_already_attempted", supported && deniedRequestCount > 0)
        data.put(
            "message",
            if (supported) {
                if (granted) {
                    "通知权限已允许。"
                } else if (openSettingsRecommended) {
                    "系统可能已不再显示通知授权弹窗，请到应用通知设置中手动允许。"
                } else {
                    "Android 13+ 需要允许通知，前台服务和定时任务状态才会稳定显示。"
                }
            } else {
                "当前 Android 版本无需运行时通知权限。"
            },
        )
        return data
    }

    @JvmStatic
    fun openSettings(context: Context) {
        openSettings(context, IntentStarter { intent ->
            try {
                context.startActivity(intent)
                true
            } catch (error: ActivityNotFoundException) {
                Log.e(TAG, "Failed to start Android notification settings intent.", error)
                false
            } catch (error: SecurityException) {
                Log.e(TAG, "Failed to start Android notification settings intent.", error)
                false
            }
        })
    }

    @JvmStatic
    fun openSettings(context: Context, starter: IntentStarter) {
        for (intent in settingsIntents(context)) {
            if (starter.tryStart(intent)) {
                return
            }
        }
        throw IllegalStateException("系统无法打开通知权限设置。")
    }

    @JvmStatic
    fun settingsIntents(context: Context): List<Intent> {
        return listOf(
            Intent(Settings.ACTION_APP_NOTIFICATION_SETTINGS).apply {
                putExtra(Settings.EXTRA_APP_PACKAGE, context.packageName)
            },
            Intent(Settings.ACTION_APPLICATION_DETAILS_SETTINGS).apply {
                data = Uri.parse("package:${context.packageName}")
            },
        ).map { intent ->
            intent.apply {
                addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
            }
        }
    }
}
