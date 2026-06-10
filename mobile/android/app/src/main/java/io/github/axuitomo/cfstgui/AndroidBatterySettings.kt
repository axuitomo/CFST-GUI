package io.github.axuitomo.cfstgui

import android.annotation.SuppressLint
import android.content.ActivityNotFoundException
import android.content.ComponentName
import android.content.Context
import android.content.Intent
import android.net.Uri
import android.os.Build
import android.os.PowerManager
import android.provider.Settings
import android.util.Log
import com.getcapacitor.JSObject
import java.util.Locale

object AndroidBatterySettings {
    private const val TAG = "AndroidBatterySettings"

    fun interface IntentStarter {
        fun tryStart(intent: Intent): Boolean
    }

    @JvmStatic
    fun openSettings(context: Context, mode: String?) {
        openSettings(context, mode, IntentStarter { intent ->
            try {
                context.startActivity(intent)
                true
            } catch (error: ActivityNotFoundException) {
                Log.e(TAG, "Failed to start Android battery settings intent.", error)
                false
            } catch (error: SecurityException) {
                Log.e(TAG, "Failed to start Android battery settings intent.", error)
                false
            }
        })
    }

    @JvmStatic
    fun openSettings(context: Context, mode: String?, starter: IntentStarter) {
        if (mode?.trim()?.lowercase(Locale.ROOT) == "settings") {
            for (intent in manufacturerSettingsIntents()) {
                intent.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
                if (starter.tryStart(intent)) {
                    return
                }
            }
        }
        if (!starter.tryStart(settingsIntent(context, mode))) {
            throw IllegalStateException("系统无法打开省电策略设置。")
        }
    }

    @JvmStatic
    @SuppressLint("BatteryLife")
    fun settingsIntent(context: Context, mode: String?): Intent {
        val normalized = mode?.trim()?.lowercase(Locale.ROOT) ?: "request"
        return when (normalized) {
            "request" -> Intent(Settings.ACTION_REQUEST_IGNORE_BATTERY_OPTIMIZATIONS).apply {
                data = Uri.parse("package:${context.packageName}")
            }
            "details" -> Intent(Settings.ACTION_APPLICATION_DETAILS_SETTINGS).apply {
                data = Uri.parse("package:${context.packageName}")
            }
            else -> Intent(Settings.ACTION_IGNORE_BATTERY_OPTIMIZATION_SETTINGS)
        }.apply {
            addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
        }
    }

    @JvmStatic
    fun manufacturerSettingsIntents(): List<Intent> {
        val manufacturer = Build.MANUFACTURER?.trim()?.lowercase(Locale.ROOT).orEmpty()
        val intents = mutableListOf<Intent>()
        when {
            manufacturer.contains("xiaomi") || manufacturer.contains("redmi") || manufacturer.contains("poco") -> {
                intents += componentIntent("com.miui.securitycenter", "com.miui.permcenter.autostart.AutoStartManagementActivity")
                intents += componentIntent("com.miui.securitycenter", "com.miui.powercenter.PowerSettings")
            }
            manufacturer.contains("huawei") || manufacturer.contains("honor") -> {
                intents += componentIntent("com.huawei.systemmanager", "com.huawei.systemmanager.startupmgr.ui.StartupNormalAppListActivity")
                intents += componentIntent("com.huawei.systemmanager", "com.huawei.systemmanager.optimize.process.ProtectActivity")
            }
            manufacturer.contains("oppo") || manufacturer.contains("oneplus") || manufacturer.contains("realme") -> {
                intents += componentIntent("com.coloros.oppoguardelf", "com.coloros.powermanager.fuelgaue.PowerUsageModelActivity")
                intents += componentIntent("com.oplus.battery", "com.oplus.powermanager.fuelgaue.PowerUsageModelActivity")
            }
            manufacturer.contains("vivo") || manufacturer.contains("iqoo") -> {
                intents += componentIntent("com.iqoo.secure", "com.iqoo.secure.ui.phoneoptimize.AddWhiteListActivity")
                intents += componentIntent("com.vivo.permissionmanager", "com.vivo.permissionmanager.activity.BgStartUpManagerActivity")
            }
            manufacturer.contains("samsung") -> {
                intents += Intent(Intent.ACTION_POWER_USAGE_SUMMARY)
            }
        }
        return intents
    }

    @JvmStatic
    fun manufacturerHint(): String {
        val manufacturer = Build.MANUFACTURER?.trim()?.lowercase(Locale.ROOT).orEmpty()
        return when {
            manufacturer.contains("xiaomi") || manufacturer.contains("redmi") || manufacturer.contains("poco") ->
                "MIUI/HyperOS 常见于“省电策略”“后台弹出界面”“自启动管理”，建议同时放行。"
            manufacturer.contains("huawei") || manufacturer.contains("honor") ->
                "华为/荣耀常见于“启动管理”“应用启动”“电池优化”，建议允许后台活动。"
            manufacturer.contains("oppo") || manufacturer.contains("oneplus") || manufacturer.contains("realme") ->
                "OPPO/OnePlus/realme 常见于“自动启动”“后台冻结”“耗电保护”，建议关闭限制。"
            manufacturer.contains("vivo") || manufacturer.contains("iqoo") ->
                "vivo/iQOO 常见于“后台高耗电”“自启动管理”，建议允许后台运行。"
            manufacturer.contains("samsung") ->
                "Samsung 常见于“电池-后台使用限制”“未使用应用休眠”，建议加入永不休眠。"
            else ->
                "若系统仍会回收后台任务，请同时检查厂商自启动、后台冻结和电池优化设置。"
        }
    }

    @JvmStatic
    fun statusPayload(context: Context): JSObject {
        val ignoring = try {
            val powerManager = context.getSystemService(Context.POWER_SERVICE) as? PowerManager
            powerManager != null && powerManager.isIgnoringBatteryOptimizations(context.packageName)
        } catch (_: Exception) {
            false
        }
        val data = JSObject()
        data.put("supported", true)
        data.put("ignoring_optimizations", ignoring)
        data.put("manufacturer", Build.MANUFACTURER?.trim().orEmpty())
        data.put("brand", Build.BRAND?.trim().orEmpty())
        data.put("model", Build.MODEL?.trim().orEmpty())
        data.put("needs_guidance", !ignoring)
        data.put("settings_hint", manufacturerHint())
        return data
    }

    private fun componentIntent(packageName: String, className: String): Intent =
        Intent().apply {
            component = ComponentName(packageName, className)
        }
}
