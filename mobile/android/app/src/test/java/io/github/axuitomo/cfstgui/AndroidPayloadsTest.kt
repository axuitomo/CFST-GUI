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

    @Test
    fun formatsProbeSpeedContentUsingReadyFlags() {
        val warmup = JSONObject()
            .put("ip", "1.1.1.1")
            .put("current_speed_mb_s", 0.0)
            .put("average_speed_mb_s", 0.0)
            .put("current_ready", false)
            .put("average_ready", false)
        assertEquals("1.1.1.1 正在测速中。", AndroidPayloads.formatProbeSpeedContent(warmup))

        val ready = JSONObject()
            .put("ip", "1.1.1.1")
            .put("current_speed_mb_s", 12.345)
            .put("average_speed_mb_s", 8.9)
            .put("current_ready", true)
            .put("average_ready", true)
        assertEquals("1.1.1.1 当前 12.35 MB/s，均速 8.90 MB/s。", AndroidPayloads.formatProbeSpeedContent(ready))

        val currentOnly = JSONObject()
            .put("ip", "1.1.1.1")
            .put("current_speed_mb_s", 4.0)
            .put("average_speed_mb_s", 0.0)
            .put("current_ready", true)
            .put("average_ready", false)
        assertEquals("1.1.1.1 当前 4.00 MB/s，均速 -。", AndroidPayloads.formatProbeSpeedContent(currentOnly))
    }

}
