package io.github.axuitomo.cfstgui

import android.content.Intent
import org.json.JSONObject
import org.junit.Assert.assertEquals
import org.junit.Assert.assertThrows
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RuntimeEnvironment
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [35])
class AndroidExternalNavigationTest {
    @Test
    fun releasePageStartsViewIntentAndReturnsCompatibleCommand() {
        val starter = RecordingActivityStarter()

        val command = JSONObject(
            AndroidExternalNavigation.openReleasePageCommand(
                RuntimeEnvironment.getApplication(),
                starter,
            ).toString(),
        )

        assertEquals("RELEASE_OPENED", command.getString("code"))
        assertTrue(command.getBoolean("ok"))
        assertEquals(AndroidUpdateRelease.RELEASE_PAGE_URL, command.getJSONObject("data").getString("release_url"))
        assertEquals(1, starter.intents.size)
        val intent = starter.intents[0]
        assertEquals(Intent.ACTION_VIEW, intent.action)
        assertEquals(AndroidUpdateRelease.RELEASE_PAGE_URL, intent.dataString)
        assertTrue(hasFlags(intent.flags, Intent.FLAG_ACTIVITY_NEW_TASK))
    }

    @Test
    fun openPathReturnsCompatibleCommandAfterStartingIntent() {
        val starter = AndroidTargetOpenerTestStarter(true)

        val command = JSONObject(
            AndroidExternalNavigation.openPathCommand(
                RuntimeEnvironment.getApplication(),
                " https://example.test/result ",
                starter,
            ).toString(),
        )

        assertEquals("OPEN_PATH_OK", command.getString("code"))
        assertTrue(command.getBoolean("ok"))
        assertEquals("https://example.test/result", command.getJSONObject("data").getString("target_path"))
        assertEquals(1, starter.intents.size)
        assertEquals(Intent.ACTION_VIEW, starter.intents[0].action)
        assertEquals("https://example.test/result", starter.intents[0].dataString)
    }

    @Test
    fun openPathRejectsMissingTargetBeforeStartingIntent() {
        val starter = AndroidTargetOpenerTestStarter(true)

        val error = assertThrows(IllegalArgumentException::class.java) {
            AndroidExternalNavigation.openPathCommand(RuntimeEnvironment.getApplication(), " ", starter)
        }

        assertEquals("缺少可打开的目标路径。", error.message)
        assertTrue(starter.intents.isEmpty())
    }

    private fun hasFlags(actualFlags: Int, expectedFlags: Int): Boolean {
        return actualFlags and expectedFlags == expectedFlags
    }

    private class RecordingActivityStarter : AndroidExternalNavigation.ActivityStarter {
        val intents: MutableList<Intent> = ArrayList()

        override fun startActivity(intent: Intent) {
            intents.add(intent)
        }
    }

    private class AndroidTargetOpenerTestStarter(vararg results: Boolean) : AndroidTargetOpener.IntentStarter {
        private val results = results.toList()
        private var index = 0
        val intents: MutableList<Intent> = ArrayList()

        override fun tryStart(intent: Intent): Boolean {
            intents.add(intent)
            if (index >= results.size) {
                return false
            }
            return results[index++]
        }
    }
}
