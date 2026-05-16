package io.github.axuitomo.cfstgui;

import static org.junit.Assert.assertEquals;
import static org.junit.Assert.assertFalse;
import static org.junit.Assert.assertTrue;

import java.io.File;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import org.junit.Test;

public class ConfigLoadResultRewriterTest {

    @Test
    public void keepsFirstRunDefaultConfigReadyWhenNoExternalStorage() throws Exception {
        File tempDir = Files.createTempDirectory("cfst-config-load").toFile();
        ConfigLoadResultRewriter.RewriteDecision decision = diagnose("CONFIG_READY", tempDir, "", true, "", false);

        assertFalse(decision.shouldRewrite);
    }

    @Test
    public void rewritesPermissionLossToExplicitFailure() throws Exception {
        File tempDir = Files.createTempDirectory("cfst-config-load").toFile();
        ConfigLoadResultRewriter.RewriteDecision decision = diagnose(
            "CONFIG_READY",
            tempDir,
            "content://tree/documents",
            false,
            "Android 未持有所选目录的持久化权限，请重新选择储存目录。",
            false
        );

        assertTrue(decision.shouldRewrite);
        assertEquals(ConfigLoadResultRewriter.CODE_PERMISSION_LOST, decision.code);
        assertEquals("Android 未持有所选目录的持久化权限，请重新选择储存目录。", decision.message);
    }

    @Test
    public void rewritesSyncFailureWhenConfigFallsBackToDefault() throws Exception {
        File tempDir = Files.createTempDirectory("cfst-config-load").toFile();
        ConfigLoadResultRewriter.RewriteDecision decision = diagnose(
            "CONFIG_READY",
            tempDir,
            "content://tree/documents",
            true,
            "无法读取储存目录中的文件：mobile-config.json",
            false
        );

        assertTrue(decision.shouldRewrite);
        assertEquals(ConfigLoadResultRewriter.CODE_SYNC_FAILED, decision.code);
        assertEquals("无法读取储存目录中的文件：mobile-config.json", decision.message);
    }

    @Test
    public void keepsReadOkWhenRuntimeConfigExists() throws Exception {
        File tempDir = Files.createTempDirectory("cfst-config-load").toFile();
        ConfigLoadResultRewriter.RewriteDecision decision = diagnose(
            "CONFIG_READ_OK",
            tempDir,
            "content://tree/documents",
            true,
            "",
            true
        );

        assertFalse(decision.shouldRewrite);
    }

    private ConfigLoadResultRewriter.RewriteDecision diagnose(
        String code,
        File runtimeDir,
        String storageUri,
        boolean permissionOk,
        String lastSyncError,
        boolean createConfigFile
    ) throws Exception {
        File configFile = new File(runtimeDir, "mobile-config.json");
        if (createConfigFile) {
            Files.write(configFile.toPath(), "{\"config_snapshot\":{}}".getBytes(StandardCharsets.UTF_8));
        }
        return ConfigLoadResultRewriter.diagnose(
            code,
            storageUri,
            permissionOk,
            lastSyncError,
            permissionOk ? "储存目录可用。" : lastSyncError,
            configFile.getAbsolutePath()
        );
    }
}
