package io.github.axuitomo.cfstgui

import android.Manifest
import java.io.File
import java.nio.file.Files
import java.time.Instant
import org.json.JSONObject
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RuntimeEnvironment
import org.robolectric.RobolectricTestRunner
import org.robolectric.Shadows
import org.robolectric.annotation.Config

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [35])
class LogMonitorForegroundServiceTest {
    @Test
    fun manifestDeclaresIndependentLogMonitorProcess() {
        val text = manifestFile().readText()

        assertTrue(text.contains("""android:name=".LogMonitorForegroundService""""))
        assertTrue(text.contains("""android:exported="false""""))
        assertTrue(text.contains("""android:process=":logmonitor""""))
    }

    @Test
    fun monitorDefaultsToEnabledAndHonorsConfigSwitch() {
        val runtimeDir = tempRuntimeDir()

        assertTrue(LogMonitorForegroundService.monitorEnabled(runtimeDir))

        File(runtimeDir, "mobile-config.json").writeText(
            JSONObject()
                .put(
                    "config_snapshot",
                    JSONObject().put("logging", JSONObject().put("monitor_enabled", false)),
                )
                .toString(),
        )

        assertFalse(LogMonitorForegroundService.monitorEnabled(runtimeDir))
    }

    @Test
    fun startIfConfiguredStartsForegroundServiceWhenAllowed() {
        val context = RuntimeEnvironment.getApplication()
        val runtimeDir = tempRuntimeDir()
        Shadows.shadowOf(context).grantPermissions(Manifest.permission.POST_NOTIFICATIONS)

        val started = LogMonitorForegroundService.startIfConfigured(context, runtimeDir.absolutePath)
        val intent = Shadows.shadowOf(context).nextStartedService

        assertTrue(started)
        assertEquals(LogMonitorForegroundService::class.java.name, intent.component?.className)
    }

    @Test
    fun staleHeartbeatWritesMonitorLog() {
        val runtimeDir = tempRuntimeDir()
        val logDir = File(runtimeDir, "logs")
        logDir.mkdirs()
        File(logDir, "main-heartbeat.json").writeText(
            JSONObject()
                .put("pid", 0)
                .put("started_at", "2026-06-14T12:00:00Z")
                .put("last_seen_at", "2026-06-14T12:00:00Z")
                .put("state", "running")
                .put("log_dir", logDir.absolutePath)
                .toString(),
        )

        val stopped = LogMonitorForegroundService.checkHeartbeatForTest(runtimeDir, Instant.parse("2026-06-14T12:00:11Z"))

        assertFalse(stopped)
        val monitorFiles = logDir.listFiles { _, name -> name.startsWith("monitor-") && name.endsWith(".jsonl") }.orEmpty()
        assertEquals(1, monitorFiles.size)
        val text = monitorFiles[0].readText()
        val entry = JSONObject(text.trim())
        assertEquals("cfst-log-v1", entry.getString("schema_version"))
        assertEquals("monitor", entry.getString("channel"))
        assertEquals("info", entry.getString("level"))
        assertEquals("main.hung", entry.getString("event"))
        assertEquals("running", entry.getJSONObject("data").getString("state"))
    }

    private fun tempRuntimeDir(): File {
        return Files.createTempDirectory("cfst-log-monitor").toFile()
    }

    private fun manifestFile(): File {
        return listOf(
            File("src/main/AndroidManifest.xml"),
            File("app/src/main/AndroidManifest.xml"),
        ).first { it.exists() }
    }
}
