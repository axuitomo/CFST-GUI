package io.github.axuitomo.cfstgui

import android.content.Context
import android.util.Log
import androidx.work.ExistingWorkPolicy
import androidx.work.OneTimeWorkRequest
import androidx.work.WorkManager
import androidx.work.Worker
import androidx.work.WorkerParameters
import org.json.JSONObject
import java.time.Duration
import java.time.Instant
import java.util.concurrent.TimeUnit

class SchedulerWorker(context: Context, workerParams: WorkerParameters) : Worker(context, workerParams) {
    override fun doWork(): Result {
        val context = applicationContext
        return try {
            // Do NOT call setForegroundAsync() here: on Android 14+ / targetSdk 34+
            // WorkManager's SystemForegroundService can start with an empty
            // foregroundServiceType on a cold start, which the system rejects with
            // InvalidForegroundServiceTypeException. The scheduled probe instead runs
            // inside our own ProbeForegroundService, which declares dataSync and calls
            // startForeground with the correct type. The worker only needs to survive
            // long enough to hand off to that service.
            CfstRuntime.ensureInitialized(context, AndroidStorageState.defaultRuntimeDir(context).absolutePath)
            val serviceIntent = ProbeForegroundService.startScheduledIntent(context)
            context.startForegroundService(serviceIntent)
            Result.success()
        } catch (error: Exception) {
            Log.e(TAG, "Android scheduled probe failed", error)
            try {
                scheduleFromStatus(context, CfstRuntime.service().invoke("scheduler.refresh", "{}"))
            } catch (_: Exception) {
                // Keep WorkManager failure handling simple; scheduler can be rearmed on next config save/app launch.
                Log.w(TAG, "Failed to re-arm Android scheduler after probe failure", error)
            }
            Result.failure()
        }
    }

    companion object {
        private const val TAG = "SchedulerWorker"
        private const val UNIQUE_WORK_NAME = "cfst-android-scheduler"

        @JvmStatic
        fun refresh(context: Context): String {
            CfstRuntime.ensureInitialized(context, AndroidStorageState.defaultRuntimeDir(context).absolutePath)
            val response = CfstRuntime.service().invoke("scheduler.refresh", "{}")
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
