package io.github.axuitomo.cfstgui

import org.json.JSONArray
import org.json.JSONObject
import org.junit.Assert.assertEquals
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [35])
class AndroidStageSummaryTest {
    @Test
    fun formatsFunnelReasonsAndDuration() {
        val payload = JSONObject()
            .put("stage", "stage1_tcp")
            .put("duration_ms", 225_000L)
            .put("counts", JSONObject().put("input", 10).put("total", 10).put("passed", 4).put("failed", 6))
            .put(
                "reasons",
                JSONArray()
                    .put(JSONObject().put("reason", "tcp_no_response").put("label", "TCP 无响应").put("count", 5))
                    .put(JSONObject().put("reason", "tcp_loss_above_limit").put("label", "TCP 丢包超上限").put("count", 1)),
            )

        assertEquals(
            "输入 10 → 通过 4 / 淘汰 6（TCP 无响应 5、TCP 丢包超上限 1），耗时 3 分 45 秒。",
            AndroidStageSummary.formatContent(payload),
        )
    }

    @Test
    fun fallsBackWhenCountsOrPayloadAreMissing() {
        assertEquals("阶段已结束。", AndroidStageSummary.formatContent(null))

        // counts 只有 total/passed 时按差额推淘汰数,并保留超过 4 条时的原因总数。
        val reasons = JSONArray()
        for (index in 6 downTo 1) {
            reasons.put(JSONObject().put("label", "原因$index").put("count", index))
        }
        val payload = JSONObject()
            .put("counts", JSONObject().put("total", 12).put("passed", 2))
            .put("reasons", reasons)

        assertEquals(
            "输入 12 → 通过 2 / 淘汰 10（原因6 6、原因5 5、原因4 4、原因3 3 等 6 项）。",
            AndroidStageSummary.formatContent(payload),
        )
    }

    @Test
    fun suffixOnlyAppearsAfterASummary() {
        assertEquals("", AndroidStageSummary.formatSuffix(""))
        assertEquals("", AndroidStageSummary.formatSuffix("   "))
        assertEquals(
            "\n上一阶段：输入 10 → 通过 4 / 淘汰 6。",
            AndroidStageSummary.formatSuffix("输入 10 → 通过 4 / 淘汰 6。"),
        )
    }
}
