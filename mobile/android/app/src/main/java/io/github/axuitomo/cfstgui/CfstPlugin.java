package io.github.axuitomo.cfstgui;

import android.app.Activity;
import android.content.Intent;
import android.database.Cursor;
import android.net.Uri;
import android.provider.OpenableColumns;
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
import java.util.Locale;
import java.util.concurrent.Callable;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import mobileapi.EventSink;
import mobileapi.Mobileapi;
import mobileapi.Service;
import org.json.JSONArray;
import org.json.JSONException;
import org.json.JSONObject;

@CapacitorPlugin(name = "Cfst")
public class CfstPlugin extends Plugin {
    private static final String APP_VERSION = "1.1";
    private static final String LATEST_RELEASE_API = "https://api.github.com/repos/axuitomo/CFST-GUI/releases/latest";
    private static final String RELEASE_PAGE_URL = "https://github.com/axuitomo/CFST-GUI/releases/latest";
    private final ExecutorService executor = Executors.newSingleThreadExecutor();
    private Service service;

    @Override
    public void load() {
        service = Mobileapi.newService();
        service.setEventSink(new EventSink() {
            @Override
            public void onProbeEvent(String eventJSON) {
                try {
                    notifyListeners("desktop:probe", new JSObject(eventJSON));
                } catch (JSONException error) {
                    JSObject fallback = new JSObject();
                    fallback.put("event", "probe.failed");
                    fallback.put("schema_version", "cfst-gui-mobile-v1");
                    fallback.put("task_id", "");
                    fallback.put("seq", 0);
                    fallback.put("ts", "");
                    JSObject payload = new JSObject();
                    payload.put("message", error.getMessage());
                    fallback.put("payload", payload);
                    notifyListeners("desktop:probe", fallback);
                }
            }
        });
        service.init(getContext().getFilesDir().getAbsolutePath());
    }

    @PluginMethod
    public void Init(PluginCall call) {
        String baseDir = call.getString("baseDir", getContext().getFilesDir().getAbsolutePath());
        runAsync(call, () -> service.init(baseDir));
    }

    @PluginMethod
    public void LoadConfig(PluginCall call) {
        runAsync(call, () -> service.loadConfig());
    }

    @PluginMethod
    public void GetAppInfo(PluginCall call) {
        JSObject data = new JSObject();
        data.put("current_version", APP_VERSION);
        data.put("install_mode", "android_apk");
        data.put("platform", "android");
        data.put("release_url", RELEASE_PAGE_URL);
        call.resolve(command("APP_INFO_READY", data, "应用信息已读取。", true));
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
                String downloadURL = info.getString("download_url", "");
                String assetName = info.getString("asset_name", "cfst-gui-android-release.apk");
                File updateDir = new File(getContext().getFilesDir(), "updates");
                if (!updateDir.exists() && !updateDir.mkdirs()) {
                    throw new IllegalStateException("创建更新目录失败：" + updateDir.getAbsolutePath());
                }
                File apk = new File(updateDir, assetName);
                downloadURLToFile(downloadURL, apk);
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
        runAsync(call, () -> service.saveConfig(call.getData().toString()));
    }

    @PluginMethod
    public void SetStorageDirectory(PluginCall call) {
        runAsync(call, () -> service.setStorageDirectory(call.getData().toString()));
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
                call.resolve(new JSObject(response));
            } catch (Exception error) {
                call.reject(error.getMessage(), error);
            }
        });
    }

    @PluginMethod
    public void BackupCurrentConfig(PluginCall call) {
        runAsync(call, () -> service.backupCurrentConfig(call.getData().toString()));
    }

    @PluginMethod
    public void LoadProfiles(PluginCall call) {
        runAsync(call, () -> service.loadProfiles());
    }

    @PluginMethod
    public void SaveCurrentProfile(PluginCall call) {
        runAsync(call, () -> service.saveCurrentProfile(call.getData().toString()));
    }

    @PluginMethod
    public void SwitchProfile(PluginCall call) {
        runAsync(call, () -> service.switchProfile(call.getData().toString()));
    }

    @PluginMethod
    public void DeleteProfile(PluginCall call) {
        runAsync(call, () -> service.deleteProfile(call.getData().toString()));
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
        runAsync(call, () -> service.updateColoDictionary(call.getData().toString()));
    }

    @PluginMethod
    public void ProcessColoDictionary(PluginCall call) {
        runAsync(call, () -> service.processColoDictionary(call.getData().toString()));
    }

    @PluginMethod
    public void RunProbe(PluginCall call) {
        String payload = call.getData().toString();
        executor.execute(() -> {
            try {
                String exportURI = extractExportTargetURI(payload);
                String response = service.runProbe(withAndroidExportURI(payload, exportURI));
                if (!exportURI.isEmpty()) {
                    response = copyProbeExportToURI(response, exportURI);
                }
                call.resolve(new JSObject(response));
            } catch (Exception error) {
                call.reject(error.getMessage(), error);
            }
        });
    }

    @PluginMethod
    public void CancelProbe(PluginCall call) {
        try {
            call.resolve(new JSObject(service.cancelProbe(call.getData().toString())));
        } catch (Exception error) {
            call.reject(error.getMessage(), error);
        }
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
    public void OpenPath(PluginCall call) {
        runAsync(call, () -> service.openPath(call.getString("targetPath", "")));
    }

    @PluginMethod
    public void SelectPath(PluginCall call) {
        String mode = normalizePathSelectionMode(call.getString("mode", ""));
        Intent intent;
        if (isStorageDirMode(mode)) {
            intent = new Intent(Intent.ACTION_OPEN_DOCUMENT_TREE);
        } else if (isExportTargetMode(mode) || isConfigExportMode(mode)) {
            String defaultFileName = call.getString("defaultFileName", call.getString("default_file_name", "result.csv"));
            if (defaultFileName == null || defaultFileName.trim().isEmpty()) {
                defaultFileName = isConfigExportMode(mode) ? "cfst-gui-config.json" : "result.csv";
            }
            intent = new Intent(Intent.ACTION_CREATE_DOCUMENT);
            intent.addCategory(Intent.CATEGORY_OPENABLE);
            intent.setType(isConfigExportMode(mode) ? "application/json" : "text/csv");
            intent.putExtra(Intent.EXTRA_TITLE, defaultFileName);
        } else {
            intent = new Intent(Intent.ACTION_OPEN_DOCUMENT);
            intent.addCategory(Intent.CATEGORY_OPENABLE);
            intent.setType("*/*");
            if (isConfigImportMode(mode)) {
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
            persistUriPermission(resultData, uri);
            if (isStorageDirMode(mode)) {
                data.put("storage_uri", uri.toString());
                data.put("target_uri", uri.toString());
                data.put("path", displayName.isEmpty() ? uri.toString() : displayName);
                call.resolve(command("PATH_SELECTED", data, "已选择储存目录。", true));
                return;
            }

            if (isExportTargetMode(mode) || isConfigExportMode(mode)) {
                data.put("target_uri", uri.toString());
                data.put("path", displayName.isEmpty() ? uri.toString() : displayName);
                call.resolve(command("PATH_SELECTED", data, isConfigExportMode(mode) ? "已选择配置导出文件。" : "已选择导出文件。", true));
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

    private void runAsync(PluginCall call, Callable<String> action) {
        executor.execute(() -> {
            try {
                call.resolve(new JSObject(action.call()));
            } catch (Exception error) {
                call.reject(error.getMessage(), error);
            }
        });
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

    private JSObject checkForUpdatesPayload() throws Exception {
        String response = readURL(LATEST_RELEASE_API);
        JSONObject release = new JSONObject(response);
        String latestVersion = normalizeVersion(release.optString("tag_name", ""));
        JSObject data = new JSObject();
        data.put("current_version", APP_VERSION);
        data.put("install_mode", "android_apk");
        data.put("platform", "android");
        data.put("latest_version", latestVersion);
        data.put("release_name", release.optString("name", ""));
        data.put("release_url", release.optString("html_url", RELEASE_PAGE_URL));
        boolean available = compareVersions(latestVersion, APP_VERSION) > 0;
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
                for (int i = 0; i < manifestAssets.length(); i++) {
                    JSONObject asset = manifestAssets.optJSONObject(i);
                    if (asset == null) {
                        continue;
                    }
                    String platform = asset.optString("platform", "");
                    String goos = asset.optString("goos", "");
                    if ("android".equalsIgnoreCase(platform) || "android".equalsIgnoreCase(goos)) {
                        String name = asset.optString("name", "cfst-gui-android-release.apk");
                        data.put("asset_name", name);
                        data.put("download_url", firstNonEmpty(asset.optString("download_url", ""), assetDownloadURL(assets, name)));
                        data.put("sha256", asset.optString("sha256", ""));
                        return;
                    }
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

    private String readURL(String rawURL) throws Exception {
        java.net.HttpURLConnection connection = (java.net.HttpURLConnection) new java.net.URL(rawURL).openConnection();
        connection.setConnectTimeout(30000);
        connection.setReadTimeout(30000);
        connection.setRequestProperty("Accept", "application/vnd.github+json");
        connection.setRequestProperty("User-Agent", "CFST-GUI/" + APP_VERSION);
        int status = connection.getResponseCode();
        InputStream input = status >= 200 && status < 300 ? connection.getInputStream() : connection.getErrorStream();
        try (InputStream body = input; ByteArrayOutputStream output = new ByteArrayOutputStream()) {
            if (body != null) {
                copy(body, output);
            }
            if (status < 200 || status >= 300) {
                throw new IllegalStateException("HTTP " + status + "：" + output.toString(StandardCharsets.UTF_8.name()));
            }
            return output.toString(StandardCharsets.UTF_8.name());
        } finally {
            connection.disconnect();
        }
    }

    private void downloadURLToFile(String rawURL, File target) throws Exception {
        java.net.HttpURLConnection connection = (java.net.HttpURLConnection) new java.net.URL(rawURL).openConnection();
        connection.setConnectTimeout(30000);
        connection.setReadTimeout(30000);
        connection.setRequestProperty("User-Agent", "CFST-GUI/" + APP_VERSION);
        int status = connection.getResponseCode();
        if (status < 200 || status >= 300) {
            throw new IllegalStateException("下载 APK 返回 HTTP " + status);
        }
        try (InputStream input = connection.getInputStream(); OutputStream output = new FileOutputStream(target)) {
            copy(input, output);
        } finally {
            connection.disconnect();
        }
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

    private String firstNonEmpty(String first, String second) {
        return first == null || first.trim().isEmpty() ? second : first;
    }

    private String normalizePathSelectionMode(String mode) {
        if (mode == null || mode.trim().isEmpty()) {
            return "source_file";
        }
        return mode.trim().toLowerCase().replace('-', '_');
    }

    private boolean isExportTargetMode(String mode) {
        return "export_target".equals(mode) || "export_file".equals(mode) || "save_file".equals(mode);
    }

    private boolean isConfigExportMode(String mode) {
        return "config_export".equals(mode);
    }

    private boolean isConfigImportMode(String mode) {
        return "config_import".equals(mode) || "import_config".equals(mode);
    }

    private boolean isStorageDirMode(String mode) {
        return "storage_dir".equals(mode);
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
            return value == null ? "" : value.trim();
        } catch (Exception ignored) {
            return "";
        }
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
        try {
            JSONObject command = new JSONObject(responseJSON);
            JSONObject data = command.optJSONObject("data");
            if (data == null) {
                return responseJSON;
            }
            String outputFile = data.optString("outputFile", "");
            if (outputFile.isEmpty()) {
                return responseJSON;
            }
            File source = new File(outputFile);
            if (!source.exists()) {
                appendWarning(command, data, "Android 导出文件不存在，无法写入系统选择的目标。");
                return command.toString();
            }
            try (InputStream input = new FileInputStream(source);
                 OutputStream output = getContext().getContentResolver().openOutputStream(Uri.parse(exportURI), "wt")) {
                if (output == null) {
                    appendWarning(command, data, "Android 系统导出目标无法写入。");
                    return command.toString();
                }
                copy(input, output);
            }
            data.put("outputFile", exportURI);
            data.put("androidExportUri", exportURI);
            return command.toString();
        } catch (Exception error) {
            try {
                JSONObject command = new JSONObject(responseJSON);
                JSONObject data = command.optJSONObject("data");
                appendWarning(command, data, "Android 导出到系统文件失败：" + error.getMessage());
                return command.toString();
            } catch (Exception ignored) {
                return responseJSON;
            }
        }
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
            try (OutputStream output = getContext().getContentResolver().openOutputStream(Uri.parse(targetURI), "wt")) {
                if (output == null) {
                    appendWarning(command, data, "Android 系统配置导出目标无法写入。");
                    return command.toString();
                }
                output.write(content.getBytes(StandardCharsets.UTF_8));
            }
            data.put("target_uri", targetURI);
            data.put("path", targetURI);
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
