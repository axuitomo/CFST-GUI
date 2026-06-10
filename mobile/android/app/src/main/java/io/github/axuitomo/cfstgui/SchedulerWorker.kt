package io.github.axuitomo.cfstgui

import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import android.util.Log
import androidx.core.app.NotificationCompat
import androidx.work.ExistingWorkPolicy
import androidx.work.ForegroundInfo
import androidx.work.OneTimeWorkRequest
import androidx.work.WorkManager
import androidx.work.Worker
import androidx.work.WorkerParameters
import java.time.Duration
import java.time.Instant
import java.util.concurrent.TimeUnit
import org.json.JSONObject

class SchedulerWorker(context: Context, workerParams: WorkerParameters) : Worker(context, workerParams) {
    override fun doWork(): Result {
        val context = applicationContext
        return try {
            setForegroundAsync(createForegroundInfo())
            CfstRuntime.ensureInitialized(context, CfstPlugin.defaultRuntimeDirStatic(context).absolutePath)
            val serviceIntent = ProbeForegroundService.startScheduledIntent(context)
            context.startForegroundService(serviceIntent)
            Result.success()
        } catch (error: Exception) {
            Log.e(TAG, "Android scheduled probe failed", error)
            try {
                scheduleFromStatus(context, CfstRuntime.service().refreshScheduler("{}"))
            } catch (_: Exception) {
                // Keep WorkManager failure handling simple; scheduler can be rearmed on next config save/app launch.
            }
            Result.failure()
        }
    }

    private fun createForegroundInfo(): ForegroundInfo {
        ensureNotificationChannel()
        val openAppIntent = openAppIntent(applicationContext, NOTIFICATION_ID)
        val notification = NotificationCompat.Builder(applicationContext, CHANNEL_ID)
            .setSmallIcon(android.R.drawable.stat_notify_sync)
            .setContentTitle("CFST 定时任务")
            .setContentText("正在执行 Android 定时测速。")
            .setContentIntent(openAppIntent)
            .addAction(android.R.drawable.ic_menu_view, "打开", openAppIntent)
            .setCategory(NotificationCompat.CATEGORY_SERVICE)
            .setOnlyAlertOnce(true)
            .setOngoing(true)
            .setPriority(NotificationCompat.PRIORITY_LOW)
            .build()
        return ForegroundInfo(NOTIFICATION_ID, notification)
    }

    private fun openAppIntent(context: Context, requestCode: Int): PendingIntent {
        val intent = Intent(context, MainActivity::class.java).apply {
            action = "io.github.axuitomo.cfstgui.action.OPEN_FROM_NOTIFICATION"
            addFlags(Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_ACTIVITY_CLEAR_TOP or Intent.FLAG_ACTIVITY_SINGLE_TOP)
        }
        return PendingIntent.getActivity(
            context,
            requestCode,
            intent,
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE,
        )
    }

    private fun ensureNotificationChannel() {
        val manager = applicationContext.getSystemService(Context.NOTIFICATION_SERVICE) as? NotificationManager
        if (manager == null || manager.getNotificationChannel(CHANNEL_ID) != null) {
            return
        }
        manager.createNotificationChannel(NotificationChannel(CHANNEL_ID, "CFST 定时任务", NotificationManager.IMPORTANCE_LOW))
    }

    companion object {
        private const val CHANNEL_ID = "cfst_scheduler"
        private const val NOTIFICATION_ID = 7020
        private const val TAG = "SchedulerWorker"
        private const val UNIQUE_WORK_NAME = "cfst-android-scheduler"

        @JvmStatic
        fun refresh(context: Context): String {
            CfstRuntime.ensureInitialized(context, CfstPlugin.defaultRuntimeDirStatic(context).absolutePath)
            val response = CfstRuntime.service().refreshScheduler("{}")
            scheduleFromStatus(context, response)
            return response
        }

        @JvmStatic
        fun cancel(context: Context) {
            WorkManager.getInstance(context).cancelUniqueWork(UNIQUE_WORK_NAME)
        }

        @JvmStatic
        fun scheduleFromStatus(context: Context, response: String?) {
            try {
                val command = JSONObject(response ?: "{}")
                val data = command.optJSONObject("data")
                if (data == null || !data.optBoolean("enabled", false)) {
                    cancel(context)
                    return
                }
                val nextRunAt = data.optString("next_run_at", "")
                if (nextRunAt.trim().isEmpty()) {
                    cancel(context)
                    return
                }
                val delayMs = maxOf(0L, Duration.between(Instant.now(), Instant.parse(nextRunAt)).toMillis())
                val request = OneTimeWorkRequest.Builder(SchedulerWorker::class.java)
                    .setInitialDelay(delayMs, TimeUnit.MILLISECONDS)
                    .build()
                WorkManager.getInstance(context).enqueueUniqueWork(UNIQUE_WORK_NAME, ExistingWorkPolicy.REPLACE, request)
            } catch (error: Exception) {
                Log.e(TAG, "Failed to schedule Android worker", error)
                cancel(context)
            }
        }
    }
}
