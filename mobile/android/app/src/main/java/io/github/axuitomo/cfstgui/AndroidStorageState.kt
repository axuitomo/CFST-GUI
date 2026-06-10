package io.github.axuitomo.cfstgui

import android.content.Context
import com.getcapacitor.JSObject
import java.io.ByteArrayOutputStream
import java.io.File
import java.io.FileInputStream
import java.io.FileOutputStream
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale
import java.util.TimeZone
import org.json.JSONObject

object AndroidStorageState {
    private const val STORAGE_BACKEND_PRIVATE = "private"
    private const val STORAGE_BACKEND_SAF_MIRROR = "saf_mirror"
    private const val STORAGE_BOOTSTRAP_SCHEMA = "cfst-gui-storage-v2"
    private const val LEGACY_BACKEND_FIELD = "_legacy_backend"
    private const val LEGACY_STORAGE_URI_FIELD = "_legacy_storage_uri"
    private const val LEGACY_MIRROR_MIGRATION_ATTEMPTED = "legacy_storage_mirror_migration_attempted"
    private const val LEGACY_MIRROR_MIGRATION_COMPLETED = "legacy_storage_mirror_migration_completed"
    private const val LEGACY_MIRROR_MIGRATION_ERROR = "legacy_storage_mirror_migration_error"

    @JvmStatic
    fun nowRFC3339UTC(): String {
        val format = SimpleDateFormat("yyyy-MM-dd'T'HH:mm:ss'Z'", Locale.ROOT)
        format.timeZone = TimeZone.getTimeZone("UTC")
        return format.format(Date())
    }

    @JvmStatic
    fun defaultRuntimeDir(context: Context): File = context.filesDir

    @JvmStatic
    fun storageBootstrapFile(context: Context): File = File(context.filesDir, "storage-bootstrap.json")

    @JvmStatic
    fun defaultStorageBootstrap(context: Context): JSONObject {
        return JSONObject()
            .put("backend", STORAGE_BACKEND_PRIVATE)
            .put("display_name", "")
            .put("last_sync_at", "")
            .put("last_sync_error", "")
            .put("permission_ok", true)
            .put("portable_mode", false)
            .put("schema_version", STORAGE_BOOTSTRAP_SCHEMA)
            .put("setup_completed", true)
            .put("storage_dir", defaultRuntimeDir(context).absolutePath)
            .put("storage_uri", "")
            .put("updated_at", nowRFC3339UTC())
    }

    @JvmStatic
    fun readStorageBootstrap(context: Context): JSONObject {
        val file = storageBootstrapFile(context)
        if (!file.exists()) {
            val bootstrap = defaultStorageBootstrap(context)
            writeStorageBootstrap(context, bootstrap)
            return bootstrap
        }
        FileInputStream(file).use { input ->
            val output = ByteArrayOutputStream()
            input.copyTo(output)
            val source = JSONObject(output.toString(Charsets.UTF_8.name()))
            val normalized = normalizeStorageBootstrap(context, source)
            normalized.put(LEGACY_BACKEND_FIELD, source.optString("backend", source.optString("storage_backend", "")))
            normalized.put(LEGACY_STORAGE_URI_FIELD, source.optString("storage_uri", source.optString("storageUri", "")))
            return normalized
        }
    }

    @JvmStatic
    fun normalizeStorageBootstrap(context: Context, source: JSONObject): JSONObject {
        return defaultStorageBootstrap(context)
            .put("backend", STORAGE_BACKEND_PRIVATE)
            .put("display_name", source.optString("display_name", source.optString("displayName", "")))
            .put("last_sync_at", source.optString("last_sync_at", source.optString("lastSyncAt", "")))
            .put("last_sync_error", source.optString("last_sync_error", source.optString("lastSyncError", "")))
            .put("permission_ok", source.optBoolean("permission_ok", source.optBoolean("permissionOk", true)))
            .put("portable_mode", source.optBoolean("portable_mode", source.optBoolean("portableMode", false)))
            .put("schema_version", STORAGE_BOOTSTRAP_SCHEMA)
            .put("setup_completed", source.optBoolean("setup_completed", source.optBoolean("setupCompleted", true)))
            .put("storage_dir", defaultRuntimeDir(context).absolutePath)
            .put("storage_uri", "")
            .putLegacyMigration(source)
            .put("updated_at", source.optString("updated_at", source.optString("updatedAt", nowRFC3339UTC())))
    }

    @JvmStatic
    fun writeStorageBootstrap(context: Context, bootstrap: JSONObject) {
        val normalized = normalizeStorageBootstrap(context, bootstrap)
        normalized.put("updated_at", nowRFC3339UTC())
        val target = storageBootstrapFile(context)
        val parent = target.parentFile
        if (parent != null && !parent.exists() && !parent.mkdirs()) {
            throw IllegalStateException("创建储存引导目录失败：" + parent.absolutePath)
        }
        FileOutputStream(target).use { output ->
            output.write(normalized.toString(2).toByteArray(Charsets.UTF_8))
        }
    }

    @JvmStatic
    fun resolveRuntimeDirectory(context: Context, bootstrap: JSONObject): String {
        val defaultDir = defaultRuntimeDir(context)
        ensureDirectory(defaultDir)
        val migration = migrateLegacySafMirrorIfNeeded(context, bootstrap, storageMirrorDir(context), defaultDir)
        if (migration.attempted) {
            bootstrap.put(LEGACY_MIRROR_MIGRATION_ATTEMPTED, true)
            bootstrap.put(LEGACY_MIRROR_MIGRATION_COMPLETED, migration.completed)
            bootstrap.put(LEGACY_MIRROR_MIGRATION_ERROR, AndroidStorageMigration.joinMessages(migration.failed))
        }
        bootstrap.put("backend", STORAGE_BACKEND_PRIVATE)
        bootstrap.put("last_sync_at", "")
        bootstrap.put("last_sync_error", "")
        bootstrap.put("permission_ok", true)
        if (!bootstrap.has("setup_completed")) {
            bootstrap.put("setup_completed", true)
        }
        bootstrap.put("storage_dir", defaultDir.absolutePath)
        bootstrap.put("storage_uri", "")
        writeStorageBootstrap(context, bootstrap)
        return defaultDir.absolutePath
    }

    @JvmStatic
    fun currentStorageStatus(context: Context): JSObject {
        val bootstrap = readStorageBootstrap(context)
        val runtimeDir = defaultRuntimeDir(context)
        ensureDirectory(runtimeDir)

        val health = JSObject()
        health.put("checked_at", nowRFC3339UTC())
        health.put("exists", runtimeDir.exists())
        health.put("free_bytes", -1)
        health.put("is_dir", runtimeDir.isDirectory)
        health.put("message", healthMessage(runtimeDir))
        health.put("path", runtimeDir.absolutePath)
        health.put("portable_mode", false)
        health.put("writable", runtimeDir.canWrite())
        val migrationError = bootstrap.optString(LEGACY_MIRROR_MIGRATION_ERROR, "").trim()
        if (migrationError.isNotEmpty()) {
            health.put("message", "旧 Android SAF mirror 迁移失败：$migrationError")
        }

        val status = JSObject()
        status.put("backend", STORAGE_BACKEND_PRIVATE)
        status.put("bootstrap_path", storageBootstrapFile(context).absolutePath)
        status.put("current_dir", defaultRuntimeDir(context).absolutePath)
        status.put("default_dir", defaultRuntimeDir(context).absolutePath)
        status.put("display_name", "")
        status.put("health", health)
        status.put("last_sync_at", "")
        status.put("last_sync_error", "")
        status.put("legacy_storage_mirror_migration_attempted", bootstrap.optBoolean(LEGACY_MIRROR_MIGRATION_ATTEMPTED, false))
        status.put("legacy_storage_mirror_migration_completed", bootstrap.optBoolean(LEGACY_MIRROR_MIGRATION_COMPLETED, false))
        status.put("legacy_storage_mirror_migration_error", migrationError)
        status.put("log_uri", "")
        status.put("permission_ok", true)
        status.put("portable_mode", false)
        status.put("runtime_dir", runtimeDir.absolutePath)
        status.put("setup_completed", bootstrap.optBoolean("setup_completed", true))
        status.put("setup_required", false)
        status.put("storage_uri", "")
        status.put("writable", runtimeDir.canWrite())
        return status
    }

    @JvmStatic
    fun migrateLegacySafMirrorIfNeeded(
        context: Context,
        bootstrap: JSONObject,
        mirrorDir: File = storageMirrorDir(context),
        targetDir: File = defaultRuntimeDir(context),
    ): AndroidStorageMigration.LegacyMirrorMigrationResult {
        if (bootstrap.optBoolean(LEGACY_MIRROR_MIGRATION_COMPLETED, false)) {
            return AndroidStorageMigration.LegacyMirrorMigrationResult()
        }
        val legacyBackend = bootstrap.optString(LEGACY_BACKEND_FIELD, "").trim()
        val legacyStorageURI = bootstrap.optString(LEGACY_STORAGE_URI_FIELD, "").trim()
        if (legacyBackend != STORAGE_BACKEND_SAF_MIRROR &&
            legacyStorageURI.isEmpty() &&
            !AndroidStorageMigration.hasKnownData(mirrorDir)
        ) {
            return AndroidStorageMigration.LegacyMirrorMigrationResult()
        }
        return AndroidStorageMigration.migrateLegacySafMirrorFiles(mirrorDir, targetDir)
    }

    @JvmStatic
    fun ensureDirectory(dir: File) {
        if (!dir.exists() && !dir.mkdirs()) {
            throw IllegalStateException("创建目录失败：" + dir.absolutePath)
        }
    }

    private fun storageMirrorDir(context: Context): File = File(context.filesDir, "storage-mirror")

    private fun healthMessage(runtimeDir: File): String {
        if (!runtimeDir.exists()) {
            return "应用私有数据目录尚未创建。"
        }
        return "应用私有数据目录可用。"
    }

    private fun JSONObject.putLegacyMigration(source: JSONObject): JSONObject {
        if (source.optBoolean(LEGACY_MIRROR_MIGRATION_ATTEMPTED, false)) {
            put(LEGACY_MIRROR_MIGRATION_ATTEMPTED, true)
        }
        if (source.optBoolean(LEGACY_MIRROR_MIGRATION_COMPLETED, false)) {
            put(LEGACY_MIRROR_MIGRATION_COMPLETED, true)
        }
        val migrationError = source.optString(LEGACY_MIRROR_MIGRATION_ERROR, "").trim()
        if (migrationError.isNotEmpty()) {
            put(LEGACY_MIRROR_MIGRATION_ERROR, migrationError)
        }
        return this
    }
}
