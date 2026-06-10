package io.github.axuitomo.cfstgui

import android.content.Intent
import android.net.Uri
import androidx.core.content.IntentCompat
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
class AndroidTargetOpenerTest {
    @Test
    fun opensHttpLinksWithViewIntent() {
        val starter = RecordingStarter(true)

        AndroidTargetOpener.openTargetPath(RuntimeEnvironment.getApplication(), " https://example.test/release ", starter)

        assertEquals(1, starter.intents.size)
        val intent = starter.intents[0]
        assertEquals(Intent.ACTION_VIEW, intent.action)
        assertEquals("https://example.test/release", intent.dataString)
        assertTrue(hasFlags(intent.flags, Intent.FLAG_ACTIVITY_NEW_TASK))
    }

    @Test
    fun contentFileFallsBackFromViewToShareChooser() {
        val starter = RecordingStarter(false, true)

        AndroidTargetOpener.openTargetPath(
            RuntimeEnvironment.getApplication(),
            "content://example.test/result.csv",
            starter,
        )

        assertEquals(2, starter.intents.size)
        assertEquals(Intent.ACTION_VIEW, starter.intents[0].action)
        assertEquals(Intent.ACTION_CHOOSER, starter.intents[1].action)
        val chooserTarget = requireNotNull(IntentCompat.getParcelableExtra(starter.intents[1], Intent.EXTRA_INTENT, Intent::class.java))
        assertEquals(Intent.ACTION_SEND, chooserTarget.action)
        assertEquals(Uri.parse("content://example.test/result.csv"), IntentCompat.getParcelableExtra(chooserTarget, Intent.EXTRA_STREAM, Uri::class.java))
    }

    @Test
    fun missingTargetPathIsRejectedBeforeStartingIntent() {
        val starter = RecordingStarter(true)

        val error = assertThrows(IllegalArgumentException::class.java) {
            AndroidTargetOpener.openTargetPath(RuntimeEnvironment.getApplication(), "  ", starter)
        }

        assertEquals("缺少可打开的目标路径。", error.message)
        assertTrue(starter.intents.isEmpty())
    }

    @Test
    fun localPrivatePathsAreRejected() {
        val starter = RecordingStarter(true)

        val error = assertThrows(IllegalStateException::class.java) {
            AndroidTargetOpener.openTargetPath(RuntimeEnvironment.getApplication(), "/data/user/0/app/result.csv", starter)
        }

        assertEquals("Android 端暂不直接打开应用私有目录，请先导出文件，或打开已授权的导出目录/导出文件。", error.message)
        assertTrue(starter.intents.isEmpty())
    }

    @Test
    fun exportTreePermissionMessageStaysCompatible() {
        val error = assertThrows(IllegalStateException::class.java) {
            AndroidTargetOpener.requireExportTreeUriPermission(false)
        }

        assertEquals(CfstPlugin.EXPORT_DIRECTORY_PERMISSION_LOST_MESSAGE, error.message)
    }

    @Test
    fun nullUriHasNoPersistedPermission() {
        val context = RuntimeEnvironment.getApplication()

        assertEquals(false, AndroidTargetOpener.hasPersistedUriPermission(context, null))
    }

    private fun hasFlags(actualFlags: Int, expectedFlags: Int): Boolean {
        return actualFlags and expectedFlags == expectedFlags
    }

    private class RecordingStarter(vararg results: Boolean) : AndroidTargetOpener.IntentStarter {
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
