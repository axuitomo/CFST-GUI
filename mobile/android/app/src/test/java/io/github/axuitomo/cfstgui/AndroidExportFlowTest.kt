package io.github.axuitomo.cfstgui

import android.net.Uri
import java.io.File
import java.nio.charset.StandardCharsets
import java.nio.file.Files
import org.json.JSONObject
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RuntimeEnvironment
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [35])
class AndroidExportFlowTest {
    @Test
    fun configExportWithoutTargetFinalizesStorageState() {
        val action = RecordingExportAction(
            "{\"ok\":true,\"code\":\"CONFIG_EXPORT_READY\",\"message\":\"ok\",\"data\":{\"content\":\"{}\"},\"warnings\":[]}",
        )

        val command = JSONObject(AndroidExportFlow.exportConfig(RuntimeEnvironment.getApplication(), "{\"task_id\":\"t1\"}", action))

        assertEquals("{\"task_id\":\"t1\"}", action.payload)
        assertEquals("CONFIG_EXPORT_READY", command.getString("code"))
        assertEquals("private", command.getJSONObject("data").getJSONObject("storage").getString("backend"))
        assertTrue(command.getJSONObject("data").has("content"))
    }

    @Test
    fun configExportWithTargetWritesContentAndFinalizesStorageState() {
        val context = RuntimeEnvironment.getApplication()
        val target = File(context.cacheDir, "flow-config.json")
        val action = RecordingExportAction(
            "{\"ok\":true,\"code\":\"CONFIG_EXPORT_READY\",\"message\":\"ok\",\"data\":{\"content\":\"{\\\"enabled\\\":true}\",\"file_name\":\"config.json\"},\"warnings\":[]}",
        )

        val command = JSONObject(
            AndroidExportFlow.exportConfig(
                context,
                "{\"target_uri\":\"" + Uri.fromFile(target) + "\"}",
                action,
            ),
        )
        val data = command.getJSONObject("data")

        assertEquals(Uri.fromFile(target).toString(), data.getString("target_uri"))
        assertEquals(Uri.fromFile(target).toString(), data.getString("path"))
        assertFalse(data.has("content"))
        assertEquals("private", data.getJSONObject("storage").getString("backend"))
        assertEquals("{\"enabled\":true}", String(Files.readAllBytes(target.toPath()), StandardCharsets.UTF_8))
    }

    @Test
    fun csvExportTargetFailureKeepsFailureCommandAndFinalizesStorageState() {
        val action = RecordingExportAction(
            "{\"ok\":true,\"code\":\"RESULTS_CSV_EXPORT_OK\",\"message\":\"ok\",\"data\":{\"content_base64\":\"aXAsbXMA\",\"file_name\":\"result.csv\"},\"warnings\":[]}",
        )

        var command = JSONObject(
            AndroidExportFlow.exportResultsCSV(
                RuntimeEnvironment.getApplication(),
                "{\"target_uri\":\"\"}",
                action,
            ),
        )
        var data = command.getJSONObject("data")

        assertEquals("RESULTS_CSV_EXPORT_OK", command.getString("code"))
        assertTrue(command.getBoolean("ok"))
        assertTrue(data.getJSONObject("storage").getBoolean("permission_ok"))

        command = JSONObject(
            AndroidExportFlow.exportResultsCSV(
                RuntimeEnvironment.getApplication(),
                "{\"target_uri\":\"content://missing/tree\"}",
                action,
            ),
        )
        data = command.getJSONObject("data")

        assertEquals("RESULTS_CSV_EXPORT_WRITE_FAILED", command.getString("code"))
        assertFalse(command.getBoolean("ok"))
        assertTrue(command.getString("message").startsWith("Android CSV 导出到系统文件失败："))
        assertTrue(data.getJSONArray("warnings").length() > 0)
        assertEquals("private", data.getJSONObject("storage").getString("backend"))
    }

    @Test
    fun debugLogExportWithEmptyContentMarksFailureAndFinalizesStorageState() {
        val action = RecordingExportAction(
            "{\"ok\":true,\"code\":\"DEBUG_LOG_EXPORT_OK\",\"message\":\"ok\",\"data\":{\"content_base64\":\"\",\"file_name\":\"cfip-log.txt\"},\"warnings\":[]}",
        )

        val command = JSONObject(
            AndroidExportFlow.exportDebugLog(
                RuntimeEnvironment.getApplication(),
                "{\"target_uri\":\"file:///tmp/cfip-log.txt\"}",
                action,
            ),
        )

        assertEquals("DEBUG_LOG_EXPORT_WRITE_FAILED", command.getString("code"))
        assertFalse(command.getBoolean("ok"))
        assertEquals("调试日志内容为空，未写入系统选择的目标。", command.getString("message"))
        assertTrue(command.getJSONObject("data").has("storage"))
    }

    @Test
    fun diagnosticPackageExportWithTargetWritesContentAndFinalizesStorageState() {
        val context = RuntimeEnvironment.getApplication()
        val target = File(context.cacheDir, "flow-diagnostics.zip")
        val action = RecordingExportAction(
            "{\"ok\":true,\"code\":\"DIAGNOSTIC_PACKAGE_EXPORT_OK\",\"message\":\"ok\",\"data\":{\"content_base64\":\"emlw\",\"file_name\":\"diagnostics.zip\"},\"warnings\":[]}",
        )

        val command = JSONObject(
            AndroidExportFlow.exportDiagnosticPackage(
                context,
                "{\"target_uri\":\"" + Uri.fromFile(target) + "\"}",
                action,
            ),
        )
        val data = command.getJSONObject("data")

        assertEquals("DIAGNOSTIC_PACKAGE_EXPORT_OK", command.getString("code"))
        assertEquals(Uri.fromFile(target).toString(), data.getString("target_uri"))
        assertFalse(data.has("content_base64"))
        assertEquals("private", data.getJSONObject("storage").getString("backend"))
        assertEquals("zip", String(Files.readAllBytes(target.toPath()), StandardCharsets.UTF_8))
    }

    private class RecordingExportAction(private val response: String) : AndroidExportFlow.ExportAction {
        var payload: String? = null

        override fun export(payloadJSON: String): String {
            payload = payloadJSON
            return response
        }
    }
}
