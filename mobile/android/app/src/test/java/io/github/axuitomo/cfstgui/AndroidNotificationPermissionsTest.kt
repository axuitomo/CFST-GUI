package io.github.axuitomo.cfstgui

import android.Manifest
import android.provider.Settings
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
        assertFalse(payload.getBoolean("can_request", true) == true)
        assertFalse(payload.getBoolean("open_settings_recommended", true) == true)
        assertFalse(payload.getBoolean("request_already_attempted", true) == true)
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
        assertFalse(payload.getBoolean("can_request", true) == true)
        assertFalse(payload.getBoolean("open_settings_recommended", true) == true)
        assertEquals("通知权限已允许。", payload.getString("message"))
    }

    @Test
    @Config(sdk = [35])
    fun android13AndLaterReportsInitialDeniedPermissionAsRequestableWhenActivityExists() {
        val context = RuntimeEnvironment.getApplication()
        val activity = org.robolectric.Robolectric.buildActivity(android.app.Activity::class.java).setup().get()

        val payload = AndroidNotificationPermissions.statusPayload(context, PermissionState.DENIED.toString(), activity, 0)

        assertFalse(AndroidNotificationPermissions.granted(context))
        assertFalse(payload.getBoolean("granted", true) == true)
        assertTrue(payload.getBoolean("can_request", false) == true)
        assertFalse(payload.getBoolean("open_settings_recommended", true) == true)
        assertFalse(payload.getBoolean("request_already_attempted", true) == true)
    }

    @Test
    @Config(sdk = [35])
    fun android13AndLaterReportsFirstDeniedRequestAsStillRequestable() {
        val context = RuntimeEnvironment.getApplication()
        val activity = org.robolectric.Robolectric.buildActivity(android.app.Activity::class.java).setup().get()

        val payload = AndroidNotificationPermissions.statusPayload(context, PermissionState.DENIED.toString(), activity, 1)

        assertFalse(payload.getBoolean("granted", true) == true)
        assertTrue(payload.getBoolean("can_request", false) == true)
        assertFalse(payload.getBoolean("open_settings_recommended", true) == true)
        assertTrue(payload.getBoolean("request_already_attempted", false) == true)
    }

    @Test
    @Config(sdk = [35])
    fun android13AndLaterRecommendsSettingsAfterRepeatedDeniedRequestCanNoLongerPrompt() {
        val context = RuntimeEnvironment.getApplication()

        val payload = AndroidNotificationPermissions.statusPayload(context, PermissionState.DENIED.toString(), null, 2)

        assertFalse(payload.getBoolean("granted", true) == true)
        assertFalse(payload.getBoolean("can_request", true) == true)
        assertTrue(payload.getBoolean("open_settings_recommended", false) == true)
        assertTrue(payload.getBoolean("request_already_attempted", false) == true)
        assertEquals("系统可能已不再显示通知授权弹窗，请到应用通知设置中手动允许。", payload.getString("message"))
    }

    @Test
    @Config(sdk = [35])
    fun grantedRequestClearsDeniedRequestCount() {
        val context = RuntimeEnvironment.getApplication()
        AndroidNotificationPermissions.clearRequestHistory(context)

        AndroidNotificationPermissions.recordRequestResult(context, false)
        AndroidNotificationPermissions.recordRequestResult(context, false)
        assertTrue(AndroidNotificationPermissions.requestAlreadyAttempted(context))

        AndroidNotificationPermissions.recordRequestResult(context, true)

        assertFalse(AndroidNotificationPermissions.requestAlreadyAttempted(context))
        assertEquals(0, AndroidNotificationPermissions.deniedRequestCount(context))
    }

    @Test
    @Config(sdk = [35])
    fun settingsIntentOpensAppNotificationSettingsBeforeFallbackDetails() {
        val context = RuntimeEnvironment.getApplication()

        val intents = AndroidNotificationPermissions.settingsIntents(context)

        assertEquals(Settings.ACTION_APP_NOTIFICATION_SETTINGS, intents[0].action)
        assertEquals(context.packageName, intents[0].getStringExtra(Settings.EXTRA_APP_PACKAGE))
        assertEquals(Settings.ACTION_APPLICATION_DETAILS_SETTINGS, intents[1].action)
        assertEquals("package:${context.packageName}", intents[1].data.toString())
    }
}
