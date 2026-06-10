package io.github.axuitomo.cfstgui

import java.nio.charset.StandardCharsets
import java.nio.file.Files
import org.junit.Assert.assertEquals
import org.junit.Assert.assertThrows
import org.junit.Test

class AndroidUpdateIntegrityTest {
    @Test
    fun comparesNormalizedSemanticVersions() {
        assertEquals("1.2.3", AndroidUpdateIntegrity.normalizeVersion(" v1.2.3 "))
        assertEquals(1, AndroidUpdateIntegrity.compareVersions("v1.2.4", "1.2.3"))
        assertEquals(0, AndroidUpdateIntegrity.compareVersions("1.2.0", "1.2"))
        assertEquals(-1, AndroidUpdateIntegrity.compareVersions("1.2.0-beta.1", "1.3.0"))
        assertEquals(1, AndroidUpdateIntegrity.compareVersions("2.0a", "1.9.9"))
    }

    @Test
    fun formatsSignedBytesAsTwoDigitHex() {
        assertEquals("00ff7f80", AndroidUpdateIntegrity.bytesToHex(byteArrayOf(0, (-1).toByte(), 127, (-128).toByte())))
    }

    @Test
    fun verifiesSHA256CaseInsensitively() {
        val file = Files.createTempFile("cfst-sha256", ".txt").toFile()
        try {
            Files.write(file.toPath(), "abc".toByteArray(StandardCharsets.UTF_8))

            AndroidUpdateIntegrity.verifySHA256(
                file,
                "BA7816BF8F01CFEA414140DE5DAE2223B00361A396177A9CB410FF61F20015AD",
            )
            assertThrows(IllegalStateException::class.java) {
                AndroidUpdateIntegrity.verifySHA256(file, "deadbeef")
            }
        } finally {
            Files.deleteIfExists(file.toPath())
        }
    }
}
