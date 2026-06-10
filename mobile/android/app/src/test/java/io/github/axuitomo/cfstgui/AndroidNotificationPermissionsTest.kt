package io.github.axuitomo.cfstgui

import android.Manifest
import com.getcapacitor.PermissionState
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RuntimeEnvironment
import org.robolectric.RobolectricTestRunner
import org.robolectric.Shadows
import org.robolectric.annotation.Config

@RunWith(RobolectricTestRunner::class)
class AndroidNotificationPermissionsTest {
    @Test
    @Config(sdk = [32])
    fun preAndroid13DoesNotRequireRuntimeNotificationPermission() {
        val context = RuntimeEnvironment.getApplication()

        val payload = AndroidNotificationPermissions.statusPayload(context, PermissionState.DENIED.toString(), null)

        assertFalse(AndroidNotificationPermissions.supported())
        assertTrue(AndroidNotificationPermissions.granted(context))
        assertFalse(payload.getBoolean("supported", true) == true)
        assertTrue(payload.getBoolean("granted", false) == true)
        assertEquals(PermissionState.GRANTED.toString(), payload.getString("state"))
        assertFalse(payload.getBoolean("should_show_rationale", true) == true)
        assertEquals("当前 Android 版本无需运行时通知权限。", payload.getString("message"))
    }

    @Test
    @Config(sdk = [35])
    fun android13AndLaterReportsGrantedNotificationPermission() {
        val context = RuntimeEnvironment.getApplication()
        Shadows.shadowOf(RuntimeEnvironment.getApplication()).grantPermissions(Manifest.permission.POST_NOTIFICATIONS)

        val payload = AndroidNotificationPermissions.statusPayload(context, PermissionState.GRANTED.toString(), null)

        assertTrue(AndroidNotificationPermissions.supported())
        assertTrue(AndroidNotificationPermissions.granted(context))
        assertTrue(payload.getBoolean("supported", false) == true)
        assertTrue(payload.getBoolean("granted", false) == true)
        assertEquals(PermissionState.GRANTED.toString(), payload.getString("state"))
        assertFalse(payload.getBoolean("should_show_rationale", true) == true)
        assertEquals("通知权限已允许。", payload.getString("message"))
    }
}
