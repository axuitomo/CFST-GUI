package io.github.axuitomo.cfstgui

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RuntimeEnvironment
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [35])
class AndroidUploadNotificationStateTest {
    @Before
    fun resetPreferences() {
        AndroidUploadNotificationState.clear(RuntimeEnvironment.getApplication())
    }

    @Test
    fun notificationTextDefaultsBeforeUpload() {
        val context = RuntimeEnvironment.getApplication()

        assertEquals("常驻通知栏运行中，定时任务和后台测速更稳定。", AndroidUploadNotificationState.notificationText(context))
    }

    @Test
    fun recordFromEventPersistsUploadSummary() {
        val context = RuntimeEnvironment.getApplication()
        val recorded = AndroidUploadNotificationState.recordFromEvent(
            context,
            """
            {
              "event":"upload.notification",
              "payload":{
                "source":"scheduled_probe",
                "status":"partial",
                "cloudflare_status":"completed",
                "cloudflare_upload_count":3,
                "github_status":"failed",
                "github_upload_count":0
              }
            }
            """.trimIndent(),
        )

        assertTrue(recorded)
        assertEquals("最近上传：定时任务自动上传 部分完成；CF 完成 3条，GitHub 失败 0条", AndroidUploadNotificationState.notificationText(context))
    }
}
