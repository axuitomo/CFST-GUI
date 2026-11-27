package io.github.axuitomo.cfstgui

import android.content.Intent
import android.net.Uri
import android.provider.DocumentsContract
import androidx.core.content.IntentCompat
import org.junit.Assert.assertArrayEquals
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [35])
class AndroidPathSelectionTest {
    @Test
    fun normalizesSelectionModes() {
        assertEquals("source_file", AndroidPathSelection.normalizeMode(""))
        assertEquals("export_directory", AndroidPathSelection.normalizeMode(" Export-Directory "))
        assertTrue(AndroidPathSelection.isExportDirectoryMode("export_target"))
        assertTrue(AndroidPathSelection.isConfigImportMode("import_config"))
    }

    @Test
    fun buildsExportDirectoryPickerWithInitialUri() {
        val intent = AndroidPathSelection.pickerIntent(
            "export_dir",
            "",
            "content://com.android.externalstorage.documents/tree/primary%3ADownload",
        )

        assertEquals(Intent.ACTION_OPEN_DOCUMENT_TREE, intent.action)
        assertEquals(
            Uri.parse("content://com.android.externalstorage.documents/tree/primary%3ADownload"),
            IntentCompat.getParcelableExtra(intent, DocumentsContract.EXTRA_INITIAL_URI, Uri::class.java),
        )
        assertTrue(
            hasFlags(
                intent.flags,
                Intent.FLAG_GRANT_READ_URI_PERMISSION or
                    Intent.FLAG_GRANT_WRITE_URI_PERMISSION or
                    Intent.FLAG_GRANT_PERSISTABLE_URI_PERMISSION,
            ),
        )
    }

    @Test
    fun buildsConfigArchiveExportPicker() {
        val intent = AndroidPathSelection.pickerIntent("config_archive_export", "", "")

        assertEquals(Intent.ACTION_CREATE_DOCUMENT, intent.action)
        assertEquals("application/zip", intent.type)
        assertEquals("cfst-gui-config.zip", intent.getStringExtra(Intent.EXTRA_TITLE))
        assertTrue(intent.hasCategory(Intent.CATEGORY_OPENABLE))
    }

    @Test
    fun buildsConfigImportPickerMimeTypes() {
        val intent = AndroidPathSelection.pickerIntent("config_import", "", "")

        assertEquals(Intent.ACTION_OPEN_DOCUMENT, intent.action)
        assertEquals("*/*", intent.type)
        assertArrayEquals(
            arrayOf("application/json", "text/plain", "text/json"),
            requireNotNull(intent.getStringArrayExtra(Intent.EXTRA_MIME_TYPES)),
        )
    }

    private fun hasFlags(actualFlags: Int, expectedFlags: Int): Boolean {
        return actualFlags and expectedFlags == expectedFlags
    }
}
