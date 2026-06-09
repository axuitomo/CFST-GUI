package io.github.axuitomo.cfstgui;

import static org.junit.Assert.assertEquals;
import static org.junit.Assert.assertFalse;
import static org.junit.Assert.assertTrue;

import java.io.File;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import org.junit.Test;

public class CfstPluginStorageMigrationTest {

    @Test
    public void migratesLegacySafMirrorKnownDataOverStalePrivateFiles() throws Exception {
        File root = Files.createTempDirectory("cfst-mirror-migration").toFile();
        try {
            File mirror = new File(root, "storage-mirror");
            File target = new File(root, "private");
            writeText(new File(mirror, "mobile-config.json"), "legacy-config");
            writeText(new File(mirror, "source-profiles.json"), "legacy-source-profile");
            writeText(new File(mirror, "tasks/task.json"), "legacy-task");
            writeText(new File(mirror, "unknown.txt"), "unknown");
            writeText(new File(target, "source-profiles.json"), "current-source-profile");
            assertTrue(new File(target, "tasks").mkdirs());

            CfstPlugin.LegacyMirrorMigrationResult result = CfstPlugin.migrateLegacySafMirrorFiles(mirror, target);

            assertTrue(result.attempted);
            assertTrue(result.completed);
            assertTrue(result.copied.contains("mobile-config.json"));
            assertTrue(result.copied.contains("tasks/task.json"));
            assertTrue(result.copied.contains("source-profiles.json"));
            assertEquals("legacy-config", readText(new File(target, "mobile-config.json")));
            assertEquals("legacy-task", readText(new File(target, "tasks/task.json")));
            assertEquals("legacy-source-profile", readText(new File(target, "source-profiles.json")));
            assertFalse(new File(target, "unknown.txt").exists());
            assertTrue(new File(mirror, "mobile-config.json").exists());
        } finally {
            deleteRecursively(root);
        }
    }

    @Test
    public void ignoresEmptyLegacySafMirror() throws Exception {
        File root = Files.createTempDirectory("cfst-empty-mirror").toFile();
        try {
            File mirror = new File(root, "storage-mirror");
            File target = new File(root, "private");
            assertTrue(mirror.mkdirs());

            CfstPlugin.LegacyMirrorMigrationResult result = CfstPlugin.migrateLegacySafMirrorFiles(mirror, target);

            assertFalse(result.attempted);
            assertFalse(new File(target, "mobile-config.json").exists());
        } finally {
            deleteRecursively(root);
        }
    }

    @Test
    public void cleansOnlyDownloadedAndroidUpdatePackages() throws Exception {
        File root = Files.createTempDirectory("cfst-android-updates").toFile();
        try {
            writeText(new File(root, "cfst-gui-android-release.apk"), "apk");
            writeText(new File(root, "cfst-gui-android-release.apk.0.part"), "part");
            writeText(new File(root, "notes.txt"), "keep");
            writeText(new File(root, "archive.apk.backup"), "keep");

            assertEquals(2, CfstPlugin.cleanupAndroidUpdatePackages(root));

            assertFalse(new File(root, "cfst-gui-android-release.apk").exists());
            assertFalse(new File(root, "cfst-gui-android-release.apk.0.part").exists());
            assertTrue(new File(root, "notes.txt").exists());
            assertTrue(new File(root, "archive.apk.backup").exists());
        } finally {
            deleteRecursively(root);
        }
    }

    @Test
    public void recognizesAndroidUpdatePackageNames() {
        assertTrue(CfstPlugin.isAndroidUpdatePackageFile("cfst-gui-android-release.apk"));
        assertTrue(CfstPlugin.isAndroidUpdatePackageFile("cfst-gui-android-release.apk.2.part"));
        assertFalse(CfstPlugin.isAndroidUpdatePackageFile("cfst-gui-android-release.apk.backup"));
        assertFalse(CfstPlugin.isAndroidUpdatePackageFile("notes.txt"));
    }

    private static void writeText(File target, String value) throws Exception {
        File parent = target.getParentFile();
        if (parent != null && !parent.exists()) {
            assertTrue(parent.mkdirs());
        }
        Files.write(target.toPath(), value.getBytes(StandardCharsets.UTF_8));
    }

    private static String readText(File target) throws Exception {
        return new String(Files.readAllBytes(target.toPath()), StandardCharsets.UTF_8);
    }

    private static void deleteRecursively(File file) throws Exception {
        if (file == null || !file.exists()) {
            return;
        }
        if (file.isDirectory()) {
            File[] children = file.listFiles();
            if (children != null) {
                for (File child : children) {
                    deleteRecursively(child);
                }
            }
        }
        Files.deleteIfExists(file.toPath());
    }
}
