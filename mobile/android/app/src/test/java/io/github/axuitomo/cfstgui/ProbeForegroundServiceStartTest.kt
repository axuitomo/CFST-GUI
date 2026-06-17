package io.github.axuitomo.cfstgui

import android.app.Service
import java.lang.reflect.Method
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotEquals
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.Robolectric
import org.robolectric.RuntimeEnvironment
import org.robolectric.Shadows
import org.robolectric.annotation.Config
import org.robolectric.shadows.ShadowService

@RunWith(org.robolectric.RobolectricTestRunner::class)
@Config(sdk = [35])
class ProbeForegroundServiceStartTest {
    @After
    fun tearDown() {
        ProbeForegroundService.clearQueuedStart()
    }

    @Test
    fun duplicateManualStartDoesNotStopExistingForegroundService() {
        val context = RuntimeEnvironment.getApplication()
        ProbeForegroundService.clearQueuedStart()
        setProbeStartState(queued = true, claimed = true)

        val service = Robolectric.buildService(ProbeForegroundService::class.java).create().get()
        val shadowService = Shadows.shadowOf(service) as ShadowService
        val duplicate = ProbeForegroundService.startIntent(context, """{"task_id":"second-task"}""", null)

        assertEquals(Service.START_NOT_STICKY, service.onStartCommand(duplicate, 0, 2))
        assertNotEquals(2, shadowService.stopSelfId)
        assertFalse(shadowService.isForegroundStopped)
    }

    @Test
    fun duplicateScheduledStartDoesNotStopExistingForegroundService() {
        val context = RuntimeEnvironment.getApplication()
        ProbeForegroundService.clearQueuedStart()
        setProbeStartState(queued = true, claimed = true)
        val previousRefreshScheduledWork = ProbeForegroundService.refreshScheduledWork
        ProbeForegroundService.refreshScheduledWork = {}

        try {
            val service = Robolectric.buildService(ProbeForegroundService::class.java).create().get()
            val shadowService = Shadows.shadowOf(service) as ShadowService
            val duplicate = ProbeForegroundService.startScheduledIntent(context)

            assertEquals(Service.START_NOT_STICKY, service.onStartCommand(duplicate, 0, 3))
            assertNotEquals(3, shadowService.stopSelfId)
            assertFalse(shadowService.isForegroundStopped)
        } finally {
            ProbeForegroundService.refreshScheduledWork = previousRefreshScheduledWork
        }
    }

    @Test
    fun foregroundRunStopsLatestQueuedStartId() {
        val context = RuntimeEnvironment.getApplication()
        ProbeForegroundService.clearQueuedStart()
        setProbeStartState(queued = true, claimed = true)

        val service = Robolectric.buildService(ProbeForegroundService::class.java).create().get()
        val shadowService = Shadows.shadowOf(service) as ShadowService
        val duplicate = ProbeForegroundService.startIntent(context, """{"task_id":"second-task"}""", null)

        assertEquals(Service.START_NOT_STICKY, service.onStartCommand(duplicate, 0, 4))
        callFinishForegroundRun(service)

        assertEquals(4, shadowService.stopSelfId)
        assertTrue(shadowService.isForegroundStopped)
    }

    private fun setProbeStartState(queued: Boolean, claimed: Boolean) {
        setProbeForegroundServiceBoolean("startQueued", queued)
        setProbeForegroundServiceBoolean("startClaimed", claimed)
    }

    private fun setProbeForegroundServiceBoolean(name: String, value: Boolean) {
        val field = ProbeForegroundService::class.java.getDeclaredField(name)
        field.isAccessible = true
        field.setBoolean(null, value)
    }

    private fun callFinishForegroundRun(service: ProbeForegroundService) {
        val method: Method = ProbeForegroundService::class.java.getDeclaredMethod("finishForegroundRun")
        method.isAccessible = true
        method.invoke(service)
    }
}
