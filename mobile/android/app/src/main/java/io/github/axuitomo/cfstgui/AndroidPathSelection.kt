package io.github.axuitomo.cfstgui

import android.content.Intent
import android.net.Uri
import android.provider.DocumentsContract
import java.util.Locale

object AndroidPathSelection {
    private val CONFIG_ARCHIVE_IMPORT_MIME_TYPES = arrayOf(
        "application/zip",
        "application/octet-stream",
        "application/json",
        "text/plain",
        "text/json",
    )
    private val CONFIG_IMPORT_MIME_TYPES = arrayOf("application/json", "text/plain", "text/json")
    private val SOURCE_FILE_MIME_TYPES = arrayOf("text/plain", "text/csv", "application/octet-stream", "*/*")

    @JvmStatic
    fun normalizeMode(mode: String?): String {
        if (mode == null || mode.trim().isEmpty()) {
            return "source_file"
        }
        return mode.trim().lowercase(Locale.ROOT).replace('-', '_')
    }

    @JvmStatic
    fun pickerIntent(mode: String?, defaultFileName: String?, currentPath: String?): Intent {
        val normalizedMode = normalizeMode(mode)
        val intent = when {
            isExportDirectoryMode(normalizedMode) -> Intent(Intent.ACTION_OPEN_DOCUMENT_TREE).apply {
                putInitialUriExtra(currentPath)
            }
            isExportFileMode(normalizedMode) ||
                isConfigExportMode(normalizedMode) ||
                isConfigArchiveExportMode(normalizedMode) -> Intent(Intent.ACTION_CREATE_DOCUMENT).apply {
                    addCategory(Intent.CATEGORY_OPENABLE)
                    type = createDocumentMimeType(normalizedMode)
                    putExtra(Intent.EXTRA_TITLE, exportFileName(normalizedMode, defaultFileName))
                }
            else -> Intent(Intent.ACTION_OPEN_DOCUMENT).apply {
                addCategory(Intent.CATEGORY_OPENABLE)
                type = "*/*"
                putExtra(Intent.EXTRA_MIME_TYPES, importMimeTypes(normalizedMode))
            }
        }
        intent.addFlags(
            Intent.FLAG_GRANT_READ_URI_PERMISSION or
                Intent.FLAG_GRANT_WRITE_URI_PERMISSION or
                Intent.FLAG_GRANT_PERSISTABLE_URI_PERMISSION,
        )
        return intent
    }

    @JvmStatic
    fun isStorageDirMode(mode: String?): Boolean = normalizeMode(mode) == "storage_dir"

    @JvmStatic
    fun isExportDirectoryMode(mode: String?): Boolean =
        when (normalizeMode(mode)) {
            "export_target", "export_dir", "export_directory" -> true
            else -> false
        }

    @JvmStatic
    fun isExportFileMode(mode: String?): Boolean =
        when (normalizeMode(mode)) {
            "export_file", "save_file" -> true
            else -> false
        }

    @JvmStatic
    fun isConfigExportMode(mode: String?): Boolean = normalizeMode(mode) == "config_export"

    @JvmStatic
    fun isConfigArchiveExportMode(mode: String?): Boolean = normalizeMode(mode) == "config_archive_export"

    @JvmStatic
    fun isConfigImportMode(mode: String?): Boolean =
        when (normalizeMode(mode)) {
            "config_import", "import_config" -> true
            else -> false
        }

    @JvmStatic
    fun isConfigArchiveImportMode(mode: String?): Boolean = normalizeMode(mode) == "config_archive_import"

    @JvmStatic
    fun exportFileName(mode: String?, requestedName: String?): String {
        val requested = requestedName?.trim().orEmpty()
        if (requested.isNotEmpty()) {
            return requested
        }
        return when {
            isConfigArchiveExportMode(mode) -> "cfst-gui-config.zip"
            isConfigExportMode(mode) -> "cfst-gui-config.json"
            else -> "result.csv"
        }
    }

    @JvmStatic
    fun createDocumentMimeType(mode: String?): String =
        when {
            isConfigArchiveExportMode(mode) -> "application/zip"
            isConfigExportMode(mode) -> "application/json"
            else -> "text/csv"
        }

    private fun Intent.putInitialUriExtra(currentPath: String?) {
        val normalized = currentPath?.trim().orEmpty()
        if (!normalized.startsWith("content://")) {
            return
        }
        try {
            putExtra(DocumentsContract.EXTRA_INITIAL_URI, Uri.parse(normalized))
        } catch (_: Exception) {
            // Initial URI is only a picker hint; ignore malformed saved values.
        }
    }

    private fun importMimeTypes(mode: String?): Array<String> =
        when {
            isConfigArchiveImportMode(mode) -> CONFIG_ARCHIVE_IMPORT_MIME_TYPES
            isConfigImportMode(mode) -> CONFIG_IMPORT_MIME_TYPES
            else -> SOURCE_FILE_MIME_TYPES
        }
}
