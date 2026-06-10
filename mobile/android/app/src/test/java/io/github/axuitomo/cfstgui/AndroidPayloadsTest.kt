package io.github.axuitomo.cfstgui

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
class AndroidPayloadsTest {
    @Test
    fun extractsDirectAndNestedExportTargets() {
        assertEquals("content://target/doc", AndroidPayloads.extractTargetURI("{\"target_uri\":\" content://target/doc \"}"))
        assertEquals("content://target/camel", AndroidPayloads.extractTargetURI("{\"targetUri\":\"content://target/camel\"}"))
        assertEquals(
            "content://target/nested",
            AndroidPayloads.extractTargetURI("{\"config_snapshot\":{\"export\":{\"target_uri\":\"content://target/nested\"}}}"),
        )
        assertEquals(
            "content://target/probe",
            AndroidPayloads.extractExportTargetURI("{\"config\":{\"export\":{\"targetUri\":\"content://target/probe\"}}}"),
        )
        assertEquals("", AndroidPayloads.extractTargetURI("{not-json"))
    }

    @Test
    fun addsAndroidExportUriWhenPresent() {
        val payload = JSONObject(AndroidPayloads.withAndroidExportURI("{\"task_id\":\"t1\"}", " content://export/tree "))

        assertEquals("t1", payload.getString("task_id"))
        assertEquals("content://export/tree", payload.getString("android_export_uri"))
        assertEquals("{not-json", AndroidPayloads.withAndroidExportURI("{not-json", "content://export/tree"))
    }

    @Test
    fun extractsOnlyDocumentContentUriForResultFiles() {
        assertEquals(
            "content://example/result.csv",
            AndroidPayloads.extractResultFileURI(JSONObject("{\"export_path\":\" content://example/result.csv \"}")),
        )
        assertEquals(
            "",
            AndroidPayloads.extractResultFileURI(JSONObject("{\"export_path\":\"content://example/tree/primary%3ADownload\"}")),
        )
        assertEquals(
            "content://example/nested.csv",
            AndroidPayloads.extractResultFileURI(JSONObject("{\"config\":{\"export\":{\"targetUri\":\"content://example/nested.csv\"}}}")),
        )
    }

    @Test
    fun copiesContentResultUriToPrivatePath() {
        val context = RuntimeEnvironment.getApplication()
        val source = Uri.parse("content://example.test/result.csv")
        Shadows.shadowOf(context.contentResolver).registerInputStream(
            source,
            ByteArrayInputStream("ip,ms".toByteArray(StandardCharsets.UTF_8)),
        )

        val rewritten = AndroidPayloads.withPrivateResultFilePath(context, "{\"path\":\"content://example.test/result.csv\"}")

        val payload = JSONObject(rewritten)
        assertTrue(payload.getString("path").contains("/result-files/"))
        assertEquals("content://example.test/result.csv", payload.getString("source_uri"))
        assertEquals("content://example.test/result.csv", payload.getString("sourceUri"))
        assertFalse(payload.has("export_path"))
        assertEquals("ip,ms", String(Files.readAllBytes(Path.of(payload.getString("path"))), StandardCharsets.UTF_8))
    }
}
