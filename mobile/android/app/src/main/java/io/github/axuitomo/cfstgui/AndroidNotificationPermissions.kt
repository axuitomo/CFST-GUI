package io.github.axuitomo.cfstgui

import android.Manifest
import android.app.Activity
import android.content.Context
import android.content.pm.PackageManager
import android.os.Build
import androidx.core.content.ContextCompat
import com.getcapacitor.JSObject
import com.getcapacitor.PermissionState

object AndroidNotificationPermissions {
    const val ALIAS = "notifications"

    @JvmStatic
    fun supported(): Boolean = Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU

    @JvmStatic
    fun granted(context: Context): Boolean {
        return !supported() ||
            ContextCompat.checkSelfPermission(context, Manifest.permission.POST_NOTIFICATIONS) == PackageManager.PERMISSION_GRANTED
    }

    @JvmStatic
    fun statusPayload(context: Context, permissionState: String?, activity: Activity?): JSObject {
        val supported = supported()
        val granted = granted(context)
        val data = JSObject()
        data.put("supported", supported)
        data.put("granted", granted)
        data.put("state", if (supported) permissionState.orEmpty() else PermissionState.GRANTED.toString())
        data.put("should_show_rationale", supported && activity != null && activity.shouldShowRequestPermissionRationale(Manifest.permission.POST_NOTIFICATIONS))
        data.put(
            "message",
            if (supported) {
                if (granted) {
                    "通知权限已允许。"
                } else {
                    "Android 13+ 需要允许通知，前台服务和定时任务状态才会稳定显示。"
                }
            } else {
                "当前 Android 版本无需运行时通知权限。"
            },
        )
        return data
    }
}
