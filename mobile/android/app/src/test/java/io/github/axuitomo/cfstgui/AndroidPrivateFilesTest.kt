package io.github.axuitomo.cfstgui

import android.net.Uri
import java.io.File
import java.nio.charset.StandardCharsets
import java.nio.file.Files
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RuntimeEnvironment
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [35])
class AndroidPrivateFilesTest {
    @Test
    fun sanitizesFileNames() {
        assertEquals(".._result.csv", AndroidPrivateFiles.sanitizeFileName("../result.csv"))
        assertEquals("a_b_c_.txt", AndroidPrivateFiles.sanitizeFileName(" a:b?c*.txt "))
        assertEquals("", AndroidPrivateFiles.sanitizeFileName(null))
    }

    @Test
    fun readsUriTextAndBase64() {
        val context = RuntimeEnvironment.getApplication()
        val source = File(context.cacheDir, "config.json")
        Files.write(source.toPath(), "{\"ok\":true}".toByteArray(StandardCharsets.UTF_8))
        val uri = Uri.fromFile(source)

        assertEquals("{\"ok\":true}", AndroidPrivateFiles.readUriText(context, uri))
        assertEquals("eyJvayI6dHJ1ZX0=", AndroidPrivateFiles.readUriBase64(context, uri))
    }

    @Test
    fun copiesImportUriToPrivateFile() {
        val context = RuntimeEnvironment.getApplication()
        val source = File(context.cacheDir, "source.csv")
        Files.write(source.toPath(), "ip,latency".toByteArray(StandardCharsets.UTF_8))

        val copied = AndroidPrivateFiles.copyImportUriToPrivateFile(context, Uri.fromFile(source), "../source.csv")

        assertTrue(requireNotNull(copied.parentFile).absolutePath.endsWith("/imports"))
        assertTrue(copied.name.endsWith("-.._source.csv"))
        assertEquals("ip,latency", String(Files.readAllBytes(copied.toPath()), StandardCharsets.UTF_8))
    }
}
