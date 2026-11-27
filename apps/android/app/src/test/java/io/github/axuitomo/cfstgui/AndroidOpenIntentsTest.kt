package io.github.axuitomo.cfstgui

import android.content.Intent
import android.net.Uri
import androidx.core.content.IntentCompat
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [35])
class AndroidOpenIntentsTest {
    @Test
    fun mimeTypeForNameHandlesKnownExportTypes() {
        assertEquals("text/csv", AndroidOpenIntents.mimeTypeForName("result.csv"))
        assertEquals("application/json", AndroidOpenIntents.mimeTypeForName("config.json"))
        assertEquals("text/plain", AndroidOpenIntents.mimeTypeForName("debug.txt"))
        assertEquals("application/zip", AndroidOpenIntents.mimeTypeForName("backup.zip"))
        assertEquals("application/octet-stream", AndroidOpenIntents.mimeTypeForName("payload.bin"))
    }

    @Test
    fun linkIntentUsesViewActionAndNewTaskFlag() {
        val intent = AndroidOpenIntents.linkIntent(Uri.parse("https://example.test/release"))

        assertEquals(Intent.ACTION_VIEW, intent.action)
        assertEquals("https://example.test/release", intent.dataString)
        assertTrue(hasFlags(intent.flags, Intent.FLAG_ACTIVITY_NEW_TASK))
    }

    @Test
    fun viewFileIntentGrantsReadAccess() {
        val uri = Uri.parse("content://example.test/result.csv")
        val intent = AndroidOpenIntents.viewFileIntent(uri, "text/csv")

        assertEquals(Intent.ACTION_VIEW, intent.action)
        assertEquals(uri, intent.data)
        assertEquals("text/csv", intent.type)
        assertTrue(hasFlags(intent.flags, Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_GRANT_READ_URI_PERMISSION))
    }

    @Test
    fun shareFileChooserWrapsSendIntent() {
        val uri = Uri.parse("content://example.test/result.csv")
        val chooser = AndroidOpenIntents.shareFileChooserIntent(uri, "text/csv")
        val target = requireNotNull(IntentCompat.getParcelableExtra(chooser, Intent.EXTRA_INTENT, Intent::class.java))

        assertEquals(Intent.ACTION_CHOOSER, chooser.action)
        assertTrue(hasFlags(chooser.flags, Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_GRANT_READ_URI_PERMISSION))
        assertEquals(Intent.ACTION_SEND, target.action)
        assertEquals("text/csv", target.type)
        assertEquals(uri, IntentCompat.getParcelableExtra(target, Intent.EXTRA_STREAM, Uri::class.java))
    }

    private fun hasFlags(actualFlags: Int, expectedFlags: Int): Boolean {
        return actualFlags and expectedFlags == expectedFlags
    }
}
