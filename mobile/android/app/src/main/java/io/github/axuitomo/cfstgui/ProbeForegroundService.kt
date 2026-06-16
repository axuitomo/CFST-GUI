package io.github.axuitomo.cfstgui

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.app.Service
import android.content.Context
import android.content.Intent
import android.content.pm.ServiceInfo
import android.os.Build
import android.os.IBinder
import android.util.Log
import androidx.core.app.NotificationCompat
import java.util.Locale
import org.json.JSONArray
import org.json.JSONObject

class ProbeForegroundService : Service() {
    private val notificationListener = CfstRuntime.ProbeEventListener { eventJSON ->
        handleProbeEvent(eventJSON)
    }

    @Volatile
    private var currentTaskId = ""

    override fun onCreate() {
        super.onCreate()
        ensureNotificationChannel()
        CfstRuntime.registerAuxiliaryListener(notificationListener)
    }

    override fun onDestroy() {
        foregroundRunning = false
        CfstRuntime.unregisterAuxiliaryListener(notificationListener)
        super.onDestroy()
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        val action = intent?.action.orEmpty()
        if (action != ACTION_START && action != ACTION_START_SCHEDULED) {
            clearQueuedStart()
            stopSelf(startId)
            return START_NOT_STICKY
        }
        currentTaskId = if (action == ACTION_START) {
            extractTaskId(intent?.getStringExtra(EXTRA_PAYLOAD))
        } else {
            ""
        }
        try {
            startForegroundCompat()
            foregroundRunning = true
        } catch (error: SecurityException) {
            Log.e(TAG, "Failed to promote probe service to foreground", error)
            foregroundRunning = false
            clearQueuedStart()
            val payload = JSONObject()
            try {
                payload.put("message", "Android 前台服务启动失败，请检查应用通知/前台服务权限后重试。")
                payload.put("recoverable", false)
            } catch (_: Exception) {
                // Ignore JSON fallback failure and still stop the service safely.
            }
            CfstRuntime.emitSyntheticProbeEvent(currentTaskId, "probe.failed", payload)
            if (action == ACTION_START_SCHEDULED) {
                try {
                    SchedulerWorker.scheduleFromStatus(applicationContext, CfstRuntime.service().refreshScheduler("{}"))
                } catch (_: Exception) {
                    // Scheduler can be rearmed on next app start or config save.
                }
            }
            stopSelf(startId)
            return START_NOT_STICKY
        }
        if (action == ACTION_START) {
            val payload = intent?.getStringExtra(EXTRA_PAYLOAD)
            val exportURI = intent?.getStringExtra(EXTRA_EXPORT_URI)
            val manualStartAlreadyQueuedOrClaimed = synchronized(startLock) {
                if (startQueued) {
                    if (startClaimed) {
                        true
                    } else {
                        startClaimed = true
                        false
                    }
                } else {
                    startQueued = true
                    startClaimed = true
                    false
                }
            }
            if (manualStartAlreadyQueuedOrClaimed || CfstRuntime.hasRunningOrPausedTask()) {
                Log.w(TAG, "Ignored duplicated background task start request because another task is already queued or active.")
                if (!manualStartAlreadyQueuedOrClaimed) {
                    clearQueuedStart()
                }
                stopForeground(STOP_FOREGROUND_REMOVE)
                stopSelf(startId)
                return START_NOT_STICKY
            }
            CfstRuntime.executor().execute {
                val taskIdForFailure = currentTaskId
                try {
                    var response = CfstRuntime.service().runProbe(payload ?: "{}")
                    if (!exportURI.isNullOrBlank()) {
                        response = CfstPlugin.copyProbeExportToURIStatic(applicationContext, response, exportURI)
                        recordAndroidExportResult(currentTaskId, response, exportURI)
                    }
                    if (response.isNullOrBlank()) {
                        Log.w(TAG, "Foreground task finished without command response.")
                    }
                } catch (error: Exception) {
                    Log.e(TAG, "Foreground task execution failed", error)
                    emitForegroundTaskFailure(taskIdForFailure, error)
                } finally {
                    clearQueuedStart()
                    stopForeground(STOP_FOREGROUND_REMOVE)
                    stopSelf(startId)
                }
            }
        } else if (action == ACTION_START_SCHEDULED) {
            val scheduledStartAlreadyQueued = synchronized(startLock) {
                if (startQueued) {
                    true
                } else {
                    startQueued = true
                    startClaimed = true
                    false
                }
            }
            if (scheduledStartAlreadyQueued) {
                Log.w(TAG, "Ignored scheduled background task because another task start is already queued or active.")
                try {
                    SchedulerWorker.scheduleFromStatus(applicationContext, CfstRuntime.service().refreshScheduler("{}"))
                } catch (_: Exception) {
                    // Scheduler can be rearmed on next app start or config save.
                }
                stopForeground(STOP_FOREGROUND_REMOVE)
                stopSelf(startId)
                return START_NOT_STICKY
            }
            CfstRuntime.executor().execute {
                val taskIdForFailure = currentTaskId
                try {
                    val response = CfstRuntime.service().runScheduledProbe("{}")
                    SchedulerWorker.scheduleFromStatus(applicationContext, response)
                    if (response.isNullOrBlank()) {
                        Log.w(TAG, "Scheduled foreground task finished without command response.")
                    }
                } catch (error: Exception) {
                    Log.e(TAG, "Scheduled foreground task execution failed", error)
                    emitForegroundTaskFailure(taskIdForFailure, error)
                    try {
                        SchedulerWorker.scheduleFromStatus(applicationContext, CfstRuntime.service().refreshScheduler("{}"))
                    } catch (_: Exception) {
                        // Scheduler can be rearmed on next app start or config save.
                    }
                } finally {
                    clearQueuedStart()
                    stopForeground(STOP_FOREGROUND_REMOVE)
                    stopSelf(startId)
                }
            }
        }
        return START_NOT_STICKY
    }

    private fun recordAndroidExportResult(fallbackTaskId: String?, responseJSON: String?, exportURI: String?) {
        try {
            val command = JSONObject(responseJSON ?: "{}")
            val data = command.optJSONObject("data") ?: return
            val status = firstNonEmpty(data.optString("android_export_status", ""), data.optString("androidExportStatus", ""))
            if (status.isEmpty()) {
                return
            }
            val targetURI = firstNonEmpty(data.optString("android_export_uri", ""), data.optString("androidExportUri", ""), exportURI)
            val sourcePath = firstNonEmpty(data.optString("android_export_source_path", ""), data.optString("androidExportSourcePath", ""))
            var message = firstNonEmpty(data.optString("android_export_error", ""), data.optString("androidExportError", ""))
            val written = status == "written"
            if (message.isEmpty()) {
                message = if (written) "Android 系统导出文件已写入。" else "Android 系统导出文件失败。"
            }

            val payload = JSONObject()
            payload.put("ok", written)
            payload.put("message", message)
            payload.put("source_path", sourcePath)
            payload.put("status", status)
            payload.put("target_uri", targetURI)
            payload.put("task_id", firstNonEmpty(command.optString("task_id", ""), fallbackTaskId))
            payload.put("written", exportedCountFromProbeData(data))
            CfstRuntime.service().recordAndroidExportResult(payload.toString())
        } catch (error: Exception) {
            Log.e(TAG, "Failed to record Android export result", error)
            emitAndroidExportFailure(fallbackTaskId, exportURI, error.message)
        }
    }

    private fun exportedCountFromProbeData(data: JSONObject): Int {
        val results: JSONArray? = data.optJSONArray("results")
        if (results != null) {
            return results.length()
        }
        val summary = data.optJSONObject("summary")
        if (summary != null) {
            return maxOf(summary.optInt("passed", 0), summary.optInt("total", 0))
        }
        return data.optInt("exported", 0)
    }

    private fun emitAndroidExportFailure(taskId: String?, exportURI: String?, message: String?) {
        try {
            val payload = JSONObject()
            payload.put("message", if (message.isNullOrBlank()) "Android 系统导出状态记录失败。" else message)
            payload.put("recoverable", true)
            payload.put("stage", "export")
            payload.put("target_path", exportURI?.trim().orEmpty())
            CfstRuntime.emitSyntheticProbeEvent(taskId, "probe.export_failed", payload)
        } catch (_: Exception) {
            // Synthetic event fallback should never crash foreground cleanup.
        }
    }

    private fun emitForegroundTaskFailure(taskId: String?, error: Exception?) {
        try {
            val payload = JSONObject()
            var message = error?.message.orEmpty()
            if (message.trim().isEmpty()) {
                message = "后台任务执行失败。"
            }
            payload.put("message", message)
            payload.put("recoverable", false)
            CfstRuntime.emitSyntheticProbeEvent(taskId, "probe.failed", payload)
        } catch (_: Exception) {
            // Synthetic event fallback should never crash foreground cleanup.
        }
    }

    private fun firstNonEmpty(vararg values: String?): String {
        for (value in values) {
            if (!value.isNullOrBlank()) {
                return value.trim()
            }
        }
        return ""
    }

    override fun onBind(intent: Intent?): IBinder? = null

    private fun ensureNotificationChannel() {
        val manager = getSystemService(Context.NOTIFICATION_SERVICE) as? NotificationManager
        if (manager == null || manager.getNotificationChannel(CHANNEL_ID) != null) {
            return
        }
        val channel = NotificationChannel(
            CHANNEL_ID,
            "CFST 后台任务",
            NotificationManager.IMPORTANCE_LOW,
        )
        channel.description = "保持 CFST Android 长任务在前台服务中执行。"
        manager.createNotificationChannel(channel)
    }

    private fun buildNotification(title: String, content: String): Notification {
        val openAppIntent = openAppIntent()
        return NotificationCompat.Builder(this, CHANNEL_ID)
            .setContentTitle(title)
            .setContentText(content)
            .setContentIntent(openAppIntent)
            .addAction(android.R.drawable.ic_menu_view, "打开", openAppIntent)
            .setCategory(NotificationCompat.CATEGORY_SERVICE)
            .setOnlyAlertOnce(true)
            .setOngoing(true)
            .setSmallIcon(android.R.drawable.stat_sys_download_done)
            .build()
    }

    private fun openAppIntent(): PendingIntent {
        val intent = Intent(this, MainActivity::class.java).apply {
            action = "io.github.axuitomo.cfstgui.action.OPEN_FROM_NOTIFICATION"
            addFlags(Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_ACTIVITY_CLEAR_TOP or Intent.FLAG_ACTIVITY_SINGLE_TOP)
        }
        return PendingIntent.getActivity(
            this,
            NOTIFICATION_ID,
            intent,
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE,
        )
    }

    private fun startForegroundCompat() {
        val notification = buildNotification("任务运行中", "CFST 正在执行后台任务。")
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
            startForeground(NOTIFICATION_ID, notification, ServiceInfo.FOREGROUND_SERVICE_TYPE_DATA_SYNC)
            return
        }
        startForeground(NOTIFICATION_ID, notification)
    }

    private fun updateNotification(title: String, content: String) {
        val manager = getSystemService(Context.NOTIFICATION_SERVICE) as? NotificationManager ?: return
        manager.notify(NOTIFICATION_ID, buildNotification(title, content))
    }

    private fun handleProbeEvent(eventJSON: String) {
        try {
            val event = JSONObject(eventJSON)
            val taskId = event.optString("task_id", "").trim()
            if (taskId.isNotEmpty()) {
                currentTaskId = taskId
            }
            val payload = event.optJSONObject("payload")
            when (event.optString("event", "")) {
                "probe.preprocessed" -> updateNotification("正在准备任务", formatPreprocessContent(payload))
                "probe.progress" -> updateNotification(stageTitle(payload?.optString("stage", "").orEmpty()), formatProgressContent(payload))
                "probe.speed" -> updateNotification("文件测速中", formatSpeedContent(payload))
                "probe.partial_export" -> updateNotification("结果已落盘", formatExportContent(payload))
                "probe.cooling" -> updateNotification(
                    "任务冷却中",
                    payload?.optString("reason", "任务正在冷却。") ?: "任务正在等待恢复或安全停止。",
                )
                "probe.completed" -> updateNotification("任务已完成", formatCompletedContent(payload))
                "probe.failed" -> updateNotification("任务失败", payload?.optString("message", "后台探测任务失败。") ?: "后台探测任务失败。")
            }
        } catch (error: Exception) {
            Log.e(TAG, "Failed to update notification from probe event", error)
        }
    }

    private fun formatPreprocessContent(payload: JSONObject?): String {
        if (payload == null) {
            return "正在整理输入源与候选 IP。"
        }
        val accepted = payload.optInt("accepted", 0)
        val filtered = payload.optInt("filtered", 0)
        val invalid = payload.optInt("invalid", 0)
        return String.format(Locale.ROOT, "候选 %d，通过 %d，过滤 %d。", accepted + filtered + invalid, accepted, filtered + invalid)
    }

    private fun formatProgressContent(payload: JSONObject?): String {
        if (payload == null) {
            return "探测任务正在执行。"
        }
        val processed = payload.optInt("processed", 0)
        val total = payload.optInt("total", 0)
        val passed = payload.optInt("passed", 0)
        val failed = payload.optInt("failed", 0)
        if (total > 0) {
            return String.format(Locale.ROOT, "已处理 %d/%d，通过 %d，失败 %d。", processed, total, passed, failed)
        }
        return String.format(Locale.ROOT, "已处理 %d，通过 %d，失败 %d。", processed, passed, failed)
    }

    private fun formatSpeedContent(payload: JSONObject?): String {
        if (payload == null) {
            return "正在采集测速样本。"
        }
        val ip = payload.optString("ip", "当前 IP")
        val current = payload.optDouble("current_speed_mb_s", 0.0)
        val average = payload.optDouble("average_speed_mb_s", 0.0)
        if (average > 0) {
            return String.format(Locale.ROOT, "%s 当前 %.2f MB/s，均速 %.2f MB/s。", ip, current, average)
        }
        if (current > 0) {
            return String.format(Locale.ROOT, "%s 当前 %.2f MB/s。", ip, current)
        }
        return "$ip 正在测速中。"
    }

    private fun formatExportContent(payload: JSONObject?): String {
        if (payload == null) {
            return "已有部分结果写入磁盘。"
        }
        val written = payload.optInt("written", 0)
        val target = payload.optString("target_path", "")
        return if (target.isEmpty()) {
            String.format(Locale.ROOT, "已写出 %d 条结果。", written)
        } else {
            String.format(Locale.ROOT, "已写出 %d 条结果到导出文件。", written)
        }
    }

    private fun formatCompletedContent(payload: JSONObject?): String {
        if (payload == null) {
            return "后台探测任务已完成。"
        }
        val results = payload.optInt("result_count", payload.optInt("passed", payload.optInt("exported", 0)))
        return if (results > 0) {
            String.format(Locale.ROOT, "任务完成，可用结果 %d 条。", results)
        } else {
            "任务完成，但当前没有可用结果。"
        }
    }

    private fun stageTitle(stage: String?): String {
        return when (stage?.trim().orEmpty()) {
            "stage0_pool" -> "输入预处理"
            "stage1_tcp" -> "TCP测延迟"
            "stage2_trace", "stage2_head" -> "追踪探测"
            "stage3_get" -> "文件测速"
            else -> "任务运行中"
        }
    }

    private fun extractTaskId(payloadJSON: String?): String {
        if (payloadJSON.isNullOrBlank()) {
            return ""
        }
        return try {
            val payload = JSONObject(payloadJSON)
            firstNonEmpty(
                payload.optString("task_id", ""),
                payload.optString("taskId", ""),
            )
        } catch (_: Exception) {
            ""
        }
    }

    companion object {
        private const val ACTION_START = "io.github.axuitomo.cfstgui.action.START_PROBE"
        private const val ACTION_START_SCHEDULED = "io.github.axuitomo.cfstgui.action.START_SCHEDULED_PROBE"
        private const val CHANNEL_ID = "cfst_probe"
        private const val NOTIFICATION_ID = 7010
        private const val EXTRA_PAYLOAD = "payload"
        private const val EXTRA_EXPORT_URI = "export_uri"
        private const val TAG = "ProbeFgService"
        private val startLock = Any()

        @Volatile
        private var startQueued = false

        @Volatile
        private var startClaimed = false

        @Volatile
        private var foregroundRunning = false

        @JvmStatic
        fun markStartQueuedIfIdle(): Boolean = synchronized(startLock) {
            if (startQueued || CfstRuntime.hasRunningOrPausedTask()) {
                false
            } else {
                startQueued = true
                startClaimed = false
                true
            }
        }

        @JvmStatic
        fun clearQueuedStart() {
            synchronized(startLock) {
                startQueued = false
                startClaimed = false
            }
        }

        @JvmStatic
        fun isForegroundRunning(): Boolean = foregroundRunning

        @JvmStatic
        fun startIntent(context: Context, payload: String?, exportURI: String?): Intent =
            Intent(context, ProbeForegroundService::class.java).apply {
                action = ACTION_START
                putExtra(EXTRA_PAYLOAD, payload)
                putExtra(EXTRA_EXPORT_URI, exportURI ?: "")
            }

        @JvmStatic
        fun startScheduledIntent(context: Context): Intent =
            Intent(context, ProbeForegroundService::class.java).apply {
                action = ACTION_START_SCHEDULED
            }
    }
}
