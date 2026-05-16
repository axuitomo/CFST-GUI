package io.github.axuitomo.cfstgui;

import java.io.File;
import org.json.JSONObject;

final class ConfigLoadResultRewriter {
    static final String CODE_PERMISSION_LOST = "CONFIG_STORAGE_PERMISSION_LOST";
    static final String CODE_SYNC_FAILED = "CONFIG_STORAGE_SYNC_FAILED";

    private ConfigLoadResultRewriter() {
    }

    static JSONObject rewrite(JSONObject command) throws Exception {
        if (command == null) {
            return null;
        }
        JSONObject data = command.optJSONObject("data");
        if (data == null) {
            return command;
        }
        JSONObject storage = data.optJSONObject("storage");
        if (storage == null) {
            return command;
        }

        RewriteDecision decision = diagnose(
            command.optString("code", ""),
            stringValue(storage, "storage_uri", "storageUri"),
            storage.optBoolean("permission_ok", storage.optBoolean("permissionOk", true)),
            stringValue(storage, "last_sync_error", "lastSyncError"),
            nestedStringValue(storage, "health", "message"),
            stringValue(data, "configPath", "config_path")
        );
        if (!decision.shouldRewrite) {
            return command;
        }
        return rewriteFailure(command, decision.code, decision.message);
    }

    static RewriteDecision diagnose(
        String currentCode,
        String storageURI,
        boolean permissionOk,
        String lastSyncError,
        String healthMessage,
        String configPath
    ) {
        if (trim(storageURI).isEmpty()) {
            return RewriteDecision.keep();
        }
        if (!permissionOk) {
            String message = trim(healthMessage);
            if (message.isEmpty()) {
                message = trim(lastSyncError);
            }
            if (message.isEmpty()) {
                message = "Android 未持有所选目录的持久化权限，请重新选择储存目录。";
            }
            return RewriteDecision.rewrite(CODE_PERMISSION_LOST, message);
        }

        String code = trim(currentCode);
        String syncError = trim(lastSyncError);
        if (!"CONFIG_READY".equals(code) || syncError.isEmpty()) {
            return RewriteDecision.keep();
        }
        String path = trim(configPath);
        if (path.isEmpty() || new File(path).isFile()) {
            return RewriteDecision.keep();
        }
        return RewriteDecision.rewrite(CODE_SYNC_FAILED, syncError);
    }

    static final class RewriteDecision {
        final String code;
        final String message;
        final boolean shouldRewrite;

        private RewriteDecision(boolean shouldRewrite, String code, String message) {
            this.shouldRewrite = shouldRewrite;
            this.code = code;
            this.message = message;
        }

        static RewriteDecision keep() {
            return new RewriteDecision(false, "", "");
        }

        static RewriteDecision rewrite(String code, String message) {
            return new RewriteDecision(true, trim(code), trim(message));
        }
    }

    private static JSONObject rewriteFailure(JSONObject command, String code, String message) throws Exception {
        command.put("code", code);
        command.put("ok", false);
        command.put("message", message);
        return command;
    }

    private static String stringValue(JSONObject object, String primaryKey, String aliasKey) {
        String value = trim(object.optString(primaryKey, ""));
        if (!value.isEmpty()) {
            return value;
        }
        return trim(object.optString(aliasKey, ""));
    }

    private static String nestedStringValue(JSONObject object, String parentKey, String childKey) {
        JSONObject nested = object.optJSONObject(parentKey);
        if (nested == null) {
            return "";
        }
        return trim(nested.optString(childKey, ""));
    }

    private static String trim(String value) {
        return value == null ? "" : value.trim();
    }
}
