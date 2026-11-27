package io.github.axuitomo.cfstgui

import android.annotation.SuppressLint
import android.content.Context
import android.content.pm.PackageInfo
import android.content.pm.PackageManager
import android.os.Build
import com.getcapacitor.JSObject

object AndroidAppInfo {
    @JvmStatic
    fun appVersion(context: Context): String {
        return try {
            currentPackageInfo(context).versionName?.trim().orEmpty().ifEmpty { "1.0" }
        } catch (_: Exception) {
            "1.0"
        }
    }

    @JvmStatic
    fun appInfoPayload(context: Context): JSObject {
        val data = JSObject()
        data.put("current_version", appVersion(context))
        data.put("install_mode", "android_apk")
        data.put("platform", "android")
        data.put("release_url", AndroidUpdateRelease.RELEASE_PAGE_URL)
        data.put("battery_optimization_supported", true)
        return data
    }

    @SuppressLint("NewApi")
    private fun currentPackageInfo(context: Context): PackageInfo {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
            return context.packageManager.getPackageInfo(context.packageName, PackageManager.PackageInfoFlags.of(0))
        }
        @Suppress("DEPRECATION")
        return context.packageManager.getPackageInfo(context.packageName, 0)
    }
}
