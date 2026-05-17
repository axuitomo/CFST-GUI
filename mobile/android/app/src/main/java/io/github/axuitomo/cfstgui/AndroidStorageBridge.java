package io.github.axuitomo.cfstgui;

import android.content.Context;
import android.content.UriPermission;
import android.net.Uri;
import androidx.documentfile.provider.DocumentFile;
import java.io.File;
import java.io.FileInputStream;
import java.io.InputStream;
import java.io.OutputStream;
import java.nio.charset.StandardCharsets;
import java.text.SimpleDateFormat;
import java.util.Date;
import java.util.List;
import java.util.Locale;
import java.util.TimeZone;
import org.json.JSONArray;
import org.json.JSONObject;

final class AndroidStorageBridge {
    private static final String STORAGE_BACKEND_SAF_MIRROR = "saf_mirror";
    private static final String[] STORAGE_ROOT_DIRECTORIES = new String[] { "backups", "exports", "tasks" };
    private static final String[] STORAGE_ROOT_FILES = new String[] {
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

    private AndroidStorageBridge() {}

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
                appendWarning(command, data, "Android 导出文件不存在，无法写入系统选择的目标。");
                return command.toString();
            }
            try (InputStream input = new FileInputStream(source);
                 OutputStream output = context.getContentResolver().openOutputStream(Uri.parse(exportURI), "wt")) {
                if (output == null) {
                    appendWarning(command, data, "Android 系统导出目标无法写入。");
                    return command.toString();
                }
                copy(input, output);
            }
            data.put("outputFile", exportURI);
            data.put("androidExportUri", exportURI);
            data.put("export_path", exportURI);
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

    static void syncRuntimeToAuthority(Context context) throws Exception {
        JSONObject bootstrap = readStorageBootstrap(context);
        if (!usesAuthorityStorage(bootstrap)) {
            return;
        }
        Uri treeUri = Uri.parse(bootstrap.optString("storage_uri", ""));
        if (!hasPersistedUriPermission(context, treeUri)) {
            throw new IllegalStateException("Android 未持有所选目录的持久化权限。");
        }
        syncLocalRootToTree(context, storageMirrorDir(context), treeUri);
        bootstrap.put("last_sync_at", nowRFC3339());
        bootstrap.put("last_sync_error", "");
        bootstrap.put("permission_ok", true);
        writeStorageBootstrap(context, bootstrap);
    }

    private static JSONObject readStorageBootstrap(Context context) throws Exception {
        File file = storageBootstrapFile(context);
        if (!file.exists()) {
            return new JSONObject();
        }
        try (InputStream input = new FileInputStream(file);
             java.io.ByteArrayOutputStream output = new java.io.ByteArrayOutputStream()) {
            copy(input, output);
            return new JSONObject(output.toString(StandardCharsets.UTF_8.name()));
        }
    }

    private static void writeStorageBootstrap(Context context, JSONObject bootstrap) throws Exception {
        File target = storageBootstrapFile(context);
        File parent = target.getParentFile();
        if (parent != null && !parent.exists() && !parent.mkdirs()) {
            throw new IllegalStateException("创建储存引导目录失败：" + parent.getAbsolutePath());
        }
        try (OutputStream output = new java.io.FileOutputStream(target)) {
            output.write(bootstrap.toString(2).getBytes(StandardCharsets.UTF_8));
        }
    }

    private static File storageBootstrapFile(Context context) {
        return new File(context.getFilesDir(), "storage-bootstrap.json");
    }

    private static File storageMirrorDir(Context context) {
        return new File(context.getFilesDir(), "storage-mirror");
    }

    private static boolean usesAuthorityStorage(JSONObject bootstrap) {
        return bootstrap != null
            && STORAGE_BACKEND_SAF_MIRROR.equals(bootstrap.optString("backend", "").trim())
            && !bootstrap.optString("storage_uri", "").trim().isEmpty();
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

    private static void syncLocalRootToTree(Context context, File localRoot, Uri treeUri) throws Exception {
        DocumentFile tree = openStorageTree(context, treeUri);
        for (String name : STORAGE_ROOT_FILES) {
            syncLocalEntryToTree(context, new File(localRoot, name), tree, name);
        }
        for (String name : STORAGE_ROOT_DIRECTORIES) {
            syncLocalEntryToTree(context, new File(localRoot, name), tree, name);
        }
    }

    private static void syncLocalEntryToTree(Context context, File source, DocumentFile parent, String relativePath) throws Exception {
        if (!source.exists()) {
            return;
        }
        if (source.isDirectory()) {
            DocumentFile targetDir = ensureTreeDirectory(parent, source.getName());
            File[] children = source.listFiles();
            if (children == null) {
                return;
            }
            for (File child : children) {
                syncLocalEntryToTree(context, child, targetDir, relativePath + "/" + child.getName());
            }
            return;
        }
        DocumentFile target = ensureTreeFile(parent, source.getName());
        try (InputStream input = new FileInputStream(source);
             OutputStream output = context.getContentResolver().openOutputStream(target.getUri(), "wt")) {
            if (output == null) {
                throw new IllegalStateException("无法写入目标文档：" + relativePath);
            }
            copy(input, output);
        }
    }

    private static DocumentFile openStorageTree(Context context, Uri treeUri) {
        DocumentFile tree = DocumentFile.fromTreeUri(context, treeUri);
        if (tree == null || !tree.isDirectory()) {
            throw new IllegalStateException("无法访问选择的储存目录。");
        }
        return tree;
    }

    private static DocumentFile ensureTreeDirectory(DocumentFile parent, String name) {
        DocumentFile existing = parent.findFile(name);
        if (existing != null && existing.isDirectory()) {
            return existing;
        }
        if (existing != null && existing.delete()) {
            existing = null;
        }
        DocumentFile created = parent.createDirectory(name);
        if (created == null) {
            throw new IllegalStateException("无法创建目录：" + name);
        }
        return created;
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

    private static String nowRFC3339() {
        SimpleDateFormat format = new SimpleDateFormat("yyyy-MM-dd'T'HH:mm:ss'Z'", Locale.ROOT);
        format.setTimeZone(TimeZone.getTimeZone("UTC"));
        return format.format(new Date());
    }
}
