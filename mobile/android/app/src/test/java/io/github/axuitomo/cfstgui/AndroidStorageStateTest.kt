package io.github.axuitomo.cfstgui

import android.content.Context
import android.content.ContextWrapper
import java.io.File
import java.nio.charset.StandardCharsets
import java.nio.file.Files
import org.json.JSONObject
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class AndroidStorageStateTest {
    @Test
    fun readsMissingBootstrapAsPrivateStorageAndWritesFile() {
        val root = Files.createTempDirectory("cfst-storage-state").toFile()
        try {
            val context: Context = FilesDirContext(root)

            val bootstrap = AndroidStorageState.readStorageBootstrap(context)

            assertEquals("private", bootstrap.getString("backend"))
            assertEquals(root.absolutePath, bootstrap.getString("storage_dir"))
            assertEquals("", bootstrap.getString("storage_uri"))
            assertTrue(AndroidStorageState.storageBootstrapFile(context).exists())
        } finally {
            deleteRecursively(root)
        }
    }

    @Test
    fun normalizesCamelCaseBootstrapFields() {
        val root = Files.createTempDirectory("cfst-storage-normalize").toFile()
        try {
            val context: Context = FilesDirContext(root)
            val source = JSONObject()
                .put("displayName", "Legacy")
                .put("lastSyncAt", "2026-06-10T00:00:00Z")
                .put("lastSyncError", "old error")
                .put("permissionOk", false)
                .put("portableMode", true)
                .put("setupCompleted", false)
                .put("storageUri", "content://legacy/tree")

            val bootstrap = AndroidStorageState.normalizeStorageBootstrap(context, source)

            assertEquals("private", bootstrap.getString("backend"))
            assertEquals("Legacy", bootstrap.getString("display_name"))
            assertEquals("2026-06-10T00:00:00Z", bootstrap.getString("last_sync_at"))
            assertEquals("old error", bootstrap.getString("last_sync_error"))
            assertFalse(bootstrap.getBoolean("permission_ok"))
            assertTrue(bootstrap.getBoolean("portable_mode"))
            assertFalse(bootstrap.getBoolean("setup_completed"))
            assertEquals(root.absolutePath, bootstrap.getString("storage_dir"))
            assertEquals("", bootstrap.getString("storage_uri"))
        } finally {
            deleteRecursively(root)
        }
    }

    @Test
    fun exposesCurrentPrivateStorageStatus() {
        val root = Files.createTempDirectory("cfst-storage-status").toFile()
        try {
            val context: Context = FilesDirContext(root)
            AndroidStorageState.writeStorageBootstrap(context, AndroidStorageState.defaultStorageBootstrap(context))

            val status = AndroidStorageState.currentStorageStatus(context)

            assertEquals("private", status.getString("backend"))
            assertEquals(root.absolutePath, status.getString("runtime_dir"))
            assertEquals(root.absolutePath, status.getString("current_dir"))
            assertTrue(status.getBoolean("permission_ok") == true)
            assertFalse(status.getBoolean("setup_required") == true)
            assertEquals("", status.getString("storage_uri"))
            assertTrue(requireNotNull(status.getJSObject("health")).getBoolean("exists") == true)
        } finally {
            deleteRecursively(root)
        }
    }

    @Test
    fun resolvesRuntimeDirectoryAndMigratesLegacySafMirror() {
        val root = Files.createTempDirectory("cfst-storage-mirror").toFile()
        try {
            val context: Context = FilesDirContext(root)
            writeText(File(root, "storage-mirror/mobile-config.json"), "legacy-config")
            writeText(
                File(root, "storage-bootstrap.json"),
                JSONObject()
                    .put("backend", "saf_mirror")
                    .put("storage_uri", "content://legacy/tree")
                    .toString(),
            )

            val bootstrap = AndroidStorageState.readStorageBootstrap(context)
            val runtimeDir = AndroidStorageState.resolveRuntimeDirectory(context, bootstrap)
            val stored = AndroidStorageState.readStorageBootstrap(context)

            assertEquals(root.absolutePath, runtimeDir)
            assertEquals("legacy-config", readText(File(root, "mobile-config.json")))
            assertTrue(stored.getBoolean("legacy_storage_mirror_migration_attempted"))
            assertTrue(stored.getBoolean("legacy_storage_mirror_migration_completed"))
            assertEquals("", stored.optString("legacy_storage_mirror_migration_error", ""))
        } finally {
            deleteRecursively(root)
        }
    }

    private fun writeText(target: File, value: String) {
        val parent = target.parentFile
        if (parent != null && !parent.exists()) {
            assertTrue(parent.mkdirs())
        }
        Files.write(target.toPath(), value.toByteArray(StandardCharsets.UTF_8))
    }

    private fun readText(target: File): String {
        return String(Files.readAllBytes(target.toPath()), StandardCharsets.UTF_8)
    }

    private fun deleteRecursively(file: File?) {
        if (file == null || !file.exists()) {
            return
        }
        if (file.isDirectory) {
            file.listFiles()?.forEach { child -> deleteRecursively(child) }
        }
        Files.deleteIfExists(file.toPath())
    }

    private class FilesDirContext(private val filesDirValue: File) : ContextWrapper(null) {
        override fun getFilesDir(): File {
            return filesDirValue
        }
    }
}
