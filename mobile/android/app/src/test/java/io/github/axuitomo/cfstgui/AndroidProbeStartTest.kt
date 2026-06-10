package io.github.axuitomo.cfstgui

import android.content.Context
import android.content.Intent
import org.json.JSONObject
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertThrows
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RuntimeEnvironment
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [35])
class AndroidProbeStartTest {
    @Test
    fun alreadyRunningReturnsCompatibleCommandWithoutStartingService() {
        val starter = RecordingStarter(false)

        val command = JSONObject(
            AndroidProbeStart.startProbe(
                RuntimeEnvironment.getApplication(),
                "{\"task_id\":\"ignored\"}",
                "task-1",
                AndroidProbeStart.StartGate { false },
                starter,
            ).toString(),
        )

        assertEquals("PROBE_ALREADY_RUNNING", command.getString("code"))
        assertFalse(command.getBoolean("ok"))
        assertFalse(command.getJSONObject("data").getBoolean("accepted"))
        assertEquals("task-1", command.getJSONObject("data").getString("task_id"))
        assertTrue(starter.intents.isEmpty())
    }

    @Test
    fun acceptedStartInjectsAndroidExportUriAndStartsForegroundService() {
        val context = RuntimeEnvironment.getApplication()
        val starter = RecordingStarter(false)
        val targetUri = "content://com.android.externalstorage.documents/tree/primary%3ADownload%2Fcfst"
        val payload = "{\"config\":{\"export\":{\"targetUri\":\"$targetUri\"}}}"

        val command = JSONObject(
            AndroidProbeStart.startProbe(
                context,
                payload,
                "task-2",
                AndroidProbeStart.StartGate { true },
                AndroidProbeStart.ExportTargetValidator { _, exportURI -> assertEquals(targetUri, exportURI) },
                starter,
            ).toString(),
        )

        assertEquals("PROBE_ACCEPTED", command.getString("code"))
        assertTrue(command.getBoolean("ok"))
        assertTrue(command.getJSONObject("data").getBoolean("accepted"))
        assertEquals("task-2", command.getJSONObject("data").getString("task_id"))
        assertEquals(targetUri, command.getJSONObject("data").getString("export_path"))
        assertEquals(1, starter.intents.size)
        val intent = starter.intents[0]
        assertEquals("io.github.axuitomo.cfstgui.action.START_PROBE", intent.action)
        val normalizedPayload = JSONObject(intent.getStringExtra("payload").orEmpty())
        assertEquals(targetUri, normalizedPayload.getString("android_export_uri"))
        assertEquals(targetUri, intent.getStringExtra("export_uri"))
    }

    @Test
    fun startFailurePropagatesErrorAfterClearingQueuedStart() {
        val context = RuntimeEnvironment.getApplication()

        val error = assertThrows(IllegalStateException::class.java) {
            AndroidProbeStart.startProbe(
                context,
                "{}",
                "task-3",
                AndroidProbeStart.StartGate { true },
                AndroidProbeStart.ExportTargetValidator { _, _ -> },
                AndroidProbeStart.ForegroundServiceStarter {
                    throw IllegalStateException("boom")
                },
            )
        }

        assertEquals("boom", error.message)
    }

    private class RecordingStarter(private val fail: Boolean) : AndroidProbeStart.ForegroundServiceStarter {
        val intents: MutableList<Intent> = ArrayList()

        override fun startForegroundService(intent: Intent) {
            intents.add(intent)
            if (fail) {
                throw IllegalStateException("boom")
            }
        }
    }
}
