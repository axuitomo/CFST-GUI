package io.github.axuitomo.cfstgui

import android.content.Intent
import android.net.Uri
import java.io.File
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RuntimeEnvironment
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [35])
class AndroidUpdateInstallerTest {
    @Test
    fun ensureUpdateDirectoryCreatesPrivateUpdatesDirectory() {
        val context = RuntimeEnvironment.getApplication()

        val updateDir = AndroidUpdateInstaller.ensureUpdateDirectory(context)

        assertTrue(updateDir.isDirectory)
        assertEquals(File(context.filesDir, "updates").absolutePath, updateDir.absolutePath)
    }

    @Test
    fun fileProviderAuthorityUsesApplicationPackage() {
        val context = RuntimeEnvironment.getApplication()

        assertEquals(context.packageName + ".fileprovider", AndroidUpdateInstaller.fileProviderAuthority(context))
    }

    @Test
    fun installIntentForUriUsesApkMimeTypeAndGrantsReadAccess() {
        val apkUri = Uri.parse("content://io.github.axuitomo.cfstgui.fileprovider/updates/cfst-gui-android-release.apk")

        val intent = AndroidUpdateInstaller.installIntentForUri(apkUri)

        assertEquals(Intent.ACTION_VIEW, intent.action)
        assertEquals("application/vnd.android.package-archive", intent.type)
        assertEquals(apkUri, intent.data)
        assertTrue(hasFlags(intent.flags, Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_GRANT_READ_URI_PERMISSION))
    }

    private fun hasFlags(actualFlags: Int, expectedFlags: Int): Boolean {
        return actualFlags and expectedFlags == expectedFlags
    }
}
