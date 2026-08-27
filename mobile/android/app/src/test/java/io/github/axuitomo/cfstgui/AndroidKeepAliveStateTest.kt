package io.github.axuitomo.cfstgui

import android.Manifest
import android.content.Context
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RuntimeEnvironment
import org.robolectric.RobolectricTestRunner
import org.robolectric.Shadows
import org.robolectric.annotation.Config

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [35])
class AndroidKeepAliveStateTest {
    @Before
    fun resetPreferences() {
        RuntimeEnvironment.getApplication()
            .getSharedPreferences("cfst_android_keep_alive", Context.MODE_PRIVATE)
            .edit()
            .clear()
            .commit()
    }

    @Test
    fun keepAliveIsUnsupportedOnAndroid15ButPreferenceIsPreserved() {
        val context = RuntimeEnvironment.getApplication()

        val payload = AndroidKeepAliveState.statusPayload(context)

        assertTrue(AndroidKeepAliveState.enabled(context))
        assertFalse(AndroidKeepAliveState.supported())
        assertFalse(payload.getBoolean("supported"))
        assertTrue(payload.getBoolean("enabled"))
        assertFalse(payload.getBoolean("running"))
        assertEquals(
            "Android 15 及以上由 WorkManager 和系统调度后台任务，不启动常驻 dataSync 保活服务。",
            payload.getString("message"),
        )
    }

    @Test
    fun disabledKeepAliveIsPersistedAndReportedStopped() {
        val context = RuntimeEnvironment.getApplication()

        val payload = AndroidKeepAliveState.setEnabled(context, false)

        assertFalse(AndroidKeepAliveState.enabled(context))
        assertFalse(payload.getBoolean("enabled"))
        assertFalse(payload.getBoolean("running"))
        assertEquals(
            "Android 15 及以上由 WorkManager 和系统调度后台任务，不启动常驻 dataSync 保活服务。",
            payload.getString("message"),
        )
    }

    @Test
    fun unsupportedKeepAliveDoesNotStartService() {
        val context = RuntimeEnvironment.getApplication()

        val started = AndroidKeepAliveState.startIfAllowed(context)
        val payload = AndroidKeepAliveState.statusPayload(context)

        assertFalse(started)
        assertTrue(payload.getBoolean("enabled"))
        assertFalse(payload.getBoolean("notification_permission_granted"))
        assertFalse(payload.getBoolean("running"))
        assertNull(Shadows.shadowOf(context).nextStartedService)
    }

    @Test
    @Config(sdk = [34])
    fun enabledKeepAliveWithNotificationPermissionStartsService() {
        val context = RuntimeEnvironment.getApplication()
        Shadows.shadowOf(context).grantPermissions(Manifest.permission.POST_NOTIFICATIONS)

        val started = AndroidKeepAliveState.startIfAllowed(context)
        val intent = Shadows.shadowOf(context).nextStartedService

        assertTrue(AndroidKeepAliveState.supported())
        assertTrue(started)
        assertEquals(AndroidKeepAliveForegroundService::class.java.name, intent.component?.className)
    }
}
