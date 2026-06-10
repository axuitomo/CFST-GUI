package io.github.axuitomo.cfstgui

import android.content.Intent
import android.provider.Settings
import org.junit.Assert.assertEquals
import org.junit.Assert.assertThrows
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RuntimeEnvironment
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.shadows.ShadowBuild

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [35])
class AndroidBatterySettingsTest {
    @Test
    fun settingsModeOpensBatteryOptimizationSettings() {
        val intent = AndroidBatterySettings.settingsIntent(TestContext("io.github.axuitomo.cfstgui"), "settings")

        assertEquals(Settings.ACTION_IGNORE_BATTERY_OPTIMIZATION_SETTINGS, intent.action)
        assertTrue(hasFlags(intent.flags, Intent.FLAG_ACTIVITY_NEW_TASK))
    }

    @Test
    fun requestModeTargetsCurrentPackage() {
        val intent = AndroidBatterySettings.settingsIntent(TestContext("io.github.axuitomo.cfstgui"), "request")

        assertEquals(Settings.ACTION_REQUEST_IGNORE_BATTERY_OPTIMIZATIONS, intent.action)
        assertEquals("package:io.github.axuitomo.cfstgui", intent.dataString)
    }

    @Test
    fun detailsModeTargetsApplicationDetails() {
        val intent = AndroidBatterySettings.settingsIntent(TestContext("io.github.axuitomo.cfstgui"), "details")

        assertEquals(Settings.ACTION_APPLICATION_DETAILS_SETTINGS, intent.action)
        assertEquals("package:io.github.axuitomo.cfstgui", intent.dataString)
    }

    @Test
    fun statusPayloadIncludesStableAndroidBatteryFields() {
        val payload = AndroidBatterySettings.statusPayload(RuntimeEnvironment.getApplication())

        val ignoring = payload.getBoolean("ignoring_optimizations", false) == true
        assertTrue(payload.getBoolean("supported", false) == true)
        assertEquals(!ignoring, payload.getBoolean("needs_guidance", true))
        assertTrue(payload.has("manufacturer"))
        assertTrue(payload.has("brand"))
        assertTrue(payload.has("model"))
        assertTrue(requireNotNull(payload.getString("settings_hint", "")).isNotEmpty())
    }

    @Test
    fun settingsModePrefersManufacturerSettingsWhenAvailable() {
        ShadowBuild.setManufacturer("xiaomi")
        val starter = RecordingStarter(true)

        AndroidBatterySettings.openSettings(TestContext("io.github.axuitomo.cfstgui"), "settings", starter)

        assertEquals(1, starter.intents.size)
        assertEquals(
            "com.miui.securitycenter/com.miui.permcenter.autostart.AutoStartManagementActivity",
            requireNotNull(starter.intents[0].component).flattenToString(),
        )
        assertTrue(hasFlags(starter.intents[0].flags, Intent.FLAG_ACTIVITY_NEW_TASK))
    }

    @Test
    fun fallsBackToSystemBatterySettingsAfterManufacturerIntentFails() {
        ShadowBuild.setManufacturer("xiaomi")
        val starter = RecordingStarter(false, false, true)

        AndroidBatterySettings.openSettings(TestContext("io.github.axuitomo.cfstgui"), "settings", starter)

        assertEquals(3, starter.intents.size)
        assertEquals(
            "com.miui.securitycenter/com.miui.permcenter.autostart.AutoStartManagementActivity",
            requireNotNull(starter.intents[0].component).flattenToString(),
        )
        assertEquals("com.miui.securitycenter/com.miui.powercenter.PowerSettings", requireNotNull(starter.intents[1].component).flattenToString())
        assertEquals(Settings.ACTION_IGNORE_BATTERY_OPTIMIZATION_SETTINGS, starter.intents[2].action)
    }

    @Test
    fun throwsWhenNoBatterySettingsIntentCanStart() {
        ShadowBuild.setManufacturer("")
        val starter = RecordingStarter(false)

        val error = assertThrows(IllegalStateException::class.java) {
            AndroidBatterySettings.openSettings(TestContext("io.github.axuitomo.cfstgui"), "details", starter)
        }

        assertEquals("系统无法打开省电策略设置。", error.message)
        assertEquals(Settings.ACTION_APPLICATION_DETAILS_SETTINGS, starter.intents[0].action)
    }

    private fun hasFlags(actualFlags: Int, expectedFlags: Int): Boolean {
        return actualFlags and expectedFlags == expectedFlags
    }

    private class RecordingStarter(vararg results: Boolean) : AndroidBatterySettings.IntentStarter {
        private val results = results.toList()
        private var index = 0
        val intents: MutableList<Intent> = ArrayList()

        override fun tryStart(intent: Intent): Boolean {
            intents.add(intent)
            if (index >= results.size) {
                return false
            }
            return results[index++]
        }
    }
}
