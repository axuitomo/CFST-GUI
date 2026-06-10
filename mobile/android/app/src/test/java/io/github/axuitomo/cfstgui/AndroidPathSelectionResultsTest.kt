package io.github.axuitomo.cfstgui

import android.app.Activity
import android.content.Intent
import android.net.Uri
import java.io.ByteArrayInputStream
import java.nio.charset.StandardCharsets
import java.nio.file.Files
import java.nio.file.Path
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
class AndroidPathSelectionResultsTest {
    @Test
    fun canceledSelectionReturnsCanceledCommand() {
        val command = AndroidPathSelectionResults.commandForResult(
            RuntimeEnvironment.getApplication(),
            "config_import",
            Activity.RESULT_CANCELED,
            null,
        )

        val payload = JSONObject(command.toString())
        assertEquals("PATH_SELECTION_CANCELED", payload.getString("code"))
        assertTrue(payload.getBoolean("ok"))
        assertEquals("已取消系统文件选择。", payload.getString("message"))
        assertTrue(payload.getJSONObject("data").getBoolean("canceled"))
        assertEquals("config_import", payload.getJSONObject("data").getString("mode"))
    }

    @Test
    fun exportFileSelectionReturnsTargetUri() {
        val uri = Uri.parse("content://example.test/export.csv")

        val payload = selectedPayload("export_file", uri)

        val data = payload.getJSONObject("data")
        assertEquals("PATH_SELECTED", payload.getString("code"))
        assertEquals("已选择导出文件。", payload.getString("message"))
        assertFalse(data.getBoolean("canceled"))
        assertEquals(uri.toString(), data.getString("target_uri"))
        assertEquals("export.csv", data.getString("path"))
    }

    @Test
    fun configImportReadsSelectedDocumentText() {
        val context = RuntimeEnvironment.getApplication()
        val uri = Uri.parse("content://example.test/config.json")
        Shadows.shadowOf(context.contentResolver).registerInputStream(
            uri,
            ByteArrayInputStream("{\"enabled\":true}".toByteArray(StandardCharsets.UTF_8)),
        )

        val command = AndroidPathSelectionResults.commandForResult(context, "config_import", Activity.RESULT_OK, resultIntent(uri))

        val payload = JSONObject(command.toString())
        val data = payload.getJSONObject("data")
        assertEquals("已读取配置文件。", payload.getString("message"))
        assertEquals("{\"enabled\":true}", data.getString("content"))
        assertEquals("config.json", data.getString("path"))
    }

    @Test
    fun sourceFileImportCopiesSelectedDocumentToPrivateImports() {
        val context = RuntimeEnvironment.getApplication()
        val uri = Uri.parse("content://example.test/source.csv")
        Shadows.shadowOf(context.contentResolver).registerInputStream(
            uri,
            ByteArrayInputStream("ip,ms".toByteArray(StandardCharsets.UTF_8)),
        )

        val command = AndroidPathSelectionResults.commandForResult(context, "source_file", Activity.RESULT_OK, resultIntent(uri))

        val payload = JSONObject(command.toString())
        val data = payload.getJSONObject("data")
        assertEquals("已选择输入源文件。", payload.getString("message"))
        assertTrue(data.getString("path").contains("/imports/"))
        assertEquals("ip,ms", String(Files.readAllBytes(Path.of(data.getString("path"))), StandardCharsets.UTF_8))
    }

    private fun selectedPayload(mode: String, uri: Uri): JSONObject {
        val command = AndroidPathSelectionResults.commandForResult(
            RuntimeEnvironment.getApplication(),
            mode,
            Activity.RESULT_OK,
            resultIntent(uri),
        )
        return JSONObject(command.toString())
    }

    private fun resultIntent(uri: Uri): Intent {
        return Intent().setData(uri)
    }
}
