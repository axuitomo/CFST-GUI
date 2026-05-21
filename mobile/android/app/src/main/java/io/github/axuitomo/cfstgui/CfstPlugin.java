package io.github.axuitomo.cfstgui;

import android.app.Activity;
import android.app.ActivityManager;
import android.content.ActivityNotFoundException;
import android.content.Context;
import android.content.Intent;
import android.content.UriPermission;
import android.content.pm.PackageInfo;
import android.database.Cursor;
import android.net.Uri;
import android.os.Build;
import android.os.PowerManager;
import android.provider.DocumentsContract;
import android.provider.Settings;
import android.provider.OpenableColumns;
import android.util.Base64;
import android.util.Log;
import androidx.activity.result.ActivityResult;
import androidx.core.content.FileProvider;
import com.getcapacitor.JSObject;
import com.getcapacitor.Plugin;
import com.getcapacitor.PluginCall;
import com.getcapacitor.PluginMethod;
import com.getcapacitor.annotation.ActivityCallback;
import com.getcapacitor.annotation.CapacitorPlugin;
import java.io.ByteArrayOutputStream;
import java.io.File;
import java.io.FileInputStream;
import java.io.FileOutputStream;
import java.io.InputStream;
import java.io.OutputStream;
import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.text.SimpleDateFormat;
import java.util.ArrayList;
import java.util.Date;
import java.util.List;
import java.util.Locale;
import java.util.TimeZone;
import java.util.concurrent.Callable;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import org.json.JSONArray;
import org.json.JSONException;
import org.json.JSONObject;

@CapacitorPlugin(name = "Cfst")
public class CfstPlugin extends Plugin {
    private static final String TAG = "CfstPlugin";
    private static final String LATEST_RELEASE_API = "https://api.github.com/repos/axuitomo/CFST-GUI/releases/latest";
    private static final String RELEASE_PAGE_URL = "https://github.com/axuitomo/CFST-GUI/releases/latest";
    static final String EXPORT_DIRECTORY_PERMISSION_LOST_MESSAGE = "Android 未持有所选导出目录的持久化权限，请重新选择导出目录。";
    static final String EXPORT_DIRECTORY_OPEN_ERROR_MESSAGE = "系统无法打开该导出目录，请安装或启用文件管理器后重试。";
    private static final String STORAGE_BACKEND_PRIVATE = "private";
    private static final String STORAGE_BACKEND_SAF_MIRROR = "saf_mirror";
    private static final String STORAGE_BOOTSTRAP_SCHEMA = "cfst-gui-storage-v2";
    private static final String LEGACY_BACKEND_FIELD = "_legacy_backend";
    private static final String LEGACY_STORAGE_URI_FIELD = "_legacy_storage_uri";
    private static final String LEGACY_MIRROR_MIGRATION_ATTEMPTED = "legacy_storage_mirror_migration_attempted";
    private static final String LEGACY_MIRROR_MIGRATION_COMPLETED = "legacy_storage_mirror_migration_completed";
    private static final String LEGACY_MIRROR_MIGRATION_ERROR = "legacy_storage_mirror_migration_error";
    private static final String[] LEGACY_MIRROR_ROOT_FILES = new String[] {
        "cfip-log.txt",
        "cloudflare-colo-locations.json",
        "cloudflare-colos-ipv4.csv",
        "cloudflare-colos-ipv6.csv",
        "cloudflare-colos.csv",
        "cloudflare-countries.json",
        "local-ip-ranges.csv",
        "mobile-config.json",
        "profiles.json",
        "source-profiles.json"
    };
    private static final String[] LEGACY_MIRROR_ROOT_DIRECTORIES = new String[] { "backups", "exports", "imports", "tasks" };
    private static final String GHPROXY_GITHUB_PREFIX = "https://ghproxy.com/";
    private static final String KKGITHUB_HOST = "kkgithub.com";
    private final ExecutorService executor = Executors.newSingleThreadExecutor();
    private mobileapi.Service service;

    @Override
    public void load() {
        CfstRuntime.ProbeEventListener sink = new CfstRuntime.ProbeEventListener() {
            @Override
            public void onProbeEvent(String eventJSON) {
                try {
                    notifyListeners("desktop:probe", new JSObject(augmentProbeEvent(eventJSON)));
                } catch (Exception error) {
                    logPluginError("Failed to augment probe event, retrying with raw payload.", error);
                    try {
                        notifyListeners("desktop:probe", new JSObject(eventJSON));
                    } catch (Exception rawError) {
                        logPluginError("Failed to dispatch raw probe event.", rawError);
                        JSObject fallback = new JSObject();
                        fallback.put("event", "probe.failed");
                        fallback.put("schema_version", "cfst-gui-mobile-v1");
                        fallback.put("task_id", "");
                        fallback.put("seq", 0);
                        fallback.put("ts", "");
                        JSObject payload = new JSObject();
                        payload.put("bridge_error", error.getMessage());
                        payload.put("message", "Android 原生事件桥接失败：" + rawError.getMessage());
                        fallback.put("payload", payload);
                        notifyListeners("desktop:probe", fallback);
                    }
                }
            }
        };
        try {
            String runtimeDir = resolveRuntimeDirectory(readStorageBootstrap());
            CfstRuntime.setPluginListener(sink);
            CfstRuntime.ensureInitialized(getContext(), runtimeDir);
            service = CfstRuntime.service();
        } catch (Exception error) {
            logPluginError("Failed to initialize storage-backed runtime directory, falling back to default private storage.", error);
            CfstRuntime.setPluginListener(sink);
            CfstRuntime.ensureInitialized(getContext(), defaultRuntimeDir().getAbsolutePath());
            service = CfstRuntime.service();
        }
    }

    @PluginMethod
    public void Init(PluginCall call) {
        runAsync(call, this::initializeServiceFromStorage);
    }

    @PluginMethod
    public void LoadConfig(PluginCall call) {
        executor.execute(() -> {
            try {
                call.resolve(new JSObject(finalizeLoadConfigResponse(service.loadConfig())));
            } catch (Exception error) {
                rejectWithLog(call, "LoadConfig", error);
            }
        });
    }

    @PluginMethod
    public void GetAppInfo(PluginCall call) {
        JSObject data = new JSObject();
        data.put("current_version", appVersion());
        data.put("install_mode", "android_apk");
        data.put("platform", "android");
        data.put("release_url", RELEASE_PAGE_URL);
        data.put("battery_optimization_supported", Build.VERSION.SDK_INT >= Build.VERSION_CODES.M);
        call.resolve(command("APP_INFO_READY", data, "应用信息已读取。", true));
    }

    @PluginMethod
    public void GetAndroidRuntimeStatus(PluginCall call) {
        executor.execute(() -> {
            try {
                call.resolve(new JSObject(commandJSON("ANDROID_RUNTIME_STATUS", androidRuntimeStatusPayload(), "Android 运行时状态已读取。", true)));
            } catch (Exception error) {
                rejectWithLog(call, "GetAndroidRuntimeStatus", error);
            }
        });
    }

    @PluginMethod
    public void CheckBatteryOptimization(PluginCall call) {
        executor.execute(() -> {
            try {
                call.resolve(new JSObject(commandJSON("ANDROID_BATTERY_STATUS", batteryOptimizationPayload(), "省电策略状态已读取。", true)));
            } catch (Exception error) {
                rejectWithLog(call, "CheckBatteryOptimization", error);
            }
        });
    }

    @PluginMethod
    public void OpenBatteryOptimizationSettings(PluginCall call) {
        executor.execute(() -> {
            try {
                String mode = call.getString("mode", "request");
                openBatteryOptimizationSettings(mode);
                JSObject data = batteryOptimizationPayload();
                data.put("mode", mode == null ? "" : mode.trim());
                call.resolve(command("ANDROID_BATTERY_SETTINGS_OPENED", data, "已打开 Android 省电策略设置。", true));
            } catch (Exception error) {
                rejectWithLog(call, "OpenBatteryOptimizationSettings", error);
            }
        });
    }

    @PluginMethod
    public void CheckForUpdates(PluginCall call) {
        runAsync(call, () -> commandJSON("UPDATE_CHECK_OK", checkForUpdatesPayload(), "更新检查完成。", true));
    }

    @PluginMethod
    public void DownloadAndInstallUpdate(PluginCall call) {
        executor.execute(() -> {
            try {
                JSObject info = checkForUpdatesPayload();
                if (!Boolean.TRUE.equals(info.getBoolean("update_available", false))) {
                    call.resolve(command("UPDATE_NOT_AVAILABLE", info, "当前已是最新版本。", true));
                    return;
                }
                String assetName = info.getString("asset_name", "cfst-gui-android-release.apk");
                File updateDir = new File(getContext().getFilesDir(), "updates");
                if (!updateDir.exists() && !updateDir.mkdirs()) {
                    throw new IllegalStateException("创建更新目录失败：" + updateDir.getAbsolutePath());
                }
                File apk = new File(updateDir, assetName);
                downloadURLToFile(info.getString("download_url", ""), apk);
                String expectedSHA256 = info.getString("sha256", "");
                if (!expectedSHA256.isEmpty()) {
                    verifySHA256(apk, expectedSHA256);
                }
                triggerAPKInstall(apk);
                info.put("downloaded_path", apk.getAbsolutePath());
                info.put("install_started", true);
                info.put("next_action", "android_install_confirmation");
                call.resolve(command("UPDATE_INSTALL_READY", info, "APK 已下载，请在系统安装确认中继续。", true));
            } catch (Exception error) {
                call.reject(error.getMessage(), error);
            }
        });
    }

    @PluginMethod
    public void OpenReleasePage(PluginCall call) {
        try {
            Intent intent = new Intent(Intent.ACTION_VIEW, Uri.parse(RELEASE_PAGE_URL));
            intent.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK);
            getContext().startActivity(intent);
            JSObject data = new JSObject();
            data.put("release_url", RELEASE_PAGE_URL);
            call.resolve(command("RELEASE_OPENED", data, "已打开发行页。", true));
        } catch (Exception error) {
            call.reject(error.getMessage(), error);
        }
    }

    @PluginMethod
    public void SaveConfig(PluginCall call) {
        runAsync(call, () -> service.saveConfig(call.getData().toString()), true);
    }

    @PluginMethod
    public void SetStorageDirectory(PluginCall call) {
        executor.execute(() -> {
            try {
                call.resolve(new JSObject(applyStorageDirectoryChange(call.getData())));
            } catch (Exception error) {
                call.reject(error.getMessage(), error);
            }
        });
    }

    @PluginMethod
    public void CheckStorageHealth(PluginCall call) {
        runAsync(call, () -> service.checkStorageHealth(call.getData().toString()));
    }

    @PluginMethod
    public void ExportConfig(PluginCall call) {
        String payload = call.getData().toString();
        executor.execute(() -> {
            try {
                String targetURI = extractTargetURI(payload);
                String response = service.exportConfig(payload);
                if (!targetURI.isEmpty()) {
                    response = writeConfigExportToURI(response, targetURI);
                }
                call.resolve(new JSObject(finalizeServiceResponse(response, false)));
            } catch (Exception error) {
                call.reject(error.getMessage(), error);
            }
        });
    }

    @PluginMethod
    public void BackupCurrentConfig(PluginCall call) {
        runAsync(call, () -> service.backupCurrentConfig(call.getData().toString()), true);
    }

    @PluginMethod
    public void ExportConfigArchive(PluginCall call) {
        String payload = call.getData().toString();
        executor.execute(() -> {
            try {
                String targetURI = extractTargetURI(payload);
                String response = service.exportConfigArchive(payload);
                if (!targetURI.isEmpty()) {
                    response = writeConfigArchiveToURI(response, targetURI);
                }
                call.resolve(new JSObject(finalizeServiceResponse(response, false)));
            } catch (Exception error) {
                call.reject(error.getMessage(), error);
            }
        });
    }

    @PluginMethod
    public void ExportResultsCSV(PluginCall call) {
        String payload = call.getData().toString();
        executor.execute(() -> {
            try {
                String targetURI = extractTargetURI(payload);
                String response = service.exportResultsCSV(payload);
                if (!targetURI.isEmpty()) {
                    response = writeCSVExportToURI(response, targetURI);
                }
                call.resolve(new JSObject(finalizeServiceResponse(response, false)));
            } catch (Exception error) {
                call.reject(error.getMessage(), error);
            }
        });
    }

    @PluginMethod
    public void ExportDebugLog(PluginCall call) {
        String payload = call.getData().toString();
        executor.execute(() -> {
            try {
                String targetURI = extractTargetURI(payload);
                String response = invokeServiceString("exportDebugLog", payload);
                if (!targetURI.isEmpty()) {
                    response = writeDebugLogExportToURI(response, targetURI);
                }
                call.resolve(new JSObject(finalizeServiceResponse(response, false)));
            } catch (Exception error) {
                call.reject(error.getMessage(), error);
            }
        });
    }

    @PluginMethod
    public void ImportConfigArchive(PluginCall call) {
        runAsync(call, () -> service.importConfigArchive(call.getData().toString()), true);
    }

    @PluginMethod
    public void TestWebDAV(PluginCall call) {
        runAsync(call, () -> service.testWebDAV(call.getData().toString()));
    }

    @PluginMethod
    public void BackupConfigToWebDAV(PluginCall call) {
        runAsync(call, () -> service.backupConfigToWebDAV(call.getData().toString()));
    }

    @PluginMethod
    public void RestoreConfigFromWebDAV(PluginCall call) {
        runAsync(call, () -> service.restoreConfigFromWebDAV(call.getData().toString()), true);
    }

    @PluginMethod
    public void LoadProfiles(PluginCall call) {
        runAsync(call, () -> service.loadProfiles());
    }

    @PluginMethod
    public void SaveCurrentProfile(PluginCall call) {
        runAsync(call, () -> service.saveCurrentProfile(call.getData().toString()), true);
    }

    @PluginMethod
    public void UpdateCurrentProfile(PluginCall call) {
        runAsync(call, () -> service.updateCurrentProfile(call.getData().toString()), true);
    }

    @PluginMethod
    public void SwitchProfile(PluginCall call) {
        runAsync(call, () -> service.switchProfile(call.getData().toString()), true);
    }

    @PluginMethod
    public void DeleteProfile(PluginCall call) {
        runAsync(call, () -> service.deleteProfile(call.getData().toString()), true);
    }

    @PluginMethod
    public void LoadSourceProfiles(PluginCall call) {
        runAsync(call, () -> service.loadSourceProfiles());
    }

    @PluginMethod
    public void SaveSourceProfile(PluginCall call) {
        runAsync(call, () -> service.saveSourceProfile(call.getData().toString()), true);
    }

    @PluginMethod
    public void UpdateCurrentSourceProfile(PluginCall call) {
        runAsync(call, () -> service.updateCurrentSourceProfile(call.getData().toString()), true);
    }

    @PluginMethod
    public void SaveSourceProfileStore(PluginCall call) {
        runAsync(call, () -> service.saveSourceProfileStore(call.getData().toString()), true);
    }

    @PluginMethod
    public void SwitchSourceProfile(PluginCall call) {
        runAsync(call, () -> service.switchSourceProfile(call.getData().toString()), true);
    }

    @PluginMethod
    public void DeleteSourceProfile(PluginCall call) {
        runAsync(call, () -> service.deleteSourceProfile(call.getData().toString()), true);
    }

    @PluginMethod
    public void PreviewSource(PluginCall call) {
        runAsync(call, () -> service.previewSource(call.getData().toString()));
    }

    @PluginMethod
    public void FetchSource(PluginCall call) {
        runAsync(call, () -> service.fetchSource(call.getData().toString()));
    }

    @PluginMethod
    public void LoadColoDictionaryStatus(PluginCall call) {
        runAsync(call, () -> service.loadColoDictionaryStatus());
    }

    @PluginMethod
    public void UpdateColoDictionary(PluginCall call) {
        runAsync(call, () -> service.updateColoDictionary(call.getData().toString()), true);
    }

    @PluginMethod
    public void ProcessColoDictionary(PluginCall call) {
        runAsync(call, () -> service.processColoDictionary(call.getData().toString()), true);
    }

    @PluginMethod
    public void RunProbe(PluginCall call) {
        String payload = call.getData().toString();
        try {
            if (!ProbeForegroundService.markStartQueuedIfIdle()) {
                JSObject data = new JSObject();
                data.put("accepted", false);
                data.put("task_id", call.getString("task_id", ""));
                call.resolve(command("PROBE_ALREADY_RUNNING", data, "当前已有探测任务运行或暂停，请完成后再启动新任务。", false));
                return;
            }
            String exportURI = extractExportTargetURI(payload);
            AndroidStorageBridge.ensureWritablePersistentExportTarget(getContext(), exportURI);
            String normalizedPayload = withAndroidExportURI(payload, exportURI);
            Intent serviceIntent = ProbeForegroundService.startIntent(getContext(), normalizedPayload, exportURI);
            if (android.os.Build.VERSION.SDK_INT >= android.os.Build.VERSION_CODES.O) {
                getContext().startForegroundService(serviceIntent);
            } else {
                getContext().startService(serviceIntent);
            }
            JSObject data = new JSObject();
            data.put("accepted", true);
            data.put("export_path", exportURI);
            data.put("task_id", call.getString("task_id", ""));
            call.resolve(command("PROBE_ACCEPTED", data, "移动端探测任务已提交到前台服务。", true));
        } catch (Exception error) {
            ProbeForegroundService.clearQueuedStart();
            rejectWithLog(call, "RunProbe", error);
        }
    }

    @PluginMethod
    public void CancelProbe(PluginCall call) {
        try {
            call.resolve(new JSObject(finalizeServiceResponse(service.cancelProbe(call.getData().toString()), false)));
        } catch (Exception error) {
            call.reject(error.getMessage(), error);
        }
    }

    @PluginMethod
    public void ResumeProbe(PluginCall call) {
        try {
            call.resolve(new JSObject(finalizeServiceResponse(service.resumeProbe(call.getData().toString()), false)));
        } catch (Exception error) {
            call.reject(error.getMessage(), error);
        }
    }

    @PluginMethod
    public void LoadTaskSnapshot(PluginCall call) {
        runAsync(call, () -> service.loadTaskSnapshot(call.getData().toString()));
    }

    @PluginMethod
    public void ListResultFile(PluginCall call) {
        String payload = call.getData().toString();
        executor.execute(() -> {
            try {
                call.resolve(new JSObject(finalizeServiceResponse(service.listResultFile(withPrivateResultFilePath(payload)), false)));
            } catch (Exception error) {
                call.reject(error.getMessage(), error);
            }
        });
    }

    @PluginMethod
    public void ListCloudflareDNSRecords(PluginCall call) {
        runAsync(call, () -> service.listCloudflareDNSRecords(call.getData().toString()));
    }

    @PluginMethod
    public void PushCloudflareDNSRecords(PluginCall call) {
        runAsync(call, () -> service.pushCloudflareDNSRecords(call.getData().toString()));
    }

    @PluginMethod
    public void LoadSchedulerStatus(PluginCall call) {
        runAsync(call, () -> service.loadSchedulerStatus());
    }

    @PluginMethod
    public void TestGitHubExport(PluginCall call) {
        runAsync(call, () -> service.testGitHubExport(call.getData().toString()));
    }

    @PluginMethod
    public void ExportResultsToGitHub(PluginCall call) {
        runAsync(call, () -> service.exportResultsToGitHub(call.getData().toString()));
    }

    @PluginMethod
    public void OpenPath(PluginCall call) {
        executor.execute(() -> {
            try {
                String targetPath = call.getString("targetPath", "");
                openTargetPath(targetPath);
                JSObject data = new JSObject();
                data.put("target_path", targetPath == null ? "" : targetPath.trim());
                call.resolve(command("OPEN_PATH_OK", data, "已打开目标。", true));
            } catch (Exception error) {
                call.reject(error.getMessage(), error);
            }
        });
    }

    @PluginMethod
    public void SelectPath(PluginCall call) {
        String mode = normalizePathSelectionMode(call.getString("mode", ""));
        Intent intent;
        if (isStorageDirMode(mode)) {
            JSObject data = new JSObject();
            data.put("canceled", false);
            data.put("mode", mode);
            data.put("path", defaultRuntimeDir().getAbsolutePath());
            data.put("directory", defaultRuntimeDir().getAbsolutePath());
            call.resolve(command("PATH_SELECTION_DEPRECATED", data, "当前版本不再支持自定义储存目录，Android 固定使用应用私有目录。", true));
            return;
        }
        if (isExportDirectoryMode(mode)) {
            intent = new Intent(Intent.ACTION_OPEN_DOCUMENT_TREE);
            putInitialURIExtra(intent, call);
        } else if (isExportFileMode(mode) || isConfigExportMode(mode) || isConfigArchiveExportMode(mode)) {
            String defaultFileName = call.getString("defaultFileName", call.getString("default_file_name", "result.csv"));
            if (defaultFileName == null || defaultFileName.trim().isEmpty()) {
                defaultFileName = isConfigArchiveExportMode(mode) ? "cfst-gui-config.zip" : isConfigExportMode(mode) ? "cfst-gui-config.json" : "result.csv";
            }
            intent = new Intent(Intent.ACTION_CREATE_DOCUMENT);
            intent.addCategory(Intent.CATEGORY_OPENABLE);
            intent.setType(isConfigArchiveExportMode(mode) ? "application/zip" : isConfigExportMode(mode) ? "application/json" : "text/csv");
            intent.putExtra(Intent.EXTRA_TITLE, defaultFileName);
        } else {
            intent = new Intent(Intent.ACTION_OPEN_DOCUMENT);
            intent.addCategory(Intent.CATEGORY_OPENABLE);
            intent.setType("*/*");
            if (isConfigArchiveImportMode(mode)) {
                intent.putExtra(Intent.EXTRA_MIME_TYPES, new String[] { "application/zip", "application/octet-stream", "application/json", "text/plain", "text/json" });
            } else if (isConfigImportMode(mode)) {
                intent.putExtra(Intent.EXTRA_MIME_TYPES, new String[] { "application/json", "text/plain", "text/json" });
            } else {
                intent.putExtra(Intent.EXTRA_MIME_TYPES, new String[] { "text/plain", "text/csv", "application/octet-stream", "*/*" });
            }
        }
        intent.addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION | Intent.FLAG_GRANT_WRITE_URI_PERMISSION | Intent.FLAG_GRANT_PERSISTABLE_URI_PERMISSION);
        startActivityForResult(call, intent, "handleSelectPathResult");
    }

    @ActivityCallback
    public void handleSelectPathResult(PluginCall call, ActivityResult result) {
        if (call == null) {
            return;
        }
        JSObject data = new JSObject();
        data.put("mode", normalizePathSelectionMode(call.getString("mode", "")));

        Intent resultData = result.getData();
        if (result.getResultCode() != Activity.RESULT_OK || resultData == null || resultData.getData() == null) {
            data.put("canceled", true);
            call.resolve(command("PATH_SELECTION_CANCELED", data, "已取消系统文件选择。", true));
            return;
        }

        Uri uri = resultData.getData();
        String mode = normalizePathSelectionMode(call.getString("mode", ""));
        String displayName = queryDisplayName(uri);
        data.put("canceled", false);
        data.put("display_name", displayName);
        data.put("uri", uri.toString());

        try {
            if (isExportDirectoryMode(mode)) {
                persistUriPermission(resultData, uri);
                data.put("target_uri", uri.toString());
                data.put("path", displayName.isEmpty() ? uri.toString() : displayName);
                call.resolve(command("PATH_SELECTED", data, "已选择导出目录。", true));
                return;
            }

            if (isExportFileMode(mode) || isConfigExportMode(mode) || isConfigArchiveExportMode(mode)) {
                data.put("target_uri", uri.toString());
                data.put("path", displayName.isEmpty() ? uri.toString() : displayName);
                call.resolve(command("PATH_SELECTED", data, isConfigExportMode(mode) || isConfigArchiveExportMode(mode) ? "已选择配置导出文件。" : "已选择导出文件。", true));
                return;
            }

            if (isConfigArchiveImportMode(mode)) {
                data.put("content_base64", readURIBase64(uri));
                data.put("path", displayName.isEmpty() ? uri.toString() : displayName);
                call.resolve(command("PATH_SELECTED", data, "已读取配置压缩包。", true));
                return;
            }

            if (isConfigImportMode(mode)) {
                data.put("content", readURIText(uri));
                data.put("path", displayName.isEmpty() ? uri.toString() : displayName);
                call.resolve(command("PATH_SELECTED", data, "已读取配置文件。", true));
                return;
            }

            File copied = copyURIToPrivateFile(uri, displayName);
            data.put("path", copied.getAbsolutePath());
            call.resolve(command("PATH_SELECTED", data, "已选择输入源文件。", true));
        } catch (Exception error) {
            call.reject(error.getMessage(), error);
        }
    }

    private static final class StorageSyncResult {
        final List<String> copied = new ArrayList<>();
        final List<String> failed = new ArrayList<>();
        final List<String> skipped = new ArrayList<>();

        JSObject toJSObject() {
            JSObject payload = new JSObject();
            payload.put("copied", new JSONArray(copied));
            payload.put("failed", new JSONArray(failed));
            payload.put("skipped", new JSONArray(skipped));
            return payload;
        }
    }

    static final class LegacyMirrorMigrationResult {
        boolean attempted;
        boolean completed;
        final List<String> copied = new ArrayList<>();
        final List<String> failed = new ArrayList<>();
        final List<String> skipped = new ArrayList<>();
    }

    private String initializeServiceFromStorage() throws Exception {
        JSONObject bootstrap = readStorageBootstrap();
        String runtimeDir = resolveRuntimeDirectory(bootstrap);
        return service.init(runtimeDir);
    }

    private String applyStorageDirectoryChange(JSObject payload) throws Exception {
        StorageSyncResult migration = new StorageSyncResult();
        JSONObject next = defaultStorageBootstrap();
        next.put("setup_completed", true);
        writeStorageBootstrap(next);
        service.init(defaultRuntimeDir().getAbsolutePath());
        return storageSetCommand(migration);
    }

    private String storageSetCommand(StorageSyncResult migration) throws Exception {
        JSObject data = new JSObject();
        data.put("migration", migration.toJSObject());
        data.put("storage", currentStorageStatus());
        return command("STORAGE_SET_DEPRECATED", data, "当前版本不再支持自定义储存目录，Android 固定使用应用私有目录。", true).toString();
    }

    private String finalizeLoadConfigResponse(String responseJSON) throws Exception {
        return finalizeServiceResponse(responseJSON, false);
    }

    private String finalizeServiceResponse(String responseJSON, boolean syncAfterWrite) throws Exception {
        JSONObject command = new JSONObject(responseJSON);
        try {
            attachStorageState(command);
        } catch (Exception error) {
            logPluginError("Failed to attach Android storage state to plugin response.", error);
            appendWarning(command, command.optJSONObject("data"), "Android 储存状态附加失败：" + error.getMessage());
        }
        return command.toString();
    }

    private JSObject androidRuntimeStatusPayload() throws Exception {
        JSObject data = new JSObject();
        JSONObject snapshotCommand = new JSONObject(service.loadTaskSnapshot("{}"));
        boolean hasTaskSnapshot = snapshotCommand.optBoolean("ok", false) && snapshotCommand.optJSONObject("data") != null;
        JSONObject snapshot = hasTaskSnapshot ? snapshotCommand.optJSONObject("data") : null;
        String taskId = snapshot == null ? "" : snapshot.optString("task_id", "");
        String sessionState = snapshot == null ? "" : snapshot.optString("session_state", "");
        boolean runtimeAttached = snapshot != null && snapshot.optBoolean("runtime_attached", false);
        boolean resumeCapable = snapshot != null && snapshot.optBoolean("resume_capable", false);
        boolean serviceRunning = isProbeForegroundServiceRunning();
        data.put("foreground_service_running", serviceRunning);
        data.put("has_task_snapshot", hasTaskSnapshot);
        data.put("resume_capable", resumeCapable);
        data.put("runtime_attached", runtimeAttached);
        data.put("session_state", sessionState);
        data.put("task_id", taskId);
        if (snapshot != null) {
            data.put("task_snapshot", snapshot);
        }
        data.put("battery", batteryOptimizationPayload());
        return data;
    }

    private JSObject batteryOptimizationPayload() {
        JSObject data = new JSObject();
        boolean supported = Build.VERSION.SDK_INT >= Build.VERSION_CODES.M;
        boolean ignoring = false;
        try {
            if (supported) {
                PowerManager powerManager = (PowerManager) getContext().getSystemService(android.content.Context.POWER_SERVICE);
                ignoring = powerManager != null && powerManager.isIgnoringBatteryOptimizations(getContext().getPackageName());
            }
        } catch (Exception error) {
            logPluginError("Failed to read battery optimization state.", error);
        }
        data.put("supported", supported);
        data.put("ignoring_optimizations", ignoring);
        data.put("manufacturer", Build.MANUFACTURER == null ? "" : Build.MANUFACTURER.trim());
        data.put("brand", Build.BRAND == null ? "" : Build.BRAND.trim());
        data.put("model", Build.MODEL == null ? "" : Build.MODEL.trim());
        data.put("needs_guidance", supported && !ignoring);
        data.put("settings_hint", manufacturerBatteryHint());
        return data;
    }

    private void openBatteryOptimizationSettings(String mode) {
        String normalized = mode == null ? "request" : mode.trim().toLowerCase(Locale.ROOT);
        Intent intent;
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.M && "request".equals(normalized)) {
            intent = new Intent(Settings.ACTION_REQUEST_IGNORE_BATTERY_OPTIMIZATIONS);
            intent.setData(Uri.parse("package:" + getContext().getPackageName()));
        } else if ("details".equals(normalized)) {
            intent = new Intent(Settings.ACTION_APPLICATION_DETAILS_SETTINGS);
            intent.setData(Uri.parse("package:" + getContext().getPackageName()));
        } else {
            intent = new Intent(Settings.ACTION_IGNORE_BATTERY_OPTIMIZATION_SETTINGS);
        }
        intent.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK);
        startExternalIntent(intent, "系统无法打开省电策略设置。");
    }

    private boolean isProbeForegroundServiceRunning() {
        try {
            ActivityManager manager = (ActivityManager) getContext().getSystemService(android.content.Context.ACTIVITY_SERVICE);
            if (manager == null) {
                return false;
            }
            for (ActivityManager.RunningServiceInfo info : manager.getRunningServices(Integer.MAX_VALUE)) {
                if (info.service != null && ProbeForegroundService.class.getName().equals(info.service.getClassName())) {
                    return true;
                }
            }
        } catch (Exception error) {
            logPluginError("Failed to inspect foreground service state.", error);
        }
        return false;
    }

    private String manufacturerBatteryHint() {
        String manufacturer = Build.MANUFACTURER == null ? "" : Build.MANUFACTURER.trim().toLowerCase(Locale.ROOT);
        if (manufacturer.contains("xiaomi") || manufacturer.contains("redmi") || manufacturer.contains("poco")) {
            return "MIUI/HyperOS 常见于“省电策略”“后台弹出界面”“自启动管理”，建议同时放行。";
        }
        if (manufacturer.contains("huawei") || manufacturer.contains("honor")) {
            return "华为/荣耀常见于“启动管理”“应用启动”“电池优化”，建议允许后台活动。";
        }
        if (manufacturer.contains("oppo") || manufacturer.contains("oneplus") || manufacturer.contains("realme")) {
            return "OPPO/OnePlus/realme 常见于“自动启动”“后台冻结”“耗电保护”，建议关闭限制。";
        }
        if (manufacturer.contains("vivo") || manufacturer.contains("iqoo")) {
            return "vivo/iQOO 常见于“后台高耗电”“自启动管理”，建议允许后台运行。";
        }
        if (manufacturer.contains("samsung")) {
            return "Samsung 常见于“电池-后台使用限制”“未使用应用休眠”，建议加入永不休眠。";
        }
        return "若系统仍会回收后台任务，请同时检查厂商自启动、后台冻结和电池优化设置。";
    }

    private void attachStorageState(JSONObject command) throws Exception {
        JSONObject data = command.optJSONObject("data");
        if (data == null) {
            return;
        }
        data.put("storage", currentStorageStatus());
    }

    private String augmentProbeEvent(String eventJSON) throws Exception {
        return eventJSON;
    }

    private JSObject currentStorageStatus() throws Exception {
        JSONObject bootstrap = readStorageBootstrap();
        File runtimeDir = defaultRuntimeDir();
        ensureDirectory(runtimeDir);

        JSObject health = new JSObject();
        health.put("checked_at", nowRFC3339());
        health.put("exists", runtimeDir.exists());
        health.put("free_bytes", -1);
        health.put("is_dir", runtimeDir.isDirectory());
        health.put("message", healthMessage(runtimeDir));
        health.put("path", runtimeDir.getAbsolutePath());
        health.put("portable_mode", false);
        health.put("writable", runtimeDir.canWrite());
        String migrationError = bootstrap.optString(LEGACY_MIRROR_MIGRATION_ERROR, "").trim();
        if (!migrationError.isEmpty()) {
            health.put("message", "旧 Android SAF mirror 迁移失败：" + migrationError);
        }

        JSObject status = new JSObject();
        status.put("backend", STORAGE_BACKEND_PRIVATE);
        status.put("bootstrap_path", storageBootstrapFile().getAbsolutePath());
        status.put("current_dir", defaultRuntimeDir().getAbsolutePath());
        status.put("default_dir", defaultRuntimeDir().getAbsolutePath());
        status.put("display_name", "");
        status.put("health", health);
        status.put("last_sync_at", "");
        status.put("last_sync_error", "");
        status.put("legacy_storage_mirror_migration_attempted", bootstrap.optBoolean(LEGACY_MIRROR_MIGRATION_ATTEMPTED, false));
        status.put("legacy_storage_mirror_migration_completed", bootstrap.optBoolean(LEGACY_MIRROR_MIGRATION_COMPLETED, false));
        status.put("legacy_storage_mirror_migration_error", migrationError);
        status.put("log_uri", "");
        status.put("permission_ok", true);
        status.put("portable_mode", false);
        status.put("runtime_dir", runtimeDir.getAbsolutePath());
        status.put("setup_completed", bootstrap.optBoolean("setup_completed", true));
        status.put("setup_required", false);
        status.put("storage_uri", "");
        status.put("writable", runtimeDir.canWrite());
        return status;
    }

    private String healthMessage(File runtimeDir) {
        if (!runtimeDir.exists()) {
            return "应用私有数据目录尚未创建。";
        }
        return "应用私有数据目录可用。";
    }

    private String resolveRuntimeDirectory(JSONObject bootstrap) throws Exception {
        File defaultDir = defaultRuntimeDir();
        ensureDirectory(defaultDir);
        LegacyMirrorMigrationResult migration = migrateLegacySafMirrorIfNeeded(bootstrap, storageMirrorDir(), defaultDir);
        if (migration.attempted) {
            bootstrap.put(LEGACY_MIRROR_MIGRATION_ATTEMPTED, true);
            bootstrap.put(LEGACY_MIRROR_MIGRATION_COMPLETED, migration.completed);
            bootstrap.put(LEGACY_MIRROR_MIGRATION_ERROR, joinMessages(migration.failed));
        }
        bootstrap.put("backend", STORAGE_BACKEND_PRIVATE);
        bootstrap.put("last_sync_at", "");
        bootstrap.put("last_sync_error", "");
        bootstrap.put("permission_ok", true);
        if (!bootstrap.has("setup_completed")) {
            bootstrap.put("setup_completed", true);
        }
        bootstrap.put("storage_dir", defaultDir.getAbsolutePath());
        bootstrap.put("storage_uri", "");
        writeStorageBootstrap(bootstrap);
        return defaultDir.getAbsolutePath();
    }

    private JSONObject defaultStorageBootstrap() throws JSONException {
        JSONObject bootstrap = new JSONObject();
        bootstrap.put("backend", STORAGE_BACKEND_PRIVATE);
        bootstrap.put("display_name", "");
        bootstrap.put("last_sync_at", "");
        bootstrap.put("last_sync_error", "");
        bootstrap.put("permission_ok", true);
        bootstrap.put("portable_mode", false);
        bootstrap.put("schema_version", STORAGE_BOOTSTRAP_SCHEMA);
        bootstrap.put("setup_completed", true);
        bootstrap.put("storage_dir", defaultRuntimeDir().getAbsolutePath());
        bootstrap.put("storage_uri", "");
        bootstrap.put("updated_at", nowRFC3339());
        return bootstrap;
    }

    private JSONObject readStorageBootstrap() throws Exception {
        File file = storageBootstrapFile();
        if (!file.exists()) {
            JSONObject bootstrap = defaultStorageBootstrap();
            writeStorageBootstrap(bootstrap);
            return bootstrap;
        }
        try (InputStream input = new FileInputStream(file); ByteArrayOutputStream output = new ByteArrayOutputStream()) {
            copy(input, output);
            JSONObject source = new JSONObject(output.toString(StandardCharsets.UTF_8.name()));
            JSONObject normalized = normalizeStorageBootstrap(source);
            normalized.put(LEGACY_BACKEND_FIELD, source.optString("backend", source.optString("storage_backend", "")));
            normalized.put(LEGACY_STORAGE_URI_FIELD, source.optString("storage_uri", source.optString("storageUri", "")));
            return normalized;
        }
    }

    private JSONObject normalizeStorageBootstrap(JSONObject source) throws Exception {
        JSONObject bootstrap = defaultStorageBootstrap();
        bootstrap.put("backend", STORAGE_BACKEND_PRIVATE);
        bootstrap.put("display_name", source.optString("display_name", source.optString("displayName", "")));
        bootstrap.put("last_sync_at", source.optString("last_sync_at", source.optString("lastSyncAt", "")));
        bootstrap.put("last_sync_error", source.optString("last_sync_error", source.optString("lastSyncError", "")));
        bootstrap.put("permission_ok", source.optBoolean("permission_ok", source.optBoolean("permissionOk", true)));
        bootstrap.put("portable_mode", source.optBoolean("portable_mode", source.optBoolean("portableMode", false)));
        bootstrap.put("schema_version", STORAGE_BOOTSTRAP_SCHEMA);
        bootstrap.put("setup_completed", source.optBoolean("setup_completed", source.optBoolean("setupCompleted", true)));
        bootstrap.put("storage_dir", defaultRuntimeDir().getAbsolutePath());
        bootstrap.put("storage_uri", "");
        if (source.optBoolean(LEGACY_MIRROR_MIGRATION_ATTEMPTED, false)) {
            bootstrap.put(LEGACY_MIRROR_MIGRATION_ATTEMPTED, true);
        }
        if (source.optBoolean(LEGACY_MIRROR_MIGRATION_COMPLETED, false)) {
            bootstrap.put(LEGACY_MIRROR_MIGRATION_COMPLETED, true);
        }
        String migrationError = source.optString(LEGACY_MIRROR_MIGRATION_ERROR, "").trim();
        if (!migrationError.isEmpty()) {
            bootstrap.put(LEGACY_MIRROR_MIGRATION_ERROR, migrationError);
        }
        bootstrap.put("updated_at", source.optString("updated_at", source.optString("updatedAt", nowRFC3339())));
        return bootstrap;
    }

    private void writeStorageBootstrap(JSONObject bootstrap) throws Exception {
        JSONObject normalized = normalizeStorageBootstrap(bootstrap);
        normalized.put("updated_at", nowRFC3339());
        File target = storageBootstrapFile();
        File parent = target.getParentFile();
        if (parent != null && !parent.exists() && !parent.mkdirs()) {
            throw new IllegalStateException("创建储存引导目录失败：" + parent.getAbsolutePath());
        }
        try (OutputStream output = new FileOutputStream(target)) {
            output.write(normalized.toString(2).getBytes(StandardCharsets.UTF_8));
        }
    }

    private File storageBootstrapFile() {
        return new File(getContext().getFilesDir(), "storage-bootstrap.json");
    }

    private File defaultRuntimeDir() {
        return getContext().getFilesDir();
    }

    private File storageMirrorDir() {
        return new File(getContext().getFilesDir(), "storage-mirror");
    }

    private LegacyMirrorMigrationResult migrateLegacySafMirrorIfNeeded(JSONObject bootstrap, File mirrorDir, File targetDir) {
        if (bootstrap.optBoolean(LEGACY_MIRROR_MIGRATION_COMPLETED, false)) {
            return new LegacyMirrorMigrationResult();
        }
        String legacyBackend = bootstrap.optString(LEGACY_BACKEND_FIELD, "").trim();
        String legacyStorageURI = bootstrap.optString(LEGACY_STORAGE_URI_FIELD, "").trim();
        if (!STORAGE_BACKEND_SAF_MIRROR.equals(legacyBackend) && legacyStorageURI.isEmpty() && !legacyMirrorHasKnownData(mirrorDir)) {
            return new LegacyMirrorMigrationResult();
        }
        return migrateLegacySafMirrorFiles(mirrorDir, targetDir);
    }

    static LegacyMirrorMigrationResult migrateLegacySafMirrorFiles(File mirrorDir, File targetDir) {
        LegacyMirrorMigrationResult result = new LegacyMirrorMigrationResult();
        if (mirrorDir == null || targetDir == null || !mirrorDir.isDirectory() || sameCanonicalFile(mirrorDir, targetDir) || !legacyMirrorHasKnownData(mirrorDir)) {
            return result;
        }
        result.attempted = true;
        if (!targetDir.exists() && !targetDir.mkdirs()) {
            result.failed.add("创建应用私有目录失败：" + targetDir.getAbsolutePath());
            result.completed = false;
            return result;
        }
        for (String name : LEGACY_MIRROR_ROOT_FILES) {
            copyLegacyMirrorEntry(new File(mirrorDir, name), new File(targetDir, name), name, result);
        }
        for (String name : LEGACY_MIRROR_ROOT_DIRECTORIES) {
            copyLegacyMirrorEntry(new File(mirrorDir, name), new File(targetDir, name), name, result);
        }
        result.completed = result.failed.isEmpty();
        return result;
    }

    private static boolean legacyMirrorHasKnownData(File mirrorDir) {
        if (mirrorDir == null || !mirrorDir.isDirectory()) {
            return false;
        }
        for (String name : LEGACY_MIRROR_ROOT_FILES) {
            if (new File(mirrorDir, name).exists()) {
                return true;
            }
        }
        for (String name : LEGACY_MIRROR_ROOT_DIRECTORIES) {
            if (new File(mirrorDir, name).exists()) {
                return true;
            }
        }
        return false;
    }

    private static void copyLegacyMirrorEntry(File source, File target, String relativePath, LegacyMirrorMigrationResult result) {
        if (!source.exists()) {
            return;
        }
        if (source.isDirectory()) {
            if (target.exists() && !target.isDirectory()) {
                result.skipped.add(relativePath);
                return;
            }
            if (!target.exists() && !target.mkdirs()) {
                result.failed.add(relativePath + ": 创建目录失败");
                return;
            }
            File[] children = source.listFiles();
            if (children == null || children.length == 0) {
                result.skipped.add(relativePath);
                return;
            }
            for (File child : children) {
                copyLegacyMirrorEntry(child, new File(target, child.getName()), relativePath + "/" + child.getName(), result);
            }
            return;
        }
        File parent = target.getParentFile();
        if (parent != null && !parent.exists() && !parent.mkdirs()) {
            result.failed.add(relativePath + ": 创建父目录失败");
            return;
        }
        try {
            copyLegacyFile(source, target);
            result.copied.add(relativePath);
        } catch (Exception error) {
            result.failed.add(relativePath + ": " + error.getMessage());
        }
    }

    private static void copyLegacyFile(File source, File target) throws Exception {
        try (InputStream input = new FileInputStream(source); OutputStream output = new FileOutputStream(target)) {
            byte[] buffer = new byte[8192];
            int read;
            while ((read = input.read(buffer)) >= 0) {
                if (read > 0) {
                    output.write(buffer, 0, read);
                }
            }
        }
    }

    private static String joinMessages(List<String> values) {
        StringBuilder builder = new StringBuilder();
        for (String value : values) {
            String normalized = value == null ? "" : value.trim();
            if (normalized.isEmpty()) {
                continue;
            }
            if (builder.length() > 0) {
                builder.append('；');
            }
            builder.append(normalized);
        }
        return builder.toString();
    }

    private static boolean sameCanonicalFile(File left, File right) {
        try {
            return left.getCanonicalFile().equals(right.getCanonicalFile());
        } catch (Exception ignored) {
            return left.getAbsoluteFile().equals(right.getAbsoluteFile());
        }
    }

    private boolean hasPersistedUriPermission(Uri uri) {
        if (uri == null) {
            return false;
        }
        List<UriPermission> permissions = getContext().getContentResolver().getPersistedUriPermissions();
        for (UriPermission permission : permissions) {
            if (uri.equals(permission.getUri()) && permission.isReadPermission() && permission.isWritePermission()) {
                return true;
            }
        }
        return false;
    }

    static void requireExportTreeUriPermission(boolean hasPermission) {
        if (!hasPermission) {
            throw new IllegalStateException(EXPORT_DIRECTORY_PERMISSION_LOST_MESSAGE);
        }
    }

    private void ensureDirectory(File dir) {
        if (!dir.exists() && !dir.mkdirs()) {
            throw new IllegalStateException("创建目录失败：" + dir.getAbsolutePath());
        }
    }

    private String mimeTypeForName(String name) {
        String lower = name == null ? "" : name.toLowerCase(Locale.ROOT);
        if (lower.endsWith(".csv")) {
            return "text/csv";
        }
        if (lower.endsWith(".json")) {
            return "application/json";
        }
        if (lower.endsWith(".txt")) {
            return "text/plain";
        }
        if (lower.endsWith(".zip")) {
            return "application/zip";
        }
        return "application/octet-stream";
    }

    private void openTargetPath(String targetPath) throws Exception {
        String normalized = targetPath == null ? "" : targetPath.trim();
        if (normalized.isEmpty()) {
            throw new IllegalArgumentException("缺少可打开的目标路径。");
        }
        Uri uri = Uri.parse(normalized);
        String scheme = uri.getScheme() == null ? "" : uri.getScheme().trim().toLowerCase(Locale.ROOT);
        if ("content".equals(scheme)) {
            if (DocumentsContract.isTreeUri(uri)) {
                openTreeUri(uri);
                return;
            }
            openContentUri(uri);
            return;
        }
        if ("http".equals(scheme) || "https".equals(scheme)) {
            Intent intent = new Intent(Intent.ACTION_VIEW, uri);
            intent.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK);
            startExternalIntent(intent, "没有可用的应用可以打开该链接。");
            return;
        }
        throw new IllegalStateException("Android 端暂不直接打开应用私有目录，请先导出文件，或打开已授权的导出目录/导出文件。");
    }

    private void openContentUri(Uri uri) throws Exception {
        String mimeType = mimeTypeForUri(uri);
        Intent viewIntent = new Intent(Intent.ACTION_VIEW);
        viewIntent.setDataAndType(uri, mimeType);
        viewIntent.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK | Intent.FLAG_GRANT_READ_URI_PERMISSION);
        try {
            startExternalIntent(viewIntent, "没有可用的应用可以打开该文件。");
            return;
        } catch (IllegalStateException error) {
            Intent shareIntent = new Intent(Intent.ACTION_SEND);
            shareIntent.setType(mimeType);
            shareIntent.putExtra(Intent.EXTRA_STREAM, uri);
            shareIntent.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK | Intent.FLAG_GRANT_READ_URI_PERMISSION);
            Intent chooser = Intent.createChooser(shareIntent, "打开文件");
            chooser.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK | Intent.FLAG_GRANT_READ_URI_PERMISSION);
            startExternalIntent(chooser, "没有可用的应用可以查看或分享该文件。");
        }
    }

    private void openTreeUri(Uri treeUri) throws Exception {
        requireExportTreeUriPermission(hasPersistedUriPermission(treeUri));
        for (Intent intent : AndroidDirectoryOpenIntents.openDirectoryIntents(treeUri)) {
            if (tryStartExternalIntent(intent)) {
                return;
            }
        }
        throw new IllegalStateException(EXPORT_DIRECTORY_OPEN_ERROR_MESSAGE);
    }

    private String mimeTypeForUri(Uri uri) {
        String mimeType = getContext().getContentResolver().getType(uri);
        if (mimeType != null && !mimeType.trim().isEmpty()) {
            return mimeType;
        }
        return mimeTypeForName(queryDisplayName(uri));
    }

    private void startExternalIntent(Intent intent, String errorMessage) {
        if (!tryStartExternalIntent(intent)) {
            throw new IllegalStateException(errorMessage);
        }
    }

    private boolean tryStartExternalIntent(Intent intent) {
        if (intent.resolveActivity(getContext().getPackageManager()) == null) {
            return false;
        }
        try {
            getContext().startActivity(intent);
            return true;
        } catch (ActivityNotFoundException | SecurityException error) {
            logPluginError("Failed to start external Android intent.", error);
            return false;
        }
    }

    static String nowRFC3339UTC() {
        SimpleDateFormat format = new SimpleDateFormat("yyyy-MM-dd'T'HH:mm:ss'Z'", Locale.ROOT);
        format.setTimeZone(TimeZone.getTimeZone("UTC"));
        return format.format(new Date());
    }

    private String nowRFC3339() {
        return nowRFC3339UTC();
    }

    private void runAsync(PluginCall call, Callable<String> action) {
        runAsync(call, action, false);
    }

    private void runAsync(PluginCall call, Callable<String> action, boolean syncAfterWrite) {
        executor.execute(() -> {
            try {
                call.resolve(new JSObject(finalizeServiceResponse(action.call(), syncAfterWrite)));
            } catch (Exception error) {
                rejectWithLog(call, "runAsync", error);
            }
        });
    }

    private String invokeServiceString(String methodName, String payload) throws Exception {
        Object result = service.getClass().getMethod(methodName, String.class).invoke(service, payload);
        return result == null ? "" : result.toString();
    }

    private void rejectWithLog(PluginCall call, String action, Exception error) {
        logPluginError("Plugin action failed: " + action, error);
        call.reject(error.getMessage(), error);
    }

    private void logPluginError(String message, Throwable error) {
        Log.e(TAG, message, error);
    }

    private JSObject command(String code, JSObject data, String message, boolean ok) {
        JSObject result = new JSObject();
        result.put("code", code);
        result.put("data", data);
        result.put("message", message);
        result.put("ok", ok);
        result.put("schema_version", "cfst-gui-mobile-v1");
        result.put("task_id", JSONObject.NULL);
        result.put("warnings", new JSONArray());
        return result;
    }

    private String commandJSON(String code, JSObject data, String message, boolean ok) {
        return command(code, data, message, ok).toString();
    }

    private String appVersion() {
        try {
            PackageInfo packageInfo = getContext().getPackageManager().getPackageInfo(getContext().getPackageName(), 0);
            String versionName = packageInfo.versionName;
            if (versionName != null && !versionName.trim().isEmpty()) {
                return versionName.trim();
            }
        } catch (Exception ignored) {
        }
        return "1.0";
    }

    private JSObject checkForUpdatesPayload() throws Exception {
        String response = readURL(LATEST_RELEASE_API);
        JSONObject release = new JSONObject(response);
        String latestVersion = normalizeVersion(release.optString("tag_name", ""));
        String currentVersion = appVersion();
        JSObject data = new JSObject();
        data.put("current_version", currentVersion);
        data.put("install_mode", "android_apk");
        data.put("platform", "android");
        data.put("latest_version", latestVersion);
        data.put("release_name", release.optString("name", ""));
        data.put("release_url", release.optString("html_url", RELEASE_PAGE_URL));
        boolean available = compareVersions(latestVersion, currentVersion) > 0;
        data.put("update_available", available);
        if (!available) {
            data.put("asset_name", "");
            data.put("download_url", "");
            data.put("sha256", "");
            return data;
        }
        applyAndroidAsset(release, data);
        if (data.getString("download_url", "").trim().isEmpty()) {
            throw new IllegalStateException("Android 更新资产缺少下载地址。");
        }
        return data;
    }

    private void applyAndroidAsset(JSONObject release, JSObject data) throws Exception {
        JSONArray assets = release.optJSONArray("assets");
        if (assets == null) {
            throw new IllegalStateException("GitHub Release 缺少 assets。");
        }
        JSONObject manifestAsset = findAsset(assets, "cfst-gui-update-manifest.json");
        if (manifestAsset != null) {
            JSONObject manifest = new JSONObject(readURL(manifestAsset.optString("browser_download_url", "")));
            JSONArray manifestAssets = manifest.optJSONArray("assets");
            if (manifestAssets != null) {
                JSONObject selected = selectAndroidManifestAsset(manifestAssets, assets);
                if (selected != null) {
                    String name = selected.optString("name", "cfst-gui-android-release.apk");
                    data.put("asset_name", name);
                    data.put("download_url", firstNonEmpty(selected.optString("download_url", ""), assetDownloadURL(assets, name)));
                    data.put("sha256", selected.optString("sha256", ""));
                    return;
                }
            }
        }
        JSONObject apk = findAsset(assets, "cfst-gui-android-release.apk");
        if (apk == null) {
            throw new IllegalStateException("GitHub Release 缺少 Android APK 资产。");
        }
        data.put("asset_name", apk.optString("name", "cfst-gui-android-release.apk"));
        data.put("download_url", apk.optString("browser_download_url", ""));
        data.put("sha256", "");
        if (data.getString("download_url", "").trim().isEmpty()) {
            throw new IllegalStateException("GitHub Release 的 Android APK 下载地址为空。");
        }
    }

    private JSONObject selectAndroidManifestAsset(JSONArray manifestAssets, JSONArray releaseAssets) {
        JSONObject universal = null;
        String[] supportedABIs = Build.VERSION.SDK_INT >= Build.VERSION_CODES.LOLLIPOP ? Build.SUPPORTED_ABIS : new String[] { Build.CPU_ABI, Build.CPU_ABI2 };
        for (String abi : supportedABIs) {
            JSONObject matched = findAndroidManifestAssetByABI(manifestAssets, releaseAssets, abi);
            if (matched != null) {
                return matched;
            }
        }
        for (int i = 0; i < manifestAssets.length(); i++) {
            JSONObject asset = manifestAssets.optJSONObject(i);
            if (asset == null || !isAndroidManifestAsset(asset)) {
                continue;
            }
            String abi = asset.optString("abi", "").trim();
            if (abi.isEmpty() || "universal".equalsIgnoreCase(abi)) {
                universal = asset;
                break;
            }
        }
        if (universal != null) {
            return universal;
        }
        for (int i = 0; i < manifestAssets.length(); i++) {
            JSONObject asset = manifestAssets.optJSONObject(i);
            if (asset != null && isAndroidManifestAsset(asset)) {
                return asset;
            }
        }
        return null;
    }

    private JSONObject findAndroidManifestAssetByABI(JSONArray manifestAssets, JSONArray releaseAssets, String deviceABI) {
        String normalizedABI = normalizeAndroidABI(deviceABI);
        if (normalizedABI.isEmpty()) {
            return null;
        }
        for (int i = 0; i < manifestAssets.length(); i++) {
            JSONObject asset = manifestAssets.optJSONObject(i);
            if (asset == null || !isAndroidManifestAsset(asset)) {
                continue;
            }
            String manifestABI = normalizeAndroidABI(asset.optString("abi", ""));
            if (!normalizedABI.equals(manifestABI)) {
                continue;
            }
            String name = asset.optString("name", "");
            if ((!name.isEmpty() && !assetDownloadURL(releaseAssets, name).trim().isEmpty()) || !asset.optString("download_url", "").trim().isEmpty()) {
                return asset;
            }
        }
        return null;
    }

    private boolean isAndroidManifestAsset(JSONObject asset) {
        String platform = asset.optString("platform", "");
        String goos = asset.optString("goos", "");
        return "android".equalsIgnoreCase(platform) || "android".equalsIgnoreCase(goos);
    }

    private String normalizeAndroidABI(String value) {
        String normalized = value == null ? "" : value.trim().toLowerCase(Locale.ROOT);
        switch (normalized) {
            case "arm64":
            case "arm64-v8a":
            case "aarch64":
                return "arm64-v8a";
            case "arm":
            case "armeabi":
            case "armeabi-v7a":
            case "armv7":
            case "armv7a":
                return "armeabi-v7a";
            case "universal":
                return "universal";
            default:
                return normalized;
        }
    }

    private JSONObject findAsset(JSONArray assets, String name) {
        for (int i = 0; i < assets.length(); i++) {
            JSONObject asset = assets.optJSONObject(i);
            if (asset != null && name.equals(asset.optString("name", ""))) {
                return asset;
            }
        }
        return null;
    }

    private String assetDownloadURL(JSONArray assets, String name) {
        JSONObject asset = findAsset(assets, name);
        return asset == null ? "" : asset.optString("browser_download_url", "");
    }

    private List<String> githubDownloadCandidates(String rawURL) {
        String value = rawURL == null ? "" : rawURL.trim();
        if (value.isEmpty()) {
            return new ArrayList<>();
        }
        try {
            java.net.URI uri = new java.net.URI(value);
            String host = uri.getHost();
            if (host == null) {
                ArrayList<String> single = new ArrayList<>();
                single.add(value);
                return single;
            }
            if (KKGITHUB_HOST.equalsIgnoreCase(host) || value.startsWith(GHPROXY_GITHUB_PREFIX)) {
                ArrayList<String> single = new ArrayList<>();
                single.add(value);
                return single;
            }
            if (!"github.com".equalsIgnoreCase(host)) {
                ArrayList<String> single = new ArrayList<>();
                single.add(value);
                return single;
            }
            java.net.URI kkUri = new java.net.URI(
                "https",
                KKGITHUB_HOST,
                uri.getPath(),
                uri.getQuery(),
                uri.getFragment()
            );
            ArrayList<String> candidates = new ArrayList<>();
            candidates.add(GHPROXY_GITHUB_PREFIX + value);
            candidates.add(kkUri.toString());
            candidates.add(value);
            return uniqueURLs(candidates);
        } catch (Exception error) {
            ArrayList<String> single = new ArrayList<>();
            single.add(value);
            return single;
        }
    }

    private String readURL(String rawURL) throws Exception {
        List<String> candidates = githubDownloadCandidates(rawURL);
        if (candidates.isEmpty()) {
            throw new IllegalStateException("缺少有效读取地址。");
        }
        Exception lastError = null;
        for (String candidate : candidates) {
            java.net.HttpURLConnection connection = null;
            try {
                connection = (java.net.HttpURLConnection) new java.net.URL(candidate).openConnection();
                connection.setConnectTimeout(30000);
                connection.setReadTimeout(30000);
                connection.setRequestProperty("Accept", "application/vnd.github+json");
                connection.setRequestProperty("User-Agent", "CFST-GUI/" + appVersion());
                int status = connection.getResponseCode();
                InputStream input = status >= 200 && status < 300 ? connection.getInputStream() : connection.getErrorStream();
                try (InputStream body = input; ByteArrayOutputStream output = new ByteArrayOutputStream()) {
                    if (body != null) {
                        copy(body, output);
                    }
                    if (status < 200 || status >= 300) {
                        throw new IllegalStateException("HTTP " + status + "：" + output.toString(StandardCharsets.UTF_8.name()) + " (" + candidate + ")");
                    }
                    return output.toString(StandardCharsets.UTF_8.name());
                }
            } catch (Exception error) {
                lastError = error;
            } finally {
                if (connection != null) {
                    connection.disconnect();
                }
            }
        }
        throw lastError == null ? new IllegalStateException("读取远程内容失败。") : lastError;
    }

    private void downloadURLToFile(String rawURL, File target) throws Exception {
        List<String> candidates = githubDownloadCandidates(rawURL);
        if (candidates.isEmpty()) {
            throw new IllegalStateException("下载 APK 缺少有效地址。");
        }
        Exception lastError = null;
        for (String candidate : candidates) {
            java.net.HttpURLConnection connection = null;
            try {
                connection = (java.net.HttpURLConnection) new java.net.URL(candidate).openConnection();
                connection.setConnectTimeout(30000);
                connection.setReadTimeout(30000);
                connection.setRequestProperty("User-Agent", "CFST-GUI/" + appVersion());
                int status = connection.getResponseCode();
                if (status < 200 || status >= 300) {
                    throw new IllegalStateException("下载 APK 返回 HTTP " + status + " (" + candidate + ")");
                }
                try (InputStream input = connection.getInputStream(); OutputStream output = new FileOutputStream(target)) {
                    copy(input, output);
                }
                return;
            } catch (Exception error) {
                lastError = error;
            } finally {
                if (connection != null) {
                    connection.disconnect();
                }
            }
        }
        throw lastError == null ? new IllegalStateException("下载 APK 失败。") : lastError;
    }

    private List<String> uniqueURLs(List<String> values) {
        ArrayList<String> result = new ArrayList<>();
        for (String value : values) {
            String normalized = value == null ? "" : value.trim();
            if (normalized.isEmpty() || result.contains(normalized)) {
                continue;
            }
            result.add(normalized);
        }
        return result;
    }

    private void verifySHA256(File file, String expected) throws Exception {
        MessageDigest digest = MessageDigest.getInstance("SHA-256");
        try (InputStream input = new FileInputStream(file)) {
            byte[] buffer = new byte[8192];
            int read;
            while ((read = input.read(buffer)) >= 0) {
                digest.update(buffer, 0, read);
            }
        }
        String actual = bytesToHex(digest.digest());
        if (!actual.equalsIgnoreCase(expected.trim())) {
            throw new IllegalStateException("SHA256 校验失败：期望 " + expected + "，实际 " + actual);
        }
    }

    private void triggerAPKInstall(File apk) {
        Uri uri = FileProvider.getUriForFile(getContext(), getContext().getPackageName() + ".fileprovider", apk);
        Intent intent = new Intent(Intent.ACTION_VIEW);
        intent.setDataAndType(uri, "application/vnd.android.package-archive");
        intent.addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION | Intent.FLAG_ACTIVITY_NEW_TASK);
        getContext().startActivity(intent);
    }

    private String normalizeVersion(String value) {
        String normalized = value == null ? "" : value.trim();
        if (normalized.startsWith("v") || normalized.startsWith("V")) {
            normalized = normalized.substring(1);
        }
        return normalized;
    }

    private int compareVersions(String left, String right) {
        int[] leftParts = versionParts(left);
        int[] rightParts = versionParts(right);
        int count = Math.max(leftParts.length, rightParts.length);
        for (int i = 0; i < count; i++) {
            int l = i < leftParts.length ? leftParts[i] : 0;
            int r = i < rightParts.length ? rightParts[i] : 0;
            if (l > r) {
                return 1;
            }
            if (l < r) {
                return -1;
            }
        }
        return 0;
    }

    private int[] versionParts(String value) {
        String normalized = normalizeVersion(value).split("[-+]")[0];
        String[] parts = normalized.split("\\.");
        int[] result = new int[parts.length];
        for (int i = 0; i < parts.length; i++) {
            String digits = parts[i].replaceAll("[^0-9].*$", "");
            result[i] = digits.isEmpty() ? 0 : Integer.parseInt(digits);
        }
        return result;
    }

    private String bytesToHex(byte[] bytes) {
        StringBuilder builder = new StringBuilder(bytes.length * 2);
        for (byte b : bytes) {
            builder.append(String.format(Locale.ROOT, "%02x", b));
        }
        return builder.toString();
    }

    private String firstNonEmpty(String... values) {
        if (values == null) {
            return "";
        }
        for (String value : values) {
            if (value != null && !value.trim().isEmpty()) {
                return value;
            }
        }
        return "";
    }

    private String normalizePathSelectionMode(String mode) {
        if (mode == null || mode.trim().isEmpty()) {
            return "source_file";
        }
        return mode.trim().toLowerCase().replace('-', '_');
    }

    private boolean isExportDirectoryMode(String mode) {
        return "export_target".equals(mode) || "export_dir".equals(mode) || "export_directory".equals(mode);
    }

    private boolean isExportFileMode(String mode) {
        return "export_file".equals(mode) || "save_file".equals(mode);
    }

    private boolean isConfigExportMode(String mode) {
        return "config_export".equals(mode);
    }

    private boolean isConfigArchiveExportMode(String mode) {
        return "config_archive_export".equals(mode);
    }

    private boolean isConfigImportMode(String mode) {
        return "config_import".equals(mode) || "import_config".equals(mode);
    }

    private boolean isConfigArchiveImportMode(String mode) {
        return "config_archive_import".equals(mode);
    }

    private boolean isStorageDirMode(String mode) {
        return "storage_dir".equals(mode);
    }

    private void putInitialURIExtra(Intent intent, PluginCall call) {
        String currentPath = firstNonEmpty(call.getString("current_path", ""), call.getString("currentPath", ""));
        if (currentPath == null || !currentPath.trim().startsWith("content://")) {
            return;
        }
        try {
            intent.putExtra(DocumentsContract.EXTRA_INITIAL_URI, Uri.parse(currentPath.trim()));
        } catch (Exception ignored) {
            // Initial URI is only a picker hint; ignore malformed saved values.
        }
    }

    private void persistUriPermission(Intent data, Uri uri) {
        int flags = data.getFlags() & (Intent.FLAG_GRANT_READ_URI_PERMISSION | Intent.FLAG_GRANT_WRITE_URI_PERMISSION);
        if (flags == 0) {
            return;
        }
        try {
            getContext().getContentResolver().takePersistableUriPermission(uri, flags);
        } catch (SecurityException ignored) {
            // Some providers grant one-shot access only; copied imports do not need persisted access.
        }
    }

    private String queryDisplayName(Uri uri) {
        try (Cursor cursor = getContext().getContentResolver().query(uri, null, null, null, null)) {
            if (cursor != null && cursor.moveToFirst()) {
                int index = cursor.getColumnIndex(OpenableColumns.DISPLAY_NAME);
                if (index >= 0) {
                    String name = cursor.getString(index);
                    return name == null ? "" : name;
                }
            }
        } catch (Exception ignored) {
            // Fall back to URI path below.
        }
        String path = uri.getLastPathSegment();
        return path == null ? "" : path;
    }

    private File copyURIToPrivateFile(Uri uri, String displayName) throws Exception {
        File dir = new File(getContext().getFilesDir(), "imports");
        if (!dir.exists() && !dir.mkdirs()) {
            throw new IllegalStateException("创建导入目录失败：" + dir.getAbsolutePath());
        }
        String name = sanitizeFileName(displayName);
        if (name.isEmpty()) {
            name = "source.txt";
        }
        File target = new File(dir, System.currentTimeMillis() + "-" + name);
        try (InputStream input = getContext().getContentResolver().openInputStream(uri);
             OutputStream output = new FileOutputStream(target)) {
            if (input == null) {
                throw new IllegalStateException("无法读取选择的文件。");
            }
            copy(input, output);
        }
        return target;
    }

    private File copyResultURIToPrivateFile(Uri uri, String displayName) throws Exception {
        File dir = new File(getContext().getFilesDir(), "result-files");
        if (!dir.exists() && !dir.mkdirs()) {
            throw new IllegalStateException("创建结果缓存目录失败：" + dir.getAbsolutePath());
        }
        String name = sanitizeFileName(displayName);
        if (name.isEmpty()) {
            name = "result.csv";
        }
        File target = new File(dir, System.currentTimeMillis() + "-" + name);
        try (InputStream input = getContext().getContentResolver().openInputStream(uri);
             OutputStream output = new FileOutputStream(target)) {
            if (input == null) {
                throw new IllegalStateException("无法读取选择的结果文件。");
            }
            copy(input, output);
        }
        return target;
    }

    private String readURIText(Uri uri) throws Exception {
        try (InputStream input = getContext().getContentResolver().openInputStream(uri);
             ByteArrayOutputStream output = new ByteArrayOutputStream()) {
            if (input == null) {
                throw new IllegalStateException("无法读取选择的配置文件。");
            }
            copy(input, output);
            return output.toString(StandardCharsets.UTF_8.name());
        }
    }

    private String readURIBase64(Uri uri) throws Exception {
        try (InputStream input = getContext().getContentResolver().openInputStream(uri);
             ByteArrayOutputStream output = new ByteArrayOutputStream()) {
            if (input == null) {
                throw new IllegalStateException("无法读取选择的配置文件。");
            }
            copy(input, output);
            return Base64.encodeToString(output.toByteArray(), Base64.NO_WRAP);
        }
    }

    private void copy(InputStream input, OutputStream output) throws Exception {
        byte[] buffer = new byte[8192];
        int read;
        while ((read = input.read(buffer)) >= 0) {
            if (read > 0) {
                output.write(buffer, 0, read);
            }
        }
    }

    private String sanitizeFileName(String value) {
        if (value == null) {
            return "";
        }
        return value.replaceAll("[\\\\/:*?\"<>|]", "_").trim();
    }

    private String extractExportTargetURI(String payloadJSON) {
        try {
            JSONObject payload = new JSONObject(payloadJSON);
            JSONObject config = payload.optJSONObject("config");
            if (config == null) {
                return "";
            }
            JSONObject exportConfig = config.optJSONObject("export");
            if (exportConfig == null) {
                return "";
            }
            String value = exportConfig.optString("target_uri", "");
            if (value.isEmpty()) {
                value = exportConfig.optString("targetUri", "");
            }
            return value == null ? "" : value.trim();
        } catch (Exception ignored) {
            return "";
        }
    }

    private String extractTargetURI(String payloadJSON) {
        try {
            JSONObject payload = new JSONObject(payloadJSON);
            String value = payload.optString("target_uri", "");
            if (value.isEmpty()) {
                value = payload.optString("targetUri", "");
            }
            if (value.isEmpty()) {
                JSONObject config = payload.optJSONObject("config");
                if (config == null) {
                    config = payload.optJSONObject("config_snapshot");
                }
                if (config == null) {
                    config = payload.optJSONObject("configSnapshot");
                }
                JSONObject exportConfig = config == null ? null : config.optJSONObject("export");
                if (exportConfig != null) {
                    value = exportConfig.optString("target_uri", "");
                    if (value.isEmpty()) {
                        value = exportConfig.optString("targetUri", "");
                    }
                }
            }
            return value == null ? "" : value.trim();
        } catch (Exception ignored) {
            return "";
        }
    }

    private String withPrivateResultFilePath(String payloadJSON) throws Exception {
        JSONObject payload = new JSONObject(payloadJSON);
        String resultURI = extractResultFileURI(payload);
        if (resultURI.isEmpty()) {
            return payloadJSON;
        }
        Uri uri = Uri.parse(resultURI);
        File copied = copyResultURIToPrivateFile(uri, queryDisplayName(uri));
        payload.put("path", copied.getAbsolutePath());
        payload.put("source_uri", resultURI);
        payload.put("sourceUri", resultURI);
        payload.remove("export_path");
        payload.remove("exportPath");
        payload.remove("source_path");
        payload.remove("sourcePath");
        return payload.toString();
    }

    private String extractResultFileURI(JSONObject payload) {
        String[] keys = new String[] { "path", "source_path", "sourcePath", "export_path", "exportPath" };
        for (String key : keys) {
            String value = payload.optString(key, "");
            if (value != null && value.trim().startsWith("content://") && !AndroidStorageBridge.isTreeURIString(value)) {
                return value.trim();
            }
        }
        JSONObject config = payload.optJSONObject("config");
        if (config == null) {
            config = payload.optJSONObject("config_snapshot");
        }
        if (config == null) {
            config = payload.optJSONObject("configSnapshot");
        }
        if (config != null) {
            JSONObject exportConfig = config.optJSONObject("export");
            if (exportConfig != null) {
                String value = exportConfig.optString("target_uri", "");
                if (value.isEmpty()) {
                    value = exportConfig.optString("targetUri", "");
                }
                if (value != null && value.trim().startsWith("content://") && !AndroidStorageBridge.isTreeURIString(value)) {
                    return value.trim();
                }
            }
        }
        return "";
    }

    private String withAndroidExportURI(String payloadJSON, String exportURI) {
        if (exportURI == null || exportURI.trim().isEmpty()) {
            return payloadJSON;
        }
        try {
            JSONObject payload = new JSONObject(payloadJSON);
            payload.put("android_export_uri", exportURI.trim());
            return payload.toString();
        } catch (Exception ignored) {
            return payloadJSON;
        }
    }

    private String copyProbeExportToURI(String responseJSON, String exportURI) {
        return AndroidStorageBridge.copyProbeExportToURI(getContext(), responseJSON, exportURI);
    }

    static String copyProbeExportToURIStatic(Context context, String responseJSON, String exportURI) {
        return AndroidStorageBridge.copyProbeExportToURI(context, responseJSON, exportURI);
    }

    private String writeConfigExportToURI(String responseJSON, String targetURI) {
        try {
            JSONObject command = new JSONObject(responseJSON);
            JSONObject data = command.optJSONObject("data");
            if (data == null) {
                return responseJSON;
            }
            String content = data.optString("content", "");
            if (content.isEmpty()) {
                appendWarning(command, data, "配置导出内容为空，未写入系统选择的目标。");
                return command.toString();
            }
            String writtenURI = AndroidStorageBridge.writeBytesToSafTarget(
                getContext(),
                targetURI,
                data.optString("file_name", "cfst-gui-config.json"),
                content.getBytes(StandardCharsets.UTF_8),
                true
            );
            data.put("target_uri", writtenURI);
            data.put("path", writtenURI);
            data.remove("content");
            return command.toString();
        } catch (Exception error) {
            try {
                JSONObject command = new JSONObject(responseJSON);
                JSONObject data = command.optJSONObject("data");
                appendWarning(command, data, "Android 配置导出到系统文件失败：" + error.getMessage());
                return command.toString();
            } catch (Exception ignored) {
                return responseJSON;
            }
        }
    }

    private String writeConfigArchiveToURI(String responseJSON, String targetURI) {
        try {
            JSONObject command = new JSONObject(responseJSON);
            JSONObject data = command.optJSONObject("data");
            if (data == null) {
                return responseJSON;
            }
            String contentBase64 = data.optString("content_base64", "");
            if (contentBase64.isEmpty()) {
                appendWarning(command, data, "配置压缩包内容为空，未写入系统选择的目标。");
                return command.toString();
            }
            byte[] archive = Base64.decode(contentBase64, Base64.DEFAULT);
            String writtenURI = AndroidStorageBridge.writeBytesToSafTarget(
                getContext(),
                targetURI,
                data.optString("file_name", "cfst-gui-config.zip"),
                archive,
                true
            );
            data.put("target_uri", writtenURI);
            data.put("path", writtenURI);
            data.remove("content_base64");
            return command.toString();
        } catch (Exception error) {
            try {
                JSONObject command = new JSONObject(responseJSON);
                JSONObject data = command.optJSONObject("data");
                appendWarning(command, data, "Android 配置压缩包导出到系统文件失败：" + error.getMessage());
                return command.toString();
            } catch (Exception ignored) {
                return responseJSON;
            }
        }
    }

    private String writeCSVExportToURI(String responseJSON, String targetURI) {
        try {
            JSONObject command = new JSONObject(responseJSON);
            JSONObject data = command.optJSONObject("data");
            if (data == null) {
                return responseJSON;
            }
            String contentBase64 = data.optString("content_base64", "");
            if (contentBase64.isEmpty()) {
                return csvExportWriteFailed(command, data, "CSV 导出内容为空，未写入系统选择的目标。");
            }
            byte[] csv = Base64.decode(contentBase64, Base64.DEFAULT);
            String writtenURI = AndroidStorageBridge.writeBytesToSafTarget(
                getContext(),
                targetURI,
                data.optString("file_name", "result.csv"),
                csv,
                true
            );
            data.put("target_uri", writtenURI);
            data.put("path", writtenURI);
            data.remove("content_base64");
            return command.toString();
        } catch (Exception error) {
            try {
                JSONObject command = new JSONObject(responseJSON);
                JSONObject data = command.optJSONObject("data");
                return csvExportWriteFailed(command, data, "Android CSV 导出到系统文件失败：" + error.getMessage());
            } catch (Exception ignored) {
                return responseJSON;
            }
        }
    }

    private String csvExportWriteFailed(JSONObject command, JSONObject data, String message) throws JSONException {
        appendWarning(command, data, message);
        command.put("code", "RESULTS_CSV_EXPORT_WRITE_FAILED");
        command.put("message", message);
        command.put("ok", false);
        return command.toString();
    }

    private String writeDebugLogExportToURI(String responseJSON, String targetURI) {
        try {
            JSONObject command = new JSONObject(responseJSON);
            JSONObject data = command.optJSONObject("data");
            if (data == null) {
                return responseJSON;
            }
            String contentBase64 = data.optString("content_base64", "");
            if (contentBase64.isEmpty()) {
                return debugLogExportWriteFailed(command, data, "调试日志内容为空，未写入系统选择的目标。");
            }
            byte[] logContent = Base64.decode(contentBase64, Base64.DEFAULT);
            String writtenURI = AndroidStorageBridge.writeBytesToSafTarget(
                getContext(),
                targetURI,
                data.optString("file_name", "cfip-log.txt"),
                logContent,
                true
            );
            data.put("target_uri", writtenURI);
            data.put("path", writtenURI);
            data.remove("content_base64");
            return command.toString();
        } catch (Exception error) {
            try {
                JSONObject command = new JSONObject(responseJSON);
                JSONObject data = command.optJSONObject("data");
                return debugLogExportWriteFailed(command, data, "Android 调试日志导出到系统文件失败：" + error.getMessage());
            } catch (Exception ignored) {
                return responseJSON;
            }
        }
    }

    private String debugLogExportWriteFailed(JSONObject command, JSONObject data, String message) throws JSONException {
        appendWarning(command, data, message);
        command.put("code", "DEBUG_LOG_EXPORT_WRITE_FAILED");
        command.put("message", message);
        command.put("ok", false);
        return command.toString();
    }

    private void appendWarning(JSONObject command, JSONObject data, String warning) throws JSONException {
        JSONArray topWarnings = command.optJSONArray("warnings");
        if (topWarnings == null) {
            topWarnings = new JSONArray();
            command.put("warnings", topWarnings);
        }
        topWarnings.put(warning);
        if (data != null) {
            JSONArray dataWarnings = data.optJSONArray("warnings");
            if (dataWarnings == null) {
                dataWarnings = new JSONArray();
                data.put("warnings", dataWarnings);
            }
            dataWarnings.put(warning);
        }
    }
}
