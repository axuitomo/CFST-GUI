package io.github.axuitomo.cfstgui;

import android.app.Notification;
import android.app.NotificationChannel;
import android.app.NotificationManager;
import android.content.Context;
import android.content.Intent;
import android.os.Build;
import android.util.Log;
import androidx.annotation.NonNull;
import androidx.core.app.NotificationCompat;
import androidx.work.ExistingWorkPolicy;
import androidx.work.ForegroundInfo;
import androidx.work.OneTimeWorkRequest;
import androidx.work.WorkManager;
import androidx.work.Worker;
import androidx.work.WorkerParameters;
import java.time.Duration;
import java.time.Instant;
import java.util.concurrent.TimeUnit;
import org.json.JSONObject;

public class SchedulerWorker extends Worker {
    private static final String CHANNEL_ID = "cfst_scheduler";
    private static final int NOTIFICATION_ID = 7020;
    private static final String TAG = "SchedulerWorker";
    private static final String UNIQUE_WORK_NAME = "cfst-android-scheduler";

    public SchedulerWorker(@NonNull Context context, @NonNull WorkerParameters workerParams) {
        super(context, workerParams);
    }

    static String refresh(Context context) {
        CfstRuntime.ensureInitialized(context, CfstPlugin.defaultRuntimeDirStatic(context).getAbsolutePath());
        String response = CfstRuntime.service().refreshScheduler("{}");
        scheduleFromStatus(context, response);
        return response;
    }

    static void cancel(Context context) {
        WorkManager.getInstance(context).cancelUniqueWork(UNIQUE_WORK_NAME);
    }

    @NonNull
    @Override
    public Result doWork() {
        Context context = getApplicationContext();
        try {
            setForegroundAsync(createForegroundInfo());
            CfstRuntime.ensureInitialized(context, CfstPlugin.defaultRuntimeDirStatic(context).getAbsolutePath());
            Intent serviceIntent = ProbeForegroundService.startScheduledIntent(context);
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
                context.startForegroundService(serviceIntent);
            } else {
                context.startService(serviceIntent);
            }
            return Result.success();
        } catch (Exception error) {
            Log.e(TAG, "Android scheduled probe failed", error);
            try {
                scheduleFromStatus(context, CfstRuntime.service().loadSchedulerStatus());
            } catch (Exception ignored) {
                // Keep WorkManager failure handling simple; scheduler can be rearmed on next config save/app launch.
            }
            return Result.failure();
        }
    }

    static void scheduleFromStatus(Context context, String response) {
        try {
            JSONObject command = new JSONObject(response == null ? "{}" : response);
            JSONObject data = command.optJSONObject("data");
            if (data == null || !data.optBoolean("enabled", false)) {
                cancel(context);
                return;
            }
            String nextRunAt = data.optString("next_run_at", "");
            if (nextRunAt.trim().isEmpty()) {
                cancel(context);
                return;
            }
            long delayMs = Math.max(0L, Duration.between(Instant.now(), Instant.parse(nextRunAt)).toMillis());
            OneTimeWorkRequest request = new OneTimeWorkRequest.Builder(SchedulerWorker.class)
                .setInitialDelay(delayMs, TimeUnit.MILLISECONDS)
                .build();
            WorkManager.getInstance(context).enqueueUniqueWork(UNIQUE_WORK_NAME, ExistingWorkPolicy.REPLACE, request);
        } catch (Exception error) {
            Log.e(TAG, "Failed to schedule Android worker", error);
            cancel(context);
        }
    }

    private ForegroundInfo createForegroundInfo() {
        ensureNotificationChannel();
        Notification notification = new NotificationCompat.Builder(getApplicationContext(), CHANNEL_ID)
            .setSmallIcon(android.R.drawable.stat_notify_sync)
            .setContentTitle("CFST 定时任务")
            .setContentText("正在执行 Android 定时测速。")
            .setOngoing(true)
            .setPriority(NotificationCompat.PRIORITY_LOW)
            .build();
        return new ForegroundInfo(NOTIFICATION_ID, notification);
    }

    private void ensureNotificationChannel() {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.O) {
            return;
        }
        NotificationManager manager = (NotificationManager) getApplicationContext().getSystemService(Context.NOTIFICATION_SERVICE);
        if (manager == null || manager.getNotificationChannel(CHANNEL_ID) != null) {
            return;
        }
        manager.createNotificationChannel(new NotificationChannel(CHANNEL_ID, "CFST 定时任务", NotificationManager.IMPORTANCE_LOW));
    }
}
