package io.github.axuitomo.cfstgui

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.RuntimeEnvironment
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

    @Test
    fun recordFromEventShowsEveryDnsProvider() {
        val context = RuntimeEnvironment.getApplication()
        val recorded = AndroidUploadNotificationState.recordFromEvent(
            context,
            """
            {
              "event":"upload.notification",
              "payload":{
                "source":"scheduled_probe",
                "status":"partial",
                "providers":[
                  {"provider":"cloudflare","label":"Cloudflare (edge.example.com)","status":"completed","upload_count":2},
                  {"provider":"dnshe","label":"DNSHE (api.example.com)","status":"failed","upload_count":0},
                  {"provider":"github","label":"GitHub","status":"skipped","upload_count":0}
                ]
              }
            }
            """.trimIndent(),
        )

        assertTrue(recorded)
        assertEquals(
            "最近上传：定时任务自动上传 部分完成；Cloudflare (edge.exampl… 完成 2条，DNSHE (api.example.com) 失败 0条，GitHub 跳过 0条",
            AndroidUploadNotificationState.notificationText(context),
        )
    }

    @Test
    fun providerLabelsRespectNotificationLengthBoundary() {
        val context = RuntimeEnvironment.getApplication()
        for (length in listOf(23, 24, 25)) {
            val label = "A".repeat(length)
            val expected = if (length > 24) "A".repeat(23) + "…" else label
            AndroidUploadNotificationState.recordFromEvent(
                context,
                """
                {"event":"upload.notification","payload":{"status":"completed","providers":[
                  {"provider":"cloudflare","label":"$label","status":"completed","upload_count":1}
                ]}}
                """.trimIndent(),
            )
            assertTrue(AndroidUploadNotificationState.notificationText(context).endsWith("$expected 完成 1条"))
        }
    }
    // aborted（任务被终止/熔断）必须和 skipped（没有可上传结果）分开显示，
    // 否则常驻通知会把「链路没跑完」说成「跳过」。
    @Test
    fun abortedStatusIsDistinguishedFromSkipped() {
        val context = RuntimeEnvironment.getApplication()
        val recorded = AndroidUploadNotificationState.recordFromEvent(
            context,
            """
            {"event":"upload.notification","payload":{"source":"scheduled_probe","status":"aborted","providers":[
              {"provider":"cloudflare","label":"Cloudflare","status":"aborted","upload_count":0},
              {"provider":"github","label":"GitHub","status":"skipped","upload_count":0}
            ]}}
            """.trimIndent(),
        )

        assertTrue(recorded)
        assertEquals(
            "最近上传：定时任务自动上传 熔断/终止；Cloudflare 熔断/终止 0条，GitHub 跳过 0条",
            AndroidUploadNotificationState.notificationText(context),
        )
    }
}
