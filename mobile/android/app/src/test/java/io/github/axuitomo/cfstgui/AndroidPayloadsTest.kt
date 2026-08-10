package io.github.axuitomo.cfstgui

import org.json.JSONObject
import org.junit.Assert.assertEquals
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
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

}
