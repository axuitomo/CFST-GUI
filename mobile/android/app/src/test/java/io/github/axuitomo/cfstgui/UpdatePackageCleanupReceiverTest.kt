package io.github.axuitomo.cfstgui

import android.content.Intent
import java.io.File
import java.nio.charset.StandardCharsets
import java.nio.file.Files
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RuntimeEnvironment
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [35])
class UpdatePackageCleanupReceiverTest {
    @Test
    fun packageReplacementDeletesOnlyDownloadedUpdatePackages() {
        val context = RuntimeEnvironment.getApplication()
        val updateDir = AndroidUpdateInstaller.ensureUpdateDirectory(context)
        try {
            val apk = writeText(File(updateDir, "cfst-gui-android-release.apk"), "apk")
            val part = writeText(File(updateDir, "cfst-gui-android-release.apk.0.part"), "part")
            val notes = writeText(File(updateDir, "notes.txt"), "keep")

            UpdatePackageCleanupReceiver().onReceive(context, Intent(Intent.ACTION_MY_PACKAGE_REPLACED))

            assertFalse(apk.exists())
            assertFalse(part.exists())
            assertTrue(notes.exists())
        } finally {
            deleteRecursively(updateDir)
        }
    }

    @Test
    fun unrelatedBroadcastDoesNotCleanUpdatePackages() {
        val context = RuntimeEnvironment.getApplication()
        val updateDir = AndroidUpdateInstaller.ensureUpdateDirectory(context)
        try {
            val apk = writeText(File(updateDir, "cfst-gui-android-release.apk"), "apk")

            UpdatePackageCleanupReceiver().onReceive(context, Intent(Intent.ACTION_PACKAGE_REPLACED))

            assertTrue(apk.exists())
        } finally {
            deleteRecursively(updateDir)
        }
    }

    private fun writeText(target: File, value: String): File {
        val parent = target.parentFile
        if (parent != null && !parent.exists()) {
            assertTrue(parent.mkdirs())
        }
        Files.write(target.toPath(), value.toByteArray(StandardCharsets.UTF_8))
        return target
    }

    private fun deleteRecursively(file: File?) {
        if (file == null || !file.exists()) {
            return
        }
        if (file.isDirectory) {
            file.listFiles()?.forEach { child -> deleteRecursively(child) }
        }
        Files.deleteIfExists(file.toPath())
    }
}
