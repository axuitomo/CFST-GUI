package io.github.axuitomo.cfstgui

import org.json.JSONArray
import org.json.JSONObject
import java.util.Locale

// `probe.stage_summary` 的通知文案:漏斗计数 + 淘汰原因 + 阶段耗时。
// 口径与 WebUI 的「阶段汇总」一致,两端看到的是同一组数字。
object AndroidStageSummary {
    private const val MILLIS_PER_SECOND = 1000L
    private const val SECONDS_PER_MINUTE = 60L
    private const val MINUTES_PER_HOUR = 60L

    // 后端已按条数降序下发,这里只截断显示:一行最多 4 条,其余只报数量。
    private const val MAX_REASONS = 4

    @JvmStatic
    fun formatContent(payload: JSONObject?): String {
        if (payload == null) {
            return "阶段已结束。"
        }
        val counts = payload.optJSONObject("counts")
        val total = counts.positiveInt("total")
        val input = counts.positiveInt("input").takeIf { it > 0 } ?: total
        val passed = counts.positiveInt("passed")
        val failed = counts.positiveInt("failed").takeIf { it > 0 } ?: (total - passed).coerceAtLeast(0)
        val reasonText = reasonText(payload.optJSONArray("reasons"))
        val reasonSuffix = if (reasonText.isEmpty()) "" else "（" + reasonText + "）"
        val durationMS = payload.optLong("duration_ms", payload.optLong("durationMS", 0L))
        val durationSuffix = if (durationMS > 0) "，耗时 " + duration(durationMS) else ""

        return "输入 $input → 通过 $passed / 淘汰 $failed$reasonSuffix$durationSuffix。"
    }

    // 阶段汇总是「刚结束那一段」的结论:挂在后续进度通知的第二行,
    // 折叠态显示第一行(实时进度),展开态(BigTextStyle)能看到汇总。
    @JvmStatic
    fun formatSuffix(summary: String): String = if (summary.isBlank()) "" else "\n上一阶段：" + summary

    private fun JSONObject?.positiveInt(key: String): Int = (this?.optInt(key, 0) ?: 0).coerceAtLeast(0)

    private fun reasonText(reasons: JSONArray?): String {
        val entries = ArrayList<String>()
        if (reasons != null) {
            for (index in 0 until reasons.length()) {
                val item = reasons.optJSONObject(index)
                val label = AndroidPayloads.firstNonEmpty(
                    item?.optString("label", "").orEmpty(),
                    item?.optString("reason", "").orEmpty(),
                ).trim()
                val count = item?.optInt("count", 0) ?: 0
                if (label.isNotEmpty() && count > 0) {
                    entries.add(String.format(Locale.ROOT, "%s %d", label, count))
                }
            }
        }
        val shown = entries.take(MAX_REASONS).joinToString("、")
        return when {
            entries.isEmpty() -> ""
            entries.size > MAX_REASONS -> "$shown 等 ${entries.size} 项"
            else -> shown
        }
    }

    private fun duration(durationMS: Long): String {
        val seconds = durationMS.coerceAtLeast(0L) / MILLIS_PER_SECOND
        val minutes = seconds / SECONDS_PER_MINUTE
        return when {
            seconds < SECONDS_PER_MINUTE -> String.format(Locale.ROOT, "%d 秒", seconds)
            minutes < MINUTES_PER_HOUR -> String.format(Locale.ROOT, "%d 分 %d 秒", minutes, seconds % SECONDS_PER_MINUTE)
            else -> String.format(Locale.ROOT, "%d 时 %d 分", minutes / MINUTES_PER_HOUR, minutes % MINUTES_PER_HOUR)
        }
    }
}
