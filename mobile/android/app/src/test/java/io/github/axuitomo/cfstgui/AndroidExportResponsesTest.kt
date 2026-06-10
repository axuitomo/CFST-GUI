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
class AndroidExportResponsesTest {
    @Test
    fun writesConfigContentToSelectedDocument() {
        val context = RuntimeEnvironment.getApplication()
        val target = File(context.cacheDir, "config-export.json")
        val response = "{\"ok\":true,\"data\":{\"content\":\"{\\\"enabled\\\":true}\",\"file_name\":\"config.json\"}}"

        val rewritten = AndroidExportResponses.writeConfigExportToURI(context, response, Uri.fromFile(target).toString())

        val command = JSONObject(rewritten)
        val data = command.getJSONObject("data")
        assertEquals(Uri.fromFile(target).toString(), data.getString("target_uri"))
        assertEquals(Uri.fromFile(target).toString(), data.getString("path"))
        assertFalse(data.has("content"))
        assertEquals("{\"enabled\":true}", String(Files.readAllBytes(target.toPath()), StandardCharsets.UTF_8))
    }

    @Test
    fun addsWarningWhenConfigArchiveContentIsEmpty() {
        val context = RuntimeEnvironment.getApplication()
        val response = "{\"ok\":true,\"data\":{\"content_base64\":\"\",\"file_name\":\"config.zip\"}}"

        val rewritten = AndroidExportResponses.writeConfigArchiveToURI(
            context,
            response,
            Uri.fromFile(File(context.cacheDir, "config.zip")).toString(),
        )

        val command = JSONObject(rewritten)
        val data = command.getJSONObject("data")
        assertEquals("配置压缩包内容为空，未写入系统选择的目标。", command.getJSONArray("warnings").getString(0))
        assertEquals("配置压缩包内容为空，未写入系统选择的目标。", data.getJSONArray("warnings").getString(0))
        assertTrue(command.getBoolean("ok"))
        assertTrue(data.has("content_base64"))
    }

    @Test
    fun marksCSVExportFailedWhenTargetCannotBeWritten() {
        val context = RuntimeEnvironment.getApplication()
        val response =
            "{\"ok\":true,\"code\":\"RESULTS_CSV_EXPORT_OK\",\"message\":\"ok\",\"data\":{\"content_base64\":\"aXAsbXMA\",\"file_name\":\"result.csv\"}}"

        val rewritten = AndroidExportResponses.writeCSVExportToURI(context, response, "")

        val command = JSONObject(rewritten)
        val data = command.getJSONObject("data")
        assertEquals("RESULTS_CSV_EXPORT_WRITE_FAILED", command.getString("code"))
        assertFalse(command.getBoolean("ok"))
        assertTrue(command.getString("message").startsWith("Android CSV 导出到系统文件失败："))
        assertEquals(command.getString("message"), command.getJSONArray("warnings").getString(0))
        assertEquals(command.getString("message"), data.getJSONArray("warnings").getString(0))
    }

    @Test
    fun marksDebugLogExportFailedWhenContentIsEmpty() {
        val context = RuntimeEnvironment.getApplication()
        val response = "{\"ok\":true,\"code\":\"DEBUG_LOG_EXPORT_OK\",\"message\":\"ok\",\"data\":{\"content_base64\":\"\",\"file_name\":\"cfip-log.txt\"}}"

        val rewritten = AndroidExportResponses.writeDebugLogExportToURI(
            context,
            response,
            Uri.fromFile(File(context.cacheDir, "cfip-log.txt")).toString(),
        )

        val command = JSONObject(rewritten)
        assertEquals("DEBUG_LOG_EXPORT_WRITE_FAILED", command.getString("code"))
        assertFalse(command.getBoolean("ok"))
        assertEquals("调试日志内容为空，未写入系统选择的目标。", command.getString("message"))
    }
}
