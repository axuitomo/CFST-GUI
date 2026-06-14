package io.github.axuitomo.cfstgui

import java.io.File
import java.nio.charset.StandardCharsets
import java.nio.file.Files
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class CfstPluginStorageMigrationTest {
    @Test
    fun migratesLegacySafMirrorKnownDataOverStalePrivateFiles() {
        val root = Files.createTempDirectory("cfst-mirror-migration").toFile()
        try {
            val mirror = File(root, "storage-mirror")
            val target = File(root, "private")
            writeText(File(mirror, "mobile-config.json"), "legacy-config")
            writeText(File(mirror, "source-profiles.json"), "legacy-source-profile")
            writeText(File(mirror, "tasks/task.json"), "legacy-task")
            writeText(File(mirror, "unknown.txt"), "unknown")
            writeText(File(target, "source-profiles.json"), "current-source-profile")
            assertTrue(File(target, "tasks").mkdirs())

            val result = CfstPlugin.migrateLegacySafMirrorFiles(mirror, target)

            assertTrue(result.attempted)
            assertTrue(result.completed)
            assertTrue(result.copied.contains("mobile-config.json"))
            assertTrue(result.copied.contains("tasks/task.json"))
            assertTrue(result.copied.contains("source-profiles.json"))
            assertEquals("legacy-config", readText(File(target, "mobile-config.json")))
            assertEquals("legacy-task", readText(File(target, "tasks/task.json")))
            assertEquals("legacy-source-profile", readText(File(target, "source-profiles.json")))
            assertFalse(File(target, "unknown.txt").exists())
            assertTrue(File(mirror, "mobile-config.json").exists())
        } finally {
            deleteRecursively(root)
        }
    }

    @Test
    fun ignoresEmptyLegacySafMirror() {
        val root = Files.createTempDirectory("cfst-empty-mirror").toFile()
        try {
            val mirror = File(root, "storage-mirror")
            val target = File(root, "private")
            assertTrue(mirror.mkdirs())

            val result = CfstPlugin.migrateLegacySafMirrorFiles(mirror, target)

            assertFalse(result.attempted)
            assertFalse(File(target, "mobile-config.json").exists())
        } finally {
            deleteRecursively(root)
        }
    }

    @Test
    fun cleansOnlyDownloadedAndroidUpdatePackages() {
        val root = Files.createTempDirectory("cfst-android-updates").toFile()
        try {
            writeText(File(root, "cfst-gui-android-release.apk"), "apk")
            writeText(File(root, "cfst-gui-android-release.apk.0.part"), "part")
            writeText(File(root, "other.apk"), "keep")
            writeText(File(root, "notes.txt"), "keep")
            writeText(File(root, "archive.apk.backup"), "keep")

            assertEquals(2, CfstPlugin.cleanupAndroidUpdatePackages(root))

            assertFalse(File(root, "cfst-gui-android-release.apk").exists())
            assertFalse(File(root, "cfst-gui-android-release.apk.0.part").exists())
            assertTrue(File(root, "other.apk").exists())
            assertTrue(File(root, "notes.txt").exists())
            assertTrue(File(root, "archive.apk.backup").exists())
        } finally {
            deleteRecursively(root)
        }
    }

    @Test
    fun recognizesAndroidUpdatePackageNames() {
        assertTrue(CfstPlugin.isAndroidUpdatePackageFile("cfst-gui-android-release.apk"))
        assertTrue(CfstPlugin.isAndroidUpdatePackageFile("cfst-gui-android-release.apk.2.part"))
        assertFalse(CfstPlugin.isAndroidUpdatePackageFile("other.apk"))
        assertFalse(CfstPlugin.isAndroidUpdatePackageFile("cfst-gui-android-release.apk.backup"))
        assertFalse(CfstPlugin.isAndroidUpdatePackageFile("notes.txt"))
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
}
