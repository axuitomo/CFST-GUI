package io.github.axuitomo.cfstgui

import java.io.File
import java.nio.file.AtomicMoveNotSupportedException
import java.nio.file.Files
import java.nio.file.StandardCopyOption

object AndroidStorageMigration {
    private val legacyMirrorRootFiles = arrayOf(
        "cfip-log.txt",
        "cloudflare-colo-locations.json",
        "cloudflare-colos-ipv4.csv",
        "cloudflare-colos-ipv6.csv",
        "cloudflare-colos.csv",
        "cloudflare-countries.json",
        "local-ip-ranges.csv",
        "mobile-config.json",
        "source-profiles.json",
    )
    private val legacyMirrorRootDirectories = arrayOf("backups", "exports", "imports", "tasks")

    class LegacyMirrorMigrationResult {
        @JvmField
        var attempted = false

        @JvmField
        var completed = false

        @JvmField
        val copied: MutableList<String> = ArrayList()

        @JvmField
        val failed: MutableList<String> = ArrayList()

        @JvmField
        val skipped: MutableList<String> = ArrayList()
    }

    @JvmStatic
    fun migrateLegacySafMirrorFiles(
        mirrorDir: File?,
        targetDir: File?,
        overwriteExisting: Boolean = true,
    ): LegacyMirrorMigrationResult {
        val result = LegacyMirrorMigrationResult()
        if (mirrorDir == null ||
            targetDir == null ||
            !mirrorDir.isDirectory ||
            sameCanonicalFile(mirrorDir, targetDir) ||
            !hasKnownData(mirrorDir)
        ) {
            return result
        }
        result.attempted = true
        if (!targetDir.exists() && !targetDir.mkdirs()) {
            result.failed.add("创建应用私有目录失败：" + targetDir.absolutePath)
            result.completed = false
            return result
        }
        for (name in legacyMirrorRootFiles) {
            copyLegacyMirrorEntry(File(mirrorDir, name), File(targetDir, name), name, overwriteExisting, result)
        }
        for (name in legacyMirrorRootDirectories) {
            copyLegacyMirrorEntry(File(mirrorDir, name), File(targetDir, name), name, overwriteExisting, result)
        }
        result.completed = result.failed.isEmpty()
        return result
    }

    @JvmStatic
    fun hasKnownData(mirrorDir: File?): Boolean {
        if (mirrorDir == null || !mirrorDir.isDirectory) {
            return false
        }
        for (name in legacyMirrorRootFiles) {
            if (File(mirrorDir, name).exists()) {
                return true
            }
        }
        for (name in legacyMirrorRootDirectories) {
            if (File(mirrorDir, name).exists()) {
                return true
            }
        }
        return false
    }

    @JvmStatic
    fun joinMessages(values: List<String>?): String {
        if (values == null) {
            return ""
        }
        val builder = StringBuilder()
        for (value in values) {
            val normalized = value.trim()
            if (normalized.isEmpty()) {
                continue
            }
            if (builder.isNotEmpty()) {
                builder.append('；')
            }
            builder.append(normalized)
        }
        return builder.toString()
    }

    private fun copyLegacyMirrorEntry(
        source: File,
        target: File,
        relativePath: String,
        overwriteExisting: Boolean,
        result: LegacyMirrorMigrationResult,
    ) {
        if (!source.exists()) {
            return
        }
        if (source.isDirectory) {
            if (target.exists() && !target.isDirectory) {
                result.skipped.add(relativePath)
                return
            }
            if (!target.exists() && !target.mkdirs()) {
                result.failed.add("$relativePath: 创建目录失败")
                return
            }
            val children = source.listFiles()
            if (children == null || children.isEmpty()) {
                result.skipped.add(relativePath)
                return
            }
            for (child in children) {
                copyLegacyMirrorEntry(child, File(target, child.name), "$relativePath/${child.name}", overwriteExisting, result)
            }
            return
        }
        if (!overwriteExisting && target.exists()) {
            result.skipped.add(relativePath)
            return
        }
        val parent = target.parentFile
        if (parent != null && !parent.exists() && !parent.mkdirs()) {
            result.failed.add("$relativePath: 创建父目录失败")
            return
        }
        try {
            copyLegacyFile(source, target)
            result.copied.add(relativePath)
        } catch (error: Exception) {
            result.failed.add(relativePath + ": " + error.message)
        }
    }

    private fun copyLegacyFile(source: File, target: File) {
        val parent = requireNotNull(target.parentFile)
        val temporary = Files.createTempFile(parent.toPath(), ".${target.name}.", ".tmp").toFile()
        try {
            Files.copy(source.toPath(), temporary.toPath(), StandardCopyOption.REPLACE_EXISTING)
            try {
                Files.move(temporary.toPath(), target.toPath(), StandardCopyOption.REPLACE_EXISTING, StandardCopyOption.ATOMIC_MOVE)
            } catch (_: AtomicMoveNotSupportedException) {
                Files.move(temporary.toPath(), target.toPath(), StandardCopyOption.REPLACE_EXISTING)
            }
        } finally {
            temporary.delete()
        }
    }

    private fun sameCanonicalFile(left: File, right: File): Boolean {
        return try {
            left.canonicalFile == right.canonicalFile
        } catch (_: Exception) {
            left.absoluteFile == right.absoluteFile
        }
    }
}
