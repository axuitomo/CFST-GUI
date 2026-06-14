package io.github.axuitomo.cfstgui

import android.Manifest
import android.content.Context
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
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
    fun keepAliveDefaultsToEnabled() {
        val context = RuntimeEnvironment.getApplication()

        val payload = AndroidKeepAliveState.statusPayload(context)

        assertTrue(AndroidKeepAliveState.enabled(context))
        assertTrue(payload.getBoolean("supported"))
        assertTrue(payload.getBoolean("enabled"))
    }

    @Test
    fun disabledKeepAliveIsPersistedAndReportedStopped() {
        val context = RuntimeEnvironment.getApplication()

        val payload = AndroidKeepAliveState.setEnabled(context, false)

        assertFalse(AndroidKeepAliveState.enabled(context))
        assertFalse(payload.getBoolean("enabled"))
        assertFalse(payload.getBoolean("running"))
        assertEquals("通知栏保活已关闭；定时任务仍可运行，但更容易受系统后台策略影响。", payload.getString("message"))
    }

    @Test
    fun enabledKeepAliveWithoutNotificationPermissionDoesNotStartService() {
        val context = RuntimeEnvironment.getApplication()

        val started = AndroidKeepAliveState.startIfAllowed(context)
        val payload = AndroidKeepAliveState.statusPayload(context)

        assertFalse(started)
        assertTrue(payload.getBoolean("enabled"))
        assertFalse(payload.getBoolean("notification_permission_granted"))
        assertFalse(payload.getBoolean("running"))
        assertEquals("通知栏保活已启用，等待通知权限后自动显示常驻通知。", payload.getString("message"))
    }

    @Test
    fun enabledKeepAliveWithNotificationPermissionStartsService() {
        val context = RuntimeEnvironment.getApplication()
        Shadows.shadowOf(context).grantPermissions(Manifest.permission.POST_NOTIFICATIONS)

        val started = AndroidKeepAliveState.startIfAllowed(context)
        val intent = Shadows.shadowOf(context).nextStartedService

        assertTrue(started)
        assertEquals(AndroidKeepAliveForegroundService::class.java.name, intent.component?.className)
    }
}
