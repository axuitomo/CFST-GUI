package io.github.axuitomo.cfstgui

import android.content.ContextWrapper
import java.io.File
import java.nio.file.Files
import org.json.JSONObject
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

class AndroidStorageDirectoryTest {
    @Test
    fun deprecatedStorageChangeRewritesPrivateBootstrapAndReturnsCompatibleCommand() {
        val root = Files.createTempDirectory("cfst-storage-deprecated").toFile()
        try {
            val context = FilesDirContext(root)
            val initializer = RecordingInitializer()

            val command = JSONObject(AndroidStorageDirectory.commandForDeprecatedChange(context, initializer).toString())
            val data = command.getJSONObject("data")
            val migration = data.getJSONObject("migration")
            val storage = data.getJSONObject("storage")
            val stored = AndroidStorageState.readStorageBootstrap(context)

            assertEquals("STORAGE_SET_DEPRECATED", command.getString("code"))
            assertTrue(command.getBoolean("ok"))
            assertEquals("当前版本不再支持自定义储存目录，Android 固定使用应用私有目录。", command.getString("message"))
            assertEquals(0, migration.getJSONArray("copied").length())
            assertEquals(0, migration.getJSONArray("failed").length())
            assertEquals(0, migration.getJSONArray("skipped").length())
            assertEquals("private", storage.getString("backend"))
            assertEquals(root.absolutePath, storage.getString("runtime_dir"))
            assertEquals(root.absolutePath, initializer.runtimeDir)
            assertEquals("private", stored.getString("backend"))
            assertEquals(root.absolutePath, stored.getString("storage_dir"))
            assertEquals("", stored.getString("storage_uri"))
            assertTrue(stored.getBoolean("setup_completed"))
        } finally {
            deleteRecursively(root)
        }
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

    private class RecordingInitializer : AndroidStorageDirectory.RuntimeInitializer {
        var runtimeDir: String? = null

        override fun init(runtimeDir: String): String {
            this.runtimeDir = runtimeDir
            return "{}"
        }
    }

    private class FilesDirContext(private val filesDirValue: File) : ContextWrapper(null) {
        override fun getFilesDir(): File {
            return filesDirValue
        }
    }
}
