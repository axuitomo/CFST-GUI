package io.github.axuitomo.cfstgui

import android.Manifest
import android.content.Context
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
    private var runtimeDirPath: String = ""
    private lateinit var service: Service

    override fun load() {
        cleanupAndroidUpdateDownloads(context)
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
                notifyListeners("desktop:probe", JSObject(augmentProbeEvent(eventJSON)))
            } catch (error: Exception) {
                logPluginError("Failed to augment probe event, retrying with raw payload.", error)
                try {
                    notifyListeners("desktop:probe", JSObject(eventJSON))
                } catch (rawError: Exception) {
                    logPluginError("Failed to dispatch raw probe event.", rawError)
                    val fallback = JSObject()
                    fallback.put("event", "probe.failed")
                    fallback.put("schema_version", "cfst-gui-mobile-v1")
                    fallback.put("task_id", "")
                    fallback.put("seq", 0)
                    fallback.put("ts", "")
                    val payload = JSObject()
                    payload.put("bridge_error", error.message)
                    payload.put("message", "Android 原生事件桥接失败：" + rawError.message)
                    fallback.put("payload", payload)
                    notifyListeners("desktop:probe", fallback)
                }
            }
        }
        try {
            val runtimeDir = AndroidStorageState.resolveRuntimeDirectory(
                context,
                AndroidStorageState.readStorageBootstrap(context),
            )
            runtimeDirPath = runtimeDir
            CfstRuntime.setPluginListener(sink)
            CfstRuntime.ensureInitialized(context, runtimeDir)
            service = CfstRuntime.service()
            startLogMonitorIfConfigured()
        } catch (error: Exception) {
            logPluginError("Failed to initialize storage-backed runtime directory, falling back to default private storage.", error)
            runtimeDirPath = defaultRuntimeDir().absolutePath
            CfstRuntime.setPluginListener(sink)
            CfstRuntime.ensureInitialized(context, runtimeDirPath)
            service = CfstRuntime.service()
            startLogMonitorIfConfigured()
        }
        rearmSchedulerOnStartup()
        startKeepAliveIfAllowed()
    }

    @PluginMethod
    fun Init(call: PluginCall) {
        runAsync(call) { initializeServiceFromStorage() }
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
    fun LoadConfig(call: PluginCall) {
        executor.execute {
            try {
                call.resolve(JSObject(finalizeLoadConfigResponse(service.loadConfig())))
            } catch (error: Exception) {
                rejectWithLog(call, "LoadConfig", error)
            }
        }
    }

    @PluginMethod
    fun GetAppInfo(call: PluginCall) {
        call.resolve(command("APP_INFO_READY", AndroidAppInfo.appInfoPayload(context), "应用信息已读取。", true))
    }

    @PluginMethod
    fun GetAndroidRuntimeStatus(call: PluginCall) {
        executor.execute {
            try {
                call.resolve(JSObject(commandJSON("ANDROID_RUNTIME_STATUS", androidRuntimeStatusPayload(), "Android 运行时状态已读取。", true)))
            } catch (error: Exception) {
                rejectWithLog(call, "GetAndroidRuntimeStatus", error)
            }
        }
    }

    @PluginMethod
    fun CheckBatteryOptimization(call: PluginCall) {
        executor.execute {
            try {
                call.resolve(JSObject(commandJSON("ANDROID_BATTERY_STATUS", batteryOptimizationPayload(), "省电策略状态已读取。", true)))
            } catch (error: Exception) {
                rejectWithLog(call, "CheckBatteryOptimization", error)
            }
        }
    }

    @PluginMethod
    fun CheckKeepAliveStatus(call: PluginCall) {
        startKeepAliveIfAllowed()
        call.resolve(command("ANDROID_KEEP_ALIVE_STATUS", keepAlivePayload(), "通知栏保活状态已读取。", true))
    }

    @PluginMethod
    fun CheckNotificationPermission(call: PluginCall) {
        call.resolve(command("ANDROID_NOTIFICATION_PERMISSION", notificationPermissionPayload(), "通知权限状态已读取。", true))
    }

    @PluginMethod
    fun RequestNotificationPermission(call: PluginCall) {
        if (!notificationPermissionSupported() || notificationPermissionGranted()) {
            call.resolve(command("ANDROID_NOTIFICATION_PERMISSION", notificationPermissionPayload(), "通知权限已允许。", true))
            return
        }
        val payload = notificationPermissionPayload()
        if (payload.getBoolean("can_request", false) != true) {
            call.resolve(command("ANDROID_NOTIFICATION_PERMISSION", payload, payload.optString("message", "通知权限未允许。"), false))
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
            command(
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
            call.resolve(command("ANDROID_NOTIFICATION_SETTINGS_OPENED", notificationPermissionPayload(), "已打开 Android 通知权限设置。", true))
        } catch (error: Exception) {
            rejectWithLog(call, "OpenNotificationSettings", error)
        }
    }

    @PluginMethod
    fun SetKeepAliveEnabled(call: PluginCall) {
        try {
            val enabled = call.getBoolean("enabled", true) == true
            val data = AndroidKeepAliveState.setEnabled(context, enabled)
            call.resolve(command("ANDROID_KEEP_ALIVE_UPDATED", data, data.optString("message", "通知栏保活状态已更新。"), true))
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
                call.resolve(command("ANDROID_BATTERY_SETTINGS_OPENED", data, "已打开 Android 省电策略设置。", true))
            } catch (error: Exception) {
                rejectWithLog(call, "OpenBatteryOptimizationSettings", error)
            }
        }
    }

    @PluginMethod
    fun CheckForUpdates(call: PluginCall) {
        runAsync(call) {
            commandJSON("UPDATE_CHECK_OK", AndroidUpdateRelease.checkForUpdatesPayload(appVersion()), "更新检查完成。", true)
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
    fun SaveConfig(call: PluginCall) {
        runAsync(call, syncAfterWrite = true) {
            val response = service.saveConfig(call.data.toString())
            SchedulerWorker.refresh(context)
            startLogMonitorIfConfigured()
            response
        }
    }

    @PluginMethod
    fun SetStorageDirectory(call: PluginCall) {
        executor.execute {
            try {
                call.resolve(AndroidStorageDirectory.commandForDeprecatedChange(context) { runtimeDir -> service.init(runtimeDir) })
            } catch (error: Exception) {
                call.reject(error.message, error)
            }
        }
    }

    @PluginMethod
    fun CheckStorageHealth(call: PluginCall) {
        runAsync(call) { service.checkStorageHealth(call.data.toString()) }
    }

    @PluginMethod
    fun ExportConfig(call: PluginCall) {
        val payload = call.data.toString()
        executor.execute {
            try {
                call.resolve(JSObject(AndroidExportFlow.exportConfig(context, payload) { request -> service.exportConfig(request) }))
            } catch (error: Exception) {
                call.reject(error.message, error)
            }
        }
    }

    @PluginMethod
    fun BackupCurrentConfig(call: PluginCall) {
        runAsync(call, syncAfterWrite = true) { service.backupCurrentConfig(call.data.toString()) }
    }

    @PluginMethod
    fun ExportConfigArchive(call: PluginCall) {
        val payload = call.data.toString()
        executor.execute {
            try {
                call.resolve(JSObject(AndroidExportFlow.exportConfigArchive(context, payload) { request -> service.exportConfigArchive(request) }))
            } catch (error: Exception) {
                call.reject(error.message, error)
            }
        }
    }

    @PluginMethod
    fun ExportResultsCSV(call: PluginCall) {
        val payload = call.data.toString()
        executor.execute {
            try {
                call.resolve(JSObject(AndroidExportFlow.exportResultsCSV(context, payload) { request -> service.exportResultsCSV(request) }))
            } catch (error: Exception) {
                call.reject(error.message, error)
            }
        }
    }

    @PluginMethod
    fun ExportDebugLog(call: PluginCall) {
        val payload = call.data.toString()
        executor.execute {
            try {
                call.resolve(JSObject(AndroidExportFlow.exportDebugLog(context, payload) { request -> service.exportDebugLog(request) }))
            } catch (error: Exception) {
                call.reject(error.message, error)
            }
        }
    }

    @PluginMethod
    fun ExportDiagnosticBundle(call: PluginCall) {
        val payload = call.data.toString()
        executor.execute {
            try {
                call.resolve(JSObject(AndroidExportFlow.exportDiagnosticBundle(context, payload) { request -> service.exportDiagnosticBundle(request) }))
            } catch (error: Exception) {
                call.reject(error.message, error)
            }
        }
    }

    @PluginMethod
    fun OpenLogDirectory(call: PluginCall) {
        runAsync(call) { service.openLogDirectory(call.data.toString()) }
    }

    @PluginMethod
    fun ImportConfigArchive(call: PluginCall) {
        runAsync(call, syncAfterWrite = true) { service.importConfigArchive(call.data.toString()) }
    }

    @PluginMethod
    fun TestWebDAV(call: PluginCall) {
        runAsync(call) { service.testWebDAV(call.data.toString()) }
    }

    @PluginMethod
    fun BackupConfigToWebDAV(call: PluginCall) {
        runAsync(call) { service.backupConfigToWebDAV(call.data.toString()) }
    }

    @PluginMethod
    fun RestoreConfigFromWebDAV(call: PluginCall) {
        runAsync(call, syncAfterWrite = true) { service.restoreConfigFromWebDAV(call.data.toString()) }
    }

    @PluginMethod
    fun LoadSourceProfiles(call: PluginCall) {
        runAsync(call) { service.loadSourceProfiles() }
    }

    @PluginMethod
    fun SaveSourceProfile(call: PluginCall) {
        runAsync(call, syncAfterWrite = true) { service.saveSourceProfile(call.data.toString()) }
    }

    @PluginMethod
    fun UpdateCurrentSourceProfile(call: PluginCall) {
        runAsync(call, syncAfterWrite = true) { service.updateCurrentSourceProfile(call.data.toString()) }
    }

    @PluginMethod
    fun SaveSourceProfileStore(call: PluginCall) {
        runAsync(call, syncAfterWrite = true) { service.saveSourceProfileStore(call.data.toString()) }
    }

    @PluginMethod
    fun SwitchSourceProfile(call: PluginCall) {
        runAsync(call, syncAfterWrite = true) { service.switchSourceProfile(call.data.toString()) }
    }

    @PluginMethod
    fun DeleteSourceProfile(call: PluginCall) {
        runAsync(call, syncAfterWrite = true) { service.deleteSourceProfile(call.data.toString()) }
    }

    @PluginMethod
    fun PreviewSource(call: PluginCall) {
        runAsync(call) { service.previewSource(call.data.toString()) }
    }

    @PluginMethod
    fun FetchSource(call: PluginCall) {
        runAsync(call) { service.fetchSource(call.data.toString()) }
    }

    @PluginMethod
    fun LoadColoDictionaryStatus(call: PluginCall) {
        runAsync(call) { service.loadColoDictionaryStatus() }
    }

    @PluginMethod
    fun UpdateColoDictionary(call: PluginCall) {
        runAsync(call, syncAfterWrite = true) { service.updateColoDictionary(call.data.toString()) }
    }

    @PluginMethod
    fun ProcessColoDictionary(call: PluginCall) {
        runAsync(call, syncAfterWrite = true) { service.processColoDictionary(call.data.toString()) }
    }

    @PluginMethod
    fun RunProbe(call: PluginCall) {
        val payload = call.data.toString()
        try {
            call.resolve(AndroidProbeStart.startProbe(context, payload, call.getString("task_id", "") ?: ""))
        } catch (error: Exception) {
            rejectWithLog(call, "RunProbe", error)
        }
    }

    @PluginMethod
    fun CancelProbe(call: PluginCall) {
        try {
            call.resolve(JSObject(AndroidTaskBridge.cancelProbe(context, call.data.toString()) { payload -> service.cancelProbe(payload) }))
        } catch (error: Exception) {
            call.reject(error.message, error)
        }
    }

    @PluginMethod
    fun ResumeProbe(call: PluginCall) {
        try {
            call.resolve(JSObject(AndroidTaskBridge.resumeProbe(context, call.data.toString()) { payload -> service.resumeProbe(payload) }))
        } catch (error: Exception) {
            call.reject(error.message, error)
        }
    }

    @PluginMethod
    fun LoadTaskSnapshot(call: PluginCall) {
        runAsync(call) { service.loadTaskSnapshot(call.data.toString()) }
    }

    @PluginMethod
    fun ListResultFile(call: PluginCall) {
        val payload = call.data.toString()
        executor.execute {
            try {
                call.resolve(JSObject(AndroidTaskBridge.listResultFile(context, payload) { request -> service.listResultFile(request) }))
            } catch (error: Exception) {
                call.reject(error.message, error)
            }
        }
    }

    @PluginMethod
    fun ListCloudflareDNSRecords(call: PluginCall) {
        runAsync(call) { service.listCloudflareDNSRecords(call.data.toString()) }
    }

    @PluginMethod
    fun PushCloudflareDNSRecords(call: PluginCall) {
        runAsync(call) { service.pushCloudflareDNSRecords(call.data.toString()) }
    }

    @PluginMethod
    fun LoadSchedulerStatus(call: PluginCall) {
        runAsync(call) { service.loadSchedulerStatus() }
    }

    @PluginMethod
    fun RefreshScheduler(call: PluginCall) {
        runAsync(call) { SchedulerWorker.refresh(context) }
    }

    @PluginMethod
    fun RunScheduledProbe(call: PluginCall) {
        runAsync(call) { service.runScheduledProbe(call.data.toString()) }
    }

    @PluginMethod
    fun TestGitHubExport(call: PluginCall) {
        runAsync(call) { service.testGitHubExport(call.data.toString()) }
    }

    @PluginMethod
    fun ExportResultsToGitHub(call: PluginCall) {
        runAsync(call) { service.exportResultsToGitHub(call.data.toString()) }
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
            call.resolve(command("PATH_SELECTION_DEPRECATED", data, "当前版本不再支持自定义储存目录，Android 固定使用应用私有目录。", true))
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

    class LegacyMirrorMigrationResult {
        @JvmField
        var attempted = false

        @JvmField
        var completed = false

        @JvmField
        val copied: MutableList<String> = ArrayList()

        @JvmField
        val failed: MutableList<String> = ArrayList()

        @JvmField
        val skipped: MutableList<String> = ArrayList()

        constructor()

        constructor(source: AndroidStorageMigration.LegacyMirrorMigrationResult) {
            attempted = source.attempted
            completed = source.completed
            copied.addAll(source.copied)
            failed.addAll(source.failed)
            skipped.addAll(source.skipped)
        }
    }

    private fun initializeServiceFromStorage(): String {
        val bootstrap = AndroidStorageState.readStorageBootstrap(context)
        val runtimeDir = AndroidStorageState.resolveRuntimeDirectory(context, bootstrap)
        runtimeDirPath = runtimeDir
        val response = service.init(runtimeDir)
        startLogMonitorIfConfigured()
        return response
    }

    private fun finalizeLoadConfigResponse(responseJSON: String): String {
        return AndroidPluginCommands.finalizeLoadConfigResponse(context, responseJSON)
    }

    private fun finalizeServiceResponse(responseJSON: String, syncAfterWrite: Boolean): String {
        return AndroidPluginCommands.finalizeServiceResponse(context, responseJSON)
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

    private fun openBatteryOptimizationSettings(mode: String?) {
        AndroidBatterySettings.openSettings(context, mode)
    }

    private fun startKeepAliveIfAllowed() {
        AndroidKeepAliveState.startIfAllowed(context)
    }

    private fun startLogMonitorIfConfigured() {
        val runtimeDir = runtimeDirPath.trim()
        if (runtimeDir.isEmpty()) {
            return
        }
        LogMonitorForegroundService.startIfConfigured(context, runtimeDir)
    }

    private fun isProbeForegroundServiceRunning(): Boolean {
        return ProbeForegroundService.isForegroundRunning()
    }

    private fun augmentProbeEvent(eventJSON: String): String {
        return eventJSON
    }

    private fun defaultRuntimeDir(): File {
        return defaultRuntimeDirStatic(context)
    }

    private fun runAsync(call: PluginCall, syncAfterWrite: Boolean = false, action: () -> String) {
        executor.execute {
            try {
                call.resolve(JSObject(finalizeServiceResponse(action(), syncAfterWrite)))
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

    private fun command(code: String, data: JSObject, message: String, ok: Boolean): JSObject {
        return AndroidPluginCommands.command(code, data, message, ok)
    }

    private fun commandJSON(code: String, data: JSObject, message: String, ok: Boolean): String {
        return AndroidPluginCommands.commandJSON(code, data, message, ok)
    }

    private fun appVersion(): String {
        return AndroidAppInfo.appVersion(context)
    }

    companion object {
        private const val TAG = "CfstPlugin"
        const val EXPORT_DIRECTORY_PERMISSION_LOST_MESSAGE = "Android 未持有所选导出目录的持久化权限，请重新选择导出目录。"
        const val EXPORT_DIRECTORY_OPEN_ERROR_MESSAGE = "系统无法打开该导出目录，请安装或启用文件管理器后重试。"

        @JvmStatic
        fun defaultRuntimeDirStatic(context: Context): File {
            return AndroidStorageState.defaultRuntimeDir(context)
        }

        @JvmStatic
        fun migrateLegacySafMirrorFiles(mirrorDir: File?, targetDir: File?): LegacyMirrorMigrationResult {
            return LegacyMirrorMigrationResult(AndroidStorageMigration.migrateLegacySafMirrorFiles(mirrorDir, targetDir))
        }

        @JvmStatic
        fun requireExportTreeUriPermission(hasPermission: Boolean) {
            AndroidTargetOpener.requireExportTreeUriPermission(hasPermission)
        }

        @JvmStatic
        fun nowRFC3339UTC(): String {
            return AndroidStorageState.nowRFC3339UTC()
        }

        @JvmStatic
        fun cleanupAndroidUpdatePackages(updateDir: File?): Int {
            return AndroidUpdatePackages.cleanup(updateDir)
        }

        @JvmStatic
        fun cleanupAndroidUpdateDownloads(context: Context): Int {
            return AndroidUpdateInstaller.cleanupDownloadedPackages(context)
        }

        @JvmStatic
        fun isAndroidUpdatePackageFile(name: String?): Boolean {
            return AndroidUpdatePackages.isUpdatePackageFile(name)
        }

        @JvmStatic
        fun copyProbeExportToURIStatic(context: Context, responseJSON: String, exportURI: String?): String {
            return AndroidStorageBridge.copyProbeExportToURI(context, responseJSON, exportURI)
        }
    }
}
