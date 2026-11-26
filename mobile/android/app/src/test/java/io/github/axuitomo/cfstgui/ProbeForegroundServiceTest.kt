package io.github.axuitomo.cfstgui

import android.app.Notification
import android.app.NotificationManager
import org.json.JSONArray
import org.json.JSONObject
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.Robolectric
import org.robolectric.RobolectricTestRunner
import org.robolectric.RuntimeEnvironment
import org.robolectric.Shadows.shadowOf
import org.robolectric.annotation.Config

// 常驻通知的阶段汇总行为:立即显示、作为第二行黏在后续进度/测速通知上、
// 被下一段汇总替换、换任务时清空。事件走 CfstRuntime,与真机同一条分发路径。
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [35])
class ProbeForegroundServiceTest {
    @Test
    fun stageSummaryShowsFunnelWithStageTitle() {
        val controller = Robolectric.buildService(ProbeForegroundService::class.java).create()
        try {
            val firstStagePayload = stageSummaryPayload("stage1_tcp", 4, 6, reasonEntry("TCP 无响应", 6), 12_000L)
            emit("task-1", "probe.stage_summary", firstStagePayload)

            assertEquals("TCP测延迟汇总", lastTitle())
            assertEquals("输入 10 → 通过 4 / 淘汰 6（TCP 无响应 6），耗时 12 秒。", lastText())
        } finally {
            controller.destroy()
        }
    }

    @Test
    fun stageSummarySticksAsSecondLineUntilTheNextSummary() {
        val controller = Robolectric.buildService(ProbeForegroundService::class.java).create()
        try {
            val firstStagePayload = stageSummaryPayload("stage1_tcp", 4, 6, reasonEntry("TCP 无响应", 6), 12_000L)
            emit("task-1", "probe.stage_summary", firstStagePayload)
            val firstSummary = lastText()

            // 阶段汇总的下一条事件是下一阶段的进度(毫秒级),不黏住就等于看不到汇总。
            emit("task-1", "probe.progress", progressPayload("stage2_trace"))
            assertEquals("已处理 5/10，通过 3，失败 2。\n上一阶段：$firstSummary", lastText())

            // 展开态(BigTextStyle)必须能看到同一行汇总,否则折叠态只剩实时进度。
            assertEquals(lastText(), lastBigText())

            emit("task-1", "probe.speed", speedPayload())
            assertEquals("1.1.1.1 当前 12.35 MB/s，均速 8.90 MB/s。\n上一阶段：$firstSummary", lastText())

            // 下一段汇总替换上一段,之后黏的是新汇总。
            val nextStagePayload = stageSummaryPayload("stage2_trace", 2, 2, reasonEntry("状态码不匹配", 2), 3_000L)
            emit("task-1", "probe.stage_summary", nextStagePayload)
            val secondSummary = lastText()
            assertEquals("追踪探测汇总", lastTitle())
            assertEquals("输入 4 → 通过 2 / 淘汰 2（状态码不匹配 2），耗时 3 秒。", secondSummary)

            emit("task-1", "probe.progress", progressPayload("stage3_get"))
            assertEquals("已处理 5/10，通过 3，失败 2。\n上一阶段：$secondSummary", lastText())

            // 终态通知只报结果,不再拖第二行。
            emit("task-1", "probe.completed", JSONObject().put("result_count", 7))
            assertEquals("任务完成，可用结果 7 条。", lastText())
        } finally {
            controller.destroy()
        }
    }

    @Test
    fun newTaskDropsThePreviousStageSummary() {
        val controller = Robolectric.buildService(ProbeForegroundService::class.java).create()
        try {
            val firstStagePayload = stageSummaryPayload("stage1_tcp", 4, 6, reasonEntry("TCP 无响应", 6), 12_000L)
            emit("task-1", "probe.stage_summary", firstStagePayload)

            emit("task-2", "probe.progress", progressPayload("stage1_tcp"))
            assertEquals("已处理 5/10，通过 3，失败 2。", lastText())
            assertFalse(lastText().contains("上一阶段"))

            // 新任务自己的汇总照常显示并黏住。
            val taskTwoStagePayload = stageSummaryPayload("stage1_tcp", 1, 7, reasonEntry("TCP 无响应", 7), 9_000L)
            emit("task-2", "probe.stage_summary", taskTwoStagePayload)
            emit("task-2", "probe.speed", speedPayload())
            assertEquals(
                "1.1.1.1 当前 12.35 MB/s，均速 8.90 MB/s。\n上一阶段：输入 8 → 通过 1 / 淘汰 7（TCP 无响应 7），耗时 9 秒。",
                lastText(),
            )
        } finally {
            controller.destroy()
        }
    }

    private fun emit(taskId: String, event: String, payload: JSONObject) {
        CfstRuntime.emitSyntheticProbeEvent(taskId, event, payload)
    }

    // 夹具里 counts 的 input/total 恒等于 passed + failed，失败数即差额。
    private fun stageSummaryPayload(
        stage: String,
        passed: Int,
        failed: Int,
        reason: JSONObject,
        durationMS: Long,
    ): JSONObject {
        val total = passed + failed
        val counts = JSONObject().put("input", total).put("total", total).put("passed", passed).put("failed", failed)
        return JSONObject()
            .put("stage", stage)
            .put("duration_ms", durationMS)
            .put("counts", counts)
            .put("reasons", JSONArray().put(reason))
    }

    private fun reasonEntry(label: String, count: Int): JSONObject {
        return JSONObject().put("label", label).put("count", count)
    }

    private fun progressPayload(stage: String): JSONObject {
        return JSONObject()
            .put("stage", stage)
            .put("processed", 5)
            .put("total", 10)
            .put("passed", 3)
            .put("failed", 2)
    }

    private fun speedPayload(): JSONObject {
        return JSONObject()
            .put("ip", "1.1.1.1")
            .put("current_speed_mb_s", 12.345)
            .put("average_speed_mb_s", 8.9)
            .put("current_ready", true)
            .put("average_ready", true)
    }

    private fun lastTitle(): String {
        return lastNotification().extras.getCharSequence(Notification.EXTRA_TITLE)?.toString().orEmpty()
    }

    private fun lastBigText(): String {
        return lastNotification().extras.getCharSequence(Notification.EXTRA_BIG_TEXT)?.toString().orEmpty()
    }

    private fun lastText(): String {
        return lastNotification().extras.getCharSequence(Notification.EXTRA_TEXT)?.toString().orEmpty()
    }

    private fun lastNotification(): Notification {
        val manager = RuntimeEnvironment.getApplication().getSystemService(NotificationManager::class.java)
            ?: error("Robolectric 没有提供 NotificationManager")
        return shadowOf(manager).allNotifications.lastOrNull()
            ?: error("前台服务没有更新通知")
    }
}
