package io.github.axuitomo.cfstgui

import android.content.Intent
import android.net.Uri
import android.provider.DocumentsContract
import java.util.Collections

class AndroidDirectoryOpenIntents private constructor() {
    companion object {
        @JvmField val ACTION_OPEN_DOCUMENT_TREE: String = Intent.ACTION_OPEN_DOCUMENT_TREE
        @JvmField val ACTION_VIEW: String = Intent.ACTION_VIEW
        @JvmField val ACTION_CHOOSER: String = Intent.ACTION_CHOOSER
        @JvmField val CHOOSER_TITLE: String = "打开储存目录"
        @JvmField val EXTRA_INITIAL_URI: String = DocumentsContract.EXTRA_INITIAL_URI
        @JvmField val EXTRA_INTENT: String = Intent.EXTRA_INTENT
        @JvmField val MIME_TYPE_DIRECTORY: String = DocumentsContract.Document.MIME_TYPE_DIR
        @JvmField
        val TREE_OPEN_FLAGS: Int = Intent.FLAG_ACTIVITY_NEW_TASK or
            Intent.FLAG_GRANT_READ_URI_PERMISSION or
            Intent.FLAG_GRANT_WRITE_URI_PERMISSION or
            Intent.FLAG_GRANT_PERSISTABLE_URI_PERMISSION or
            Intent.FLAG_GRANT_PREFIX_URI_PERMISSION
        @JvmField
        val DIRECTORY_VIEW_FLAGS: Int = Intent.FLAG_ACTIVITY_NEW_TASK or
            Intent.FLAG_GRANT_READ_URI_PERMISSION or
            Intent.FLAG_GRANT_WRITE_URI_PERMISSION or
            Intent.FLAG_GRANT_PREFIX_URI_PERMISSION

        @JvmStatic
        fun openDirectoryIntents(treeUri: Uri): List<Intent> =
            Collections.unmodifiableList(openDirectoryIntentSpecs(treeUri.toString()).map { it.toIntent() })

        @JvmStatic
        fun openDirectoryIntentSpecs(treeUri: String?): List<IntentSpec> {
            val viewSpec = directoryViewIntentSpec(treeUri)
            return Collections.unmodifiableList(
                listOf(
                    systemStorageManagerIntentSpec(treeUri),
                    viewSpec,
                    directoryChooserIntentSpec(viewSpec),
                ),
            )
        }

        @JvmStatic
        fun systemStorageManagerIntent(treeUri: Uri): Intent =
            systemStorageManagerIntentSpec(treeUri.toString()).toIntent()

        @JvmStatic
        fun directoryViewIntent(treeUri: Uri): Intent =
            directoryViewIntentSpec(treeUri.toString()).toIntent()

        @JvmStatic
        fun directoryChooserIntent(viewIntent: Intent): Intent =
            Intent.createChooser(viewIntent, CHOOSER_TITLE).apply {
                addFlags(DIRECTORY_VIEW_FLAGS)
            }

        @JvmStatic
        fun directoryDocumentUri(treeUri: Uri): Uri =
            Uri.parse(directoryDocumentUriString(treeUri.toString()))

        private fun systemStorageManagerIntentSpec(treeUri: String?): IntentSpec =
            IntentSpec(ACTION_OPEN_DOCUMENT_TREE, "", "", treeUri.orEmpty(), TREE_OPEN_FLAGS, "", null)

        private fun directoryViewIntentSpec(treeUri: String?): IntentSpec =
            IntentSpec(ACTION_VIEW, directoryDocumentUriString(treeUri), MIME_TYPE_DIRECTORY, "", DIRECTORY_VIEW_FLAGS, "", null)

        private fun directoryChooserIntentSpec(viewSpec: IntentSpec): IntentSpec =
            IntentSpec(ACTION_CHOOSER, "", "", "", DIRECTORY_VIEW_FLAGS, CHOOSER_TITLE, viewSpec)

        private fun directoryDocumentUriString(treeUri: String?): String {
            val normalized = treeUri?.trim().orEmpty()
            val schemeIndex = normalized.indexOf("://")
            val authorityEnd = if (schemeIndex < 0) -1 else normalized.indexOf('/', schemeIndex + 3)
            val treeIndex = normalized.indexOf("/tree/", authorityEnd)
            if (authorityEnd < 0 || treeIndex < 0) {
                return normalized
            }
            val authorityPrefix = normalized.substring(0, authorityEnd)
            var treeDocumentId = normalized.substring(treeIndex + "/tree/".length)
            val queryIndex = firstSuffixIndex(treeDocumentId, '?', '#')
            if (queryIndex >= 0) {
                treeDocumentId = treeDocumentId.substring(0, queryIndex)
            }
            return "$authorityPrefix/tree/$treeDocumentId/document/$treeDocumentId"
        }

        private fun firstSuffixIndex(value: String, first: Char, second: Char): Int {
            val firstIndex = value.indexOf(first)
            val secondIndex = value.indexOf(second)
            return when {
                firstIndex < 0 -> secondIndex
                secondIndex < 0 -> firstIndex
                else -> minOf(firstIndex, secondIndex)
            }
        }
    }

    class IntentSpec internal constructor(
        @JvmField val action: String,
        @JvmField val dataUri: String,
        @JvmField val mimeType: String,
        @JvmField val initialUri: String,
        @JvmField val flags: Int,
        @JvmField val chooserTitle: String,
        @JvmField val chooserTarget: IntentSpec?,
    ) {
        fun toIntent(): Intent {
            if (ACTION_CHOOSER == action) {
                return Intent.createChooser(chooserTarget!!.toIntent(), chooserTitle).apply {
                    addFlags(flags)
                }
            }
            return Intent(action).apply {
                if (initialUri.isNotEmpty()) {
                    putExtra(EXTRA_INITIAL_URI, Uri.parse(initialUri))
                }
                if (dataUri.isNotEmpty()) {
                    setDataAndType(Uri.parse(dataUri), mimeType)
                }
                addFlags(flags)
            }
        }
    }
}
