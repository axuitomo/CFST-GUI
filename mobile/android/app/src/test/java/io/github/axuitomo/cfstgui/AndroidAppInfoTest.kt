package io.github.axuitomo.cfstgui

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

class AndroidAppInfoTest {
    @Test
    fun appInfoPayloadFallsBackToAndroidApkDefaults() {
        val payload = AndroidAppInfo.appInfoPayload(TestContext("io.github.axuitomo.cfstgui"))

        assertEquals("1.0", payload.getString("current_version"))
        assertEquals("android_apk", payload.getString("install_mode"))
        assertEquals("android", payload.getString("platform"))
        assertEquals(AndroidUpdateRelease.RELEASE_PAGE_URL, payload.getString("release_url"))
        assertTrue(payload.getBoolean("battery_optimization_supported", false) == true)
    }
}
