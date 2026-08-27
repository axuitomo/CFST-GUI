package io.github.axuitomo.cfstgui

import android.Manifest
import android.content.Intent
import android.util.Log
import androidx.activity.result.ActivityResult
import androidx.activity.result.ActivityResultLauncher
import androidx.activity.result.contract.ActivityResultContracts
import com.getcapacitor.JSObject
import com.getcapacitor.Plugin
import com.getcapacitor.PluginCall
import com.getcapacitor.PluginMethod
import com.getcapacitor.annotation.CapacitorPlugin
import com.getcapacitor.annotation.Permission
import com.getcapacitor.annotation.PermissionCallback
import java.io.File
import java.util.concurrent.ExecutorService
import java.util.concurrent.Executors
import mobileapi.Service
import org.json.JSONObject

@CapacitorPlugin(
    name = "Cfst",
    permissions = [
        Permission(alias = "notifications", strings = [Manifest.permission.POST_NOTIFICATIONS]),
    ],
)
class CfstPlugin : Plugin() {
    private val executor: ExecutorService = Executors.newSingleThreadExecutor()
    private var selectPathLauncher: ActivityResultLauncher<Intent>? = null
    private var pendingSelectPathCall: PluginCall? = null
    private lateinit var service: Service

    override fun load() {
        AndroidUpdateInstaller.cleanupDownloadedPackages(context)
        selectPathLauncher = bridge.registerForActivityResult(ActivityResultContracts.StartActivityForResult()) { result ->
            val call = synchronized(this) {
                val current = pendingSelectPathCall
                pendingSelectPathCall = null
                current
            }
            handleSelectPathResult(call, result)
        }
        val sink = CfstRuntime.ProbeEventListener { eventJSON ->
            try {
                notifyListeners("probe:event", JSObject(eventJSON))
            } catch (error: Exception) {
                logPluginError("Failed to decode probe event, retrying with raw payload.", error)
                try {
                    notifyListeners("probe:event", JSObject(eventJSON))
                } catch (rawError: Exception) {
                    logPluginError("Failed to dispatch raw probe event.", rawError)
                    val fallback = JSObject()
                    fallback.put("event", "probe.failed")
                    fallback.put("schema_version", "cfst-gui-event-v2")
                    fallback.put("task_id", "")
                    fallback.put("seq", 0)
                    fallback.put("ts", "")
                    val payload = JSObject()
                    payload.put("bridge_error", error.message)
                    payload.put("message", "Android 原生事件桥接失败：" + rawError.message)
                    fallback.put("payload", payload)
                    notifyListeners("probe:event", fallback)
                }
            }
        }
        try {
            val runtimeDir = AndroidStorageState.resolveRuntimeDirectory(
                context,
                AndroidStorageState.readStorageBootstrap(context),
            )
            CfstRuntime.setPluginListener(sink)
            CfstRuntime.ensureInitialized(context, runtimeDir)
            service = CfstRuntime.service()
        } catch (error: Exception) {
            logPluginError("Failed to initialize storage-backed runtime directory, falling back to default private storage.", error)
            CfstRuntime.setPluginListener(sink)
            CfstRuntime.ensureInitialized(context, defaultRuntimeDir().absolutePath)
            service = CfstRuntime.service()
        }
        rearmSchedulerOnStartup()
        startKeepAliveIfAllowed()
    }

    @PluginMethod
    fun Init(call: PluginCall) {
        runAsync(call) { initializeServiceFromStorage() }
    }

    @PluginMethod
    fun Invoke(call: PluginCall) {
        val command = call.getString("command", "")?.trim()?.lowercase().orEmpty()
        val payload = call.getString("payload_json", "{}")?.ifBlank { "{}" } ?: "{}"
        when (command) {
            "probe.start" -> {
                try {
                    val taskId = JSONObject(payload).optString("task_id", "")
                    call.resolve(AndroidProbeStart.startProbe(context, payload, taskId))
                } catch (error: Exception) {
                    rejectWithLog(call, "Invoke(probe.start)", error)
                }
            }
            "probe.pause", "probe.cancel" -> {
                try {
                    val response = service.invoke(command, payload)
                    call.resolve(JSObject(AndroidPluginCommands.finalizeServiceResponse(context, response)))
                } catch (error: Exception) {
                    rejectWithLog(call, "Invoke($command)", error)
                }
            }
            "config.export", "archive.export", "results.export_csv", "debug.export", "diagnostics.export" -> {
                executor.execute {
                    try {
                        val response = when (command) {
                            "config.export" -> AndroidExportFlow.exportConfig(context, payload) { request -> service.invoke(command, request) }
                            "archive.export" -> AndroidExportFlow.exportConfigArchive(context, payload) { request -> service.invoke(command, request) }
                            "results.export_csv" -> AndroidExportFlow.exportResultsCSV(context, payload) { request -> service.invoke(command, request) }
                            "debug.export" -> AndroidExportFlow.exportDebugLog(context, payload) { request -> service.invoke(command, request) }
                            else -> AndroidExportFlow.exportDiagnosticPackage(context, payload) { request -> service.invoke(command, request) }
                        }
                        call.resolve(JSObject(response))
                    } catch (error: Exception) {
                        rejectWithLog(call, "Invoke($command)", error)
                    }
                }
            }
            "storage.set" -> {
                executor.execute {
                    try {
                        call.resolve(AndroidStorageDirectory.commandForDeprecatedChange(context) { runtimeDir -> service.init(runtimeDir) })
                    } catch (error: Exception) {
                        rejectWithLog(call, "Invoke(storage.set)", error)
                    }
                }
            }
            "scheduler.refresh" -> runAsync(call) { SchedulerWorker.refresh(context) }
            else -> {
                runAsync(call) {
                    val response = service.invoke(command, payload)
                    if (command == "config.save") {
                        SchedulerWorker.refresh(context)
                    }
                    response
                }
            }
        }
    }

    private fun rearmSchedulerOnStartup() {
        executor.execute {
            try {
                SchedulerWorker.refresh(context)
            } catch (error: Exception) {
                logPluginError("Failed to rearm Android scheduler on startup.", error)
            }
        }
    }

    @PluginMethod
    fun GetAppInfo(call: PluginCall) {
        call.resolve(AndroidPluginCommands.command("APP_INFO_READY", AndroidAppInfo.appInfoPayload(context), "应用信息已读取。", true))
    }

    @PluginMethod
    fun GetAndroidRuntimeStatus(call: PluginCall) {
        executor.execute {
            try {
                call.resolve(JSObject(AndroidPluginCommands.commandJSON("ANDROID_RUNTIME_STATUS", androidRuntimeStatusPayload(), "Android 运行时状态已读取。", true)))
            } catch (error: Exception) {
                rejectWithLog(call, "GetAndroidRuntimeStatus", error)
            }
        }
    }

    @PluginMethod
    fun CheckBatteryOptimization(call: PluginCall) {
        executor.execute {
            try {
                call.resolve(JSObject(AndroidPluginCommands.commandJSON("ANDROID_BATTERY_STATUS", batteryOptimizationPayload(), "省电策略状态已读取。", true)))
            } catch (error: Exception) {
                rejectWithLog(call, "CheckBatteryOptimization", error)
            }
        }
    }

    @PluginMethod
    fun CheckKeepAliveStatus(call: PluginCall) {
        startKeepAliveIfAllowed()
        call.resolve(AndroidPluginCommands.command("ANDROID_KEEP_ALIVE_STATUS", keepAlivePayload(), "通知栏保活状态已读取。", true))
    }

    @PluginMethod
    fun CheckNotificationPermission(call: PluginCall) {
        call.resolve(AndroidPluginCommands.command("ANDROID_NOTIFICATION_PERMISSION", notificationPermissionPayload(), "通知权限状态已读取。", true))
    }

    @PluginMethod
    fun RequestNotificationPermission(call: PluginCall) {
        if (!notificationPermissionSupported() || notificationPermissionGranted()) {
            call.resolve(AndroidPluginCommands.command("ANDROID_NOTIFICATION_PERMISSION", notificationPermissionPayload(), "通知权限已允许。", true))
            return
        }
        val payload = notificationPermissionPayload()
        if (payload.getBoolean("can_request", false) != true) {
            call.resolve(AndroidPluginCommands.command("ANDROID_NOTIFICATION_PERMISSION", payload, payload.optString("message", "通知权限未允许。"), false))
            return
        }
        requestPermissionForAlias(AndroidNotificationPermissions.ALIAS, call, "notificationPermissionCallback")
    }

    @PermissionCallback
    private fun notificationPermissionCallback(call: PluginCall?) {
        if (call == null) {
            return
        }
        val granted = notificationPermissionGranted()
        AndroidNotificationPermissions.recordRequestResult(context, granted)
        if (granted) {
            startKeepAliveIfAllowed()
        }
        call.resolve(
            AndroidPluginCommands.command(
                "ANDROID_NOTIFICATION_PERMISSION",
                notificationPermissionPayload(),
                if (granted) "通知权限已允许。" else "通知权限未允许，后台任务通知可能不可见。",
                granted,
            ),
        )
    }

    @PluginMethod
    fun OpenNotificationSettings(call: PluginCall) {
        try {
            AndroidNotificationPermissions.openSettings(context)
            call.resolve(AndroidPluginCommands.command("ANDROID_NOTIFICATION_SETTINGS_OPENED", notificationPermissionPayload(), "已打开 Android 通知权限设置。", true))
        } catch (error: Exception) {
            rejectWithLog(call, "OpenNotificationSettings", error)
        }
    }

    @PluginMethod
    fun SetKeepAliveEnabled(call: PluginCall) {
        try {
            val enabled = call.getBoolean("enabled", true) == true
            val data = AndroidKeepAliveState.setEnabled(context, enabled)
            call.resolve(AndroidPluginCommands.command("ANDROID_KEEP_ALIVE_UPDATED", data, data.optString("message", "通知栏保活状态已更新。"), true))
        } catch (error: Exception) {
            rejectWithLog(call, "SetKeepAliveEnabled", error)
        }
    }

    @PluginMethod
    fun OpenBatteryOptimizationSettings(call: PluginCall) {
        executor.execute {
            try {
                val mode = call.getString("mode", "request")
                openBatteryOptimizationSettings(mode)
                val data = batteryOptimizationPayload()
                data.put("mode", mode?.trim().orEmpty())
                call.resolve(AndroidPluginCommands.command("ANDROID_BATTERY_SETTINGS_OPENED", data, "已打开 Android 省电策略设置。", true))
            } catch (error: Exception) {
                rejectWithLog(call, "OpenBatteryOptimizationSettings", error)
            }
        }
    }

    @PluginMethod
    fun CheckForUpdates(call: PluginCall) {
        runAsync(call) {
            AndroidPluginCommands.commandJSON("UPDATE_CHECK_OK", AndroidUpdateRelease.checkForUpdatesPayload(appVersion()), "更新检查完成。", true)
        }
    }

    @PluginMethod
    fun DownloadAndInstallUpdate(call: PluginCall) {
        executor.execute {
            try {
                call.resolve(AndroidUpdateInstallFlow.commandForDownloadAndInstall(context, appVersion()))
            } catch (error: Exception) {
                call.reject(error.message, error)
            }
        }
    }

    @PluginMethod
    fun OpenReleasePage(call: PluginCall) {
        try {
            call.resolve(AndroidExternalNavigation.openReleasePageCommand(context))
        } catch (error: Exception) {
            call.reject(error.message, error)
        }
    }

    @PluginMethod
    fun OpenLogDirectory(call: PluginCall) {
        try {
            val payload = JSONObject(call.data.toString())
            val config = payload.optJSONObject("config")
                ?: payload.optJSONObject("config_snapshot")
                ?: payload.optJSONObject("configSnapshot")
            val exportConfig = config?.optJSONObject("export")
            val targetUri = listOf(
                payload.optString("target_uri"),
                payload.optString("targetUri"),
                payload.optString("uri"),
                exportConfig?.optString("target_uri").orEmpty(),
                exportConfig?.optString("targetUri").orEmpty(),
            ).firstOrNull { it.isNotBlank() }.orEmpty()
            if (targetUri.isBlank()) {
                call.resolve(AndroidPluginCommands.command("LOG_DIRECTORY_EXPORT_TARGET_REQUIRED", null, "请先选择 Android SAF 导出目录或导出诊断包。", false))
            } else {
                val data = JSObject().apply {
                    put("target_uri", targetUri)
                    put("uri", targetUri)
                }
                call.resolve(AndroidPluginCommands.command("LOG_DIRECTORY_EXPORT_TARGET", data, "已定位 Android SAF 导出目录。", true))
            }
        } catch (error: Exception) {
            rejectWithLog(call, "OpenLogDirectory", error)
        }
    }

    @PluginMethod
    fun OpenPath(call: PluginCall) {
        executor.execute {
            try {
                call.resolve(AndroidExternalNavigation.openPathCommand(context, call.getString("targetPath", "")))
            } catch (error: Exception) {
                call.reject(error.message, error)
            }
        }
    }

    @PluginMethod
    fun SelectPath(call: PluginCall) {
        val mode = AndroidPathSelection.normalizeMode(call.getString("mode", ""))
        if (AndroidPathSelection.isStorageDirMode(mode)) {
            val data = JSObject()
            data.put("canceled", false)
            data.put("mode", mode)
            data.put("path", defaultRuntimeDir().absolutePath)
            data.put("directory", defaultRuntimeDir().absolutePath)
            call.resolve(AndroidPluginCommands.command("PATH_SELECTION_DEPRECATED", data, "当前版本不再支持自定义储存目录，Android 固定使用应用私有目录。", true))
            return
        }
        val intent = AndroidPathSelection.pickerIntent(
            mode,
            call.getString("defaultFileName", call.getString("default_file_name", "result.csv")),
            AndroidPayloads.firstNonEmpty(call.getString("current_path", ""), call.getString("currentPath", "")),
        )
        synchronized(this) {
            if (pendingSelectPathCall != null) {
                call.reject("已有系统文件选择正在进行。")
                return
            }
            pendingSelectPathCall = call
        }
        try {
            val launcher = selectPathLauncher ?: throw IllegalStateException("系统文件选择器尚未初始化。")
            launcher.launch(intent)
        } catch (error: Exception) {
            synchronized(this) {
                if (pendingSelectPathCall === call) {
                    pendingSelectPathCall = null
                }
            }
            call.reject(error.message, error)
        }
    }

    fun handleSelectPathResult(call: PluginCall?, result: ActivityResult) {
        if (call == null) {
            return
        }
        try {
            call.resolve(
                AndroidPathSelectionResults.commandForResult(
                    context,
                    call.getString("mode", ""),
                    result.resultCode,
                    result.data,
                ),
            )
        } catch (error: Exception) {
            call.reject(error.message, error)
        }
    }

    private fun initializeServiceFromStorage(): String {
        val bootstrap = AndroidStorageState.readStorageBootstrap(context)
        val runtimeDir = AndroidStorageState.resolveRuntimeDirectory(context, bootstrap)
        return service.init(runtimeDir)
    }

    private fun androidRuntimeStatusPayload(): JSObject {
        return AndroidRuntimeStatus.payload(service, isProbeForegroundServiceRunning(), batteryOptimizationPayload(), keepAlivePayload())
    }

    private fun batteryOptimizationPayload(): JSObject {
        return AndroidBatterySettings.statusPayload(context)
    }

    private fun keepAlivePayload(): JSObject {
        return AndroidKeepAliveState.statusPayload(context)
    }

    private fun notificationPermissionPayload(): JSObject {
        if (notificationPermissionGranted()) {
            AndroidNotificationPermissions.clearRequestHistory(context)
        }
        return AndroidNotificationPermissions.statusPayload(
            context,
            getPermissionState(AndroidNotificationPermissions.ALIAS).toString(),
            activity,
        )
    }

    private fun notificationPermissionSupported(): Boolean {
        return AndroidNotificationPermissions.supported()
    }

    private fun notificationPermissionGranted(): Boolean {
        return AndroidNotificationPermissions.granted(context)
    }

    private fun defaultRuntimeDir(): File {
        return AndroidStorageState.defaultRuntimeDir(context)
    }

    private fun openBatteryOptimizationSettings(mode: String?) {
        AndroidBatterySettings.openSettings(context, mode)
    }

    private fun startKeepAliveIfAllowed() {
        AndroidKeepAliveState.startIfAllowed(context)
    }

    private fun isProbeForegroundServiceRunning(): Boolean {
        return ProbeForegroundService.isForegroundRunning()
    }

    private fun runAsync(call: PluginCall, action: () -> String) {
        executor.execute {
            try {
                call.resolve(JSObject(AndroidPluginCommands.finalizeServiceResponse(context, action())))
            } catch (error: Exception) {
                rejectWithLog(call, "runAsync", error)
            }
        }
    }

    private fun rejectWithLog(call: PluginCall, action: String, error: Exception) {
        logPluginError("Plugin action failed: $action", error)
        call.reject(error.message, error)
    }

    private fun logPluginError(message: String, error: Throwable) {
        Log.e(TAG, message, error)
    }

    private fun appVersion(): String {
        return AndroidAppInfo.appVersion(context)
    }

    companion object {
        private const val TAG = "CfstPlugin"
        const val EXPORT_DIRECTORY_PERMISSION_LOST_MESSAGE = "Android 未持有所选导出目录的持久化权限，请重新选择导出目录。"
        const val EXPORT_DIRECTORY_OPEN_ERROR_MESSAGE = "系统无法打开该导出目录，请安装或启用文件管理器后重试。"

    }
}
