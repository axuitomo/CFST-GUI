package io.github.axuitomo.cfstgui;

import android.content.Context;
import android.content.UriPermission;
import android.net.Uri;
import androidx.documentfile.provider.DocumentFile;
import java.io.File;
import java.io.FileInputStream;
import java.io.InputStream;
import java.io.OutputStream;
import java.text.SimpleDateFormat;
import java.util.Date;
import java.util.List;
import java.util.Locale;
import java.util.TimeZone;
import org.json.JSONArray;
import org.json.JSONObject;

final class AndroidStorageBridge {
    private AndroidStorageBridge() {}

    static void ensureWritablePersistentExportTarget(Context context, String exportURI) {
        String targetURI = exportURI == null ? "" : exportURI.trim();
        if (targetURI.isEmpty()) {
            throw new IllegalStateException(persistentExportTargetError(targetURI));
        }
        if (!isTreeURIString(targetURI)) {
            throw new IllegalStateException(persistentExportTargetError(targetURI));
        }
        Uri treeUri = Uri.parse(targetURI);
        if (!hasPersistedUriPermission(context, treeUri)) {
            throw new IllegalStateException(persistentExportTargetError(targetURI));
        }
        DocumentFile tree = DocumentFile.fromTreeUri(context, treeUri);
        if (tree == null || !tree.isDirectory() || !tree.canWrite()) {
            throw new IllegalStateException("Android SAF 导出目录不可写，请重新选择导出目录。");
        }
    }

    static String copyProbeExportToURI(Context context, String responseJSON, String exportURI) {
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
                markAndroidExportFailed(data, exportURI, outputFile, "Android 导出文件不存在，无法写入系统选择的目标。");
                appendWarning(command, data, data.optString("android_export_error", "Android 导出文件不存在，无法写入系统选择的目标。"));
                return command.toString();
            }
            String writtenURI = writeFileToSafTarget(context, exportURI, source, false);
            markAndroidExportWritten(data, writtenURI, outputFile);
            data.put("outputFile", writtenURI);
            data.put("androidExportUri", writtenURI);
            data.put("export_path", writtenURI);
            return command.toString();
        } catch (Exception error) {
            try {
                JSONObject command = new JSONObject(responseJSON);
                JSONObject data = command.optJSONObject("data");
                String sourcePath = data == null ? "" : data.optString("outputFile", "");
                String message = androidExportFailureMessage(error);
                markAndroidExportFailed(data, exportURI, sourcePath, message);
                appendWarning(command, data, message);
                return command.toString();
            } catch (Exception ignored) {
                return responseJSON;
            }
        }
    }

    static String writeBytesToSafTarget(Context context, String targetURI, String targetFileName, byte[] content, boolean allowOneShotDocumentURI) throws Exception {
        String normalizedTargetURI = targetURI == null ? "" : targetURI.trim();
        if (normalizedTargetURI.isEmpty()) {
            throw new IllegalArgumentException("缺少 Android SAF 导出目标。");
        }
        if (content == null) {
            throw new IllegalArgumentException("Android SAF 导出内容为空。");
        }
        if (isTreeURIString(normalizedTargetURI)) {
            Uri writtenURI = writeBytesToTree(context, Uri.parse(normalizedTargetURI), safTargetFileName(targetFileName, "result.csv"), content);
            return writtenURI.toString();
        }
        Uri documentUri = Uri.parse(normalizedTargetURI);
        if (!allowOneShotDocumentURI && !hasPersistedUriPermission(context, documentUri)) {
            throw new IllegalStateException(persistentExportTargetError(normalizedTargetURI));
        }
        try (OutputStream output = context.getContentResolver().openOutputStream(documentUri, "wt")) {
            if (output == null) {
                throw new IllegalStateException("Android SAF 导出目标无法写入。");
            }
            output.write(content);
        }
        return normalizedTargetURI;
    }

    static boolean isTreeURIString(String value) {
        String normalized = value == null ? "" : value.trim();
        return normalized.startsWith("content://") && normalized.contains("/tree/");
    }

    static String persistentExportTargetError(String targetURI) {
        if (targetURI == null || targetURI.trim().isEmpty()) {
            return "缺少 Android SAF 导出目录，请重新选择导出目录。";
        }
        if (isTreeURIString(targetURI)) {
            return "Android 未持有所选导出目录的持久化权限，请重新选择导出目录。";
        }
        return "Android 导出目标不是 SAF 目录，请重新选择导出目录。";
    }

    private static boolean hasPersistedUriPermission(Context context, Uri uri) {
        if (uri == null) {
            return false;
        }
        List<UriPermission> permissions = context.getContentResolver().getPersistedUriPermissions();
        for (UriPermission permission : permissions) {
            if (uri.equals(permission.getUri()) && permission.isReadPermission() && permission.isWritePermission()) {
                return true;
            }
        }
        return false;
    }

    private static String writeFileToSafTarget(Context context, String targetURI, File source, boolean allowOneShotDocumentURI) throws Exception {
        if (source == null || !source.exists()) {
            throw new IllegalStateException("Android 导出文件不存在，无法写入系统选择的目标。");
        }
        String normalizedTargetURI = targetURI == null ? "" : targetURI.trim();
        if (normalizedTargetURI.isEmpty()) {
            throw new IllegalArgumentException("缺少 Android SAF 导出目录，请重新选择导出目录。");
        }
        if (isTreeURIString(normalizedTargetURI)) {
            Uri writtenURI = writeFileToTree(context, Uri.parse(normalizedTargetURI), source);
            return writtenURI.toString();
        }
        Uri documentUri = Uri.parse(normalizedTargetURI);
        if (!allowOneShotDocumentURI && !hasPersistedUriPermission(context, documentUri)) {
            throw new IllegalStateException(persistentExportTargetError(normalizedTargetURI));
        }
        try (InputStream input = new FileInputStream(source);
             OutputStream output = context.getContentResolver().openOutputStream(documentUri, "wt")) {
            if (output == null) {
                throw new IllegalStateException("Android SAF 导出目标无法写入。");
            }
            copy(input, output);
        }
        return normalizedTargetURI;
    }

    private static Uri writeFileToTree(Context context, Uri treeUri, File source) throws Exception {
        DocumentFile target = ensureWritableTreeFile(context, treeUri, safTargetFileName(source.getName(), "result.csv"));
        try (InputStream input = new FileInputStream(source);
             OutputStream output = context.getContentResolver().openOutputStream(target.getUri(), "wt")) {
            if (output == null) {
                throw new IllegalStateException("Android SAF 导出目录中的目标文件无法写入。");
            }
            copy(input, output);
        }
        return target.getUri();
    }

    private static Uri writeBytesToTree(Context context, Uri treeUri, String targetFileName, byte[] content) throws Exception {
        DocumentFile target = ensureWritableTreeFile(context, treeUri, targetFileName);
        try (OutputStream output = context.getContentResolver().openOutputStream(target.getUri(), "wt")) {
            if (output == null) {
                throw new IllegalStateException("Android SAF 导出目录中的目标文件无法写入。");
            }
            output.write(content);
        }
        return target.getUri();
    }

    private static DocumentFile ensureWritableTreeFile(Context context, Uri treeUri, String fileName) {
        if (!hasPersistedUriPermission(context, treeUri)) {
            throw new IllegalStateException(persistentExportTargetError(treeUri == null ? "" : treeUri.toString()));
        }
        DocumentFile tree = openStorageTree(context, treeUri);
        if (!tree.canWrite()) {
            throw new IllegalStateException("Android SAF 导出目录不可写，请重新选择导出目录。");
        }
        return ensureTreeFile(tree, fileName);
    }

    private static DocumentFile openStorageTree(Context context, Uri treeUri) {
        DocumentFile tree = DocumentFile.fromTreeUri(context, treeUri);
        if (tree == null || !tree.isDirectory()) {
            throw new IllegalStateException("无法访问选择的导出目录。");
        }
        return tree;
    }

    private static DocumentFile ensureTreeFile(DocumentFile parent, String name) {
        DocumentFile existing = parent.findFile(name);
        if (existing != null && existing.isFile()) {
            return existing;
        }
        if (existing != null && existing.delete()) {
            existing = null;
        }
        DocumentFile created = parent.createFile(mimeTypeForName(name), name);
        if (created == null) {
            throw new IllegalStateException("无法创建文件：" + name);
        }
        return created;
    }

    private static String mimeTypeForName(String name) {
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

    static String safTargetFileName(String name, String fallback) {
        String value = name == null ? "" : name.trim();
        if (value.isEmpty()) {
            value = fallback == null ? "" : fallback.trim();
        }
        value = value.replace('\\', '/');
        int separator = value.lastIndexOf('/');
        if (separator >= 0) {
            value = value.substring(separator + 1);
        }
        value = value.replaceAll("[\\\\/:*?\"<>|]", "_").trim();
        if (value.equals(".") || value.equals("..")) {
            value = "";
        }
        if (value.isEmpty()) {
            return "result.csv";
        }
        return value;
    }

    private static String androidExportFailureMessage(Exception error) {
        String message = error == null ? "" : error.getMessage();
        if (message == null || message.trim().isEmpty()) {
            message = "未知错误";
        }
        if (message.contains("请重新选择导出目录")) {
            return message;
        }
        return "Android 导出到系统文件失败：" + message;
    }

    private static void copy(InputStream input, OutputStream output) throws Exception {
        byte[] buffer = new byte[8192];
        int read;
        while ((read = input.read(buffer)) >= 0) {
            if (read > 0) {
                output.write(buffer, 0, read);
            }
        }
    }

    private static void appendWarning(JSONObject command, JSONObject data, String warning) throws Exception {
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

    private static void markAndroidExportWritten(JSONObject data, String exportURI, String sourcePath) throws Exception {
        if (data == null) {
            return;
        }
        data.put("android_export_status", "written");
        data.put("androidExportStatus", "written");
        data.put("android_export_uri", exportURI == null ? "" : exportURI);
        data.put("androidExportUri", exportURI == null ? "" : exportURI);
        data.put("android_export_source_path", sourcePath == null ? "" : sourcePath);
        data.put("androidExportSourcePath", sourcePath == null ? "" : sourcePath);
        data.put("android_export_error", "");
        data.put("androidExportError", "");
    }

    private static void markAndroidExportFailed(JSONObject data, String exportURI, String sourcePath, String message) throws Exception {
        if (data == null) {
            return;
        }
        data.put("android_export_status", "failed");
        data.put("androidExportStatus", "failed");
        data.put("android_export_uri", exportURI == null ? "" : exportURI);
        data.put("androidExportUri", exportURI == null ? "" : exportURI);
        data.put("android_export_source_path", sourcePath == null ? "" : sourcePath);
        data.put("androidExportSourcePath", sourcePath == null ? "" : sourcePath);
        data.put("android_export_error", message == null ? "" : message);
        data.put("androidExportError", message == null ? "" : message);
    }

    private static String nowRFC3339() {
        SimpleDateFormat format = new SimpleDateFormat("yyyy-MM-dd'T'HH:mm:ss'Z'", Locale.ROOT);
        format.setTimeZone(TimeZone.getTimeZone("UTC"));
        return format.format(new Date());
    }
}
