package io.github.axuitomo.cfstgui

import java.io.File
import java.util.Locale

object AndroidUpdatePackages {
    @JvmStatic
    fun cleanup(updateDir: File?): Int {
        if (updateDir == null || !updateDir.isDirectory) {
            return 0
        }
        val files = updateDir.listFiles() ?: return 0
        var deleted = 0
        for (file in files) {
            if (file == null || !file.isFile || !isUpdatePackageFile(file.name)) {
                continue
            }
            if (file.delete()) {
                deleted++
            }
        }
        return deleted
    }

    @JvmStatic
    fun isUpdatePackageFile(name: String?): Boolean {
        val normalized = name?.trim()?.lowercase(Locale.ROOT).orEmpty()
        return normalized.startsWith("cfst-gui") &&
            (normalized.endsWith(".apk") || normalized.matches(Regex(".*\\.apk\\.\\d+\\.part$")))
    }
}
