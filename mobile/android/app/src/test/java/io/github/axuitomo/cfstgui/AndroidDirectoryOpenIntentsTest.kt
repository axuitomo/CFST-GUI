package io.github.axuitomo.cfstgui

import org.junit.Assert.assertEquals
import org.junit.Assert.assertThrows
import org.junit.Assert.assertTrue
import org.junit.Test

class AndroidDirectoryOpenIntentsTest {
    @Test
    fun systemStorageManagerSpecUsesTreePickerWithInitialUri() {
        val spec = AndroidDirectoryOpenIntents.openDirectoryIntentSpecs(TREE_URI)[0]

        assertEquals(AndroidDirectoryOpenIntents.ACTION_OPEN_DOCUMENT_TREE, spec.action)
        assertEquals(TREE_URI, spec.initialUri)
        assertTrue(hasFlags(spec.flags, AndroidDirectoryOpenIntents.TREE_OPEN_FLAGS))
    }

    @Test
    fun directoryViewSpecTargetsTreeScopedDirectoryDocumentUri() {
        val spec = AndroidDirectoryOpenIntents.openDirectoryIntentSpecs(TREE_URI)[1]

        assertEquals(AndroidDirectoryOpenIntents.ACTION_VIEW, spec.action)
        assertEquals(DOCUMENT_URI, spec.dataUri)
        assertEquals(AndroidDirectoryOpenIntents.MIME_TYPE_DIRECTORY, spec.mimeType)
        assertTrue(hasFlags(spec.flags, AndroidDirectoryOpenIntents.DIRECTORY_VIEW_FLAGS))
    }

    @Test
    fun directoryOpenFallbackOrderUsesSystemThenViewThenChooser() {
        val specs = AndroidDirectoryOpenIntents.openDirectoryIntentSpecs(TREE_URI)

        assertEquals(3, specs.size)
        assertEquals(AndroidDirectoryOpenIntents.ACTION_OPEN_DOCUMENT_TREE, specs[0].action)
        assertEquals(AndroidDirectoryOpenIntents.ACTION_VIEW, specs[1].action)
        assertEquals(AndroidDirectoryOpenIntents.ACTION_CHOOSER, specs[2].action)

        val chooserTarget = requireNotNull(specs[2].chooserTarget)
        assertEquals(AndroidDirectoryOpenIntents.ACTION_VIEW, chooserTarget.action)
        assertEquals(DOCUMENT_URI, chooserTarget.dataUri)
    }

    @Test
    fun permissionLossUsesExplicitExportDirectoryMessage() {
        val error = assertThrows(IllegalStateException::class.java) {
            CfstPlugin.requireExportTreeUriPermission(false)
        }

        assertEquals(CfstPlugin.EXPORT_DIRECTORY_PERMISSION_LOST_MESSAGE, error.message)
    }

    private fun hasFlags(actualFlags: Int, expectedFlags: Int): Boolean {
        return actualFlags and expectedFlags == expectedFlags
    }

    companion object {
        private const val TREE_URI = "content://com.android.externalstorage.documents/tree/primary%3ADownload%2Fcf"
        private const val DOCUMENT_URI =
            "content://com.android.externalstorage.documents/tree/primary%3ADownload%2Fcf/document/primary%3ADownload%2Fcf"
    }
}
