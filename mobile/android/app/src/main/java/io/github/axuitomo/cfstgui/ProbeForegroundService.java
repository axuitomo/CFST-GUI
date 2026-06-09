package io.github.axuitomo.cfstgui;

import android.app.Notification;
import android.app.NotificationChannel;
import android.app.NotificationManager;
import android.app.PendingIntent;
import android.app.Service;
import android.content.Context;
import android.content.Intent;
import android.os.Build;
import android.os.IBinder;
import android.util.Log;
import androidx.core.app.NotificationCompat;
import java.util.Locale;
import org.json.JSONArray;
import org.json.JSONObject;

public class ProbeForegroundService extends Service {
    private static final String ACTION_START = "io.github.axuitomo.cfstgui.action.START_PROBE";
    private static final String ACTION_START_SCHEDULED = "io.github.axuitomo.cfstgui.action.START_SCHEDULED_PROBE";
    private static final String CHANNEL_ID = "cfst_probe";
    private static final int NOTIFICATION_ID = 7010;
    private static final String EXTRA_PAYLOAD = "payload";
    private static final String EXTRA_EXPORT_URI = "export_uri";
    private static final String TAG = "ProbeFgService";
    private static final Object START_LOCK = new Object();
    private static volatile boolean startQueued = false;
    private static volatile boolean foregroundRunning = false;
    private final CfstRuntime.ProbeEventListener notificationListener = this::handleProbeEvent;
    private volatile String currentTaskId = "";

    static boolean markStartQueuedIfIdle() {
        synchronized (START_LOCK) {
            if (startQueued || CfstRuntime.hasRunningOrPausedTask()) {
                return false;
            }
            startQueued = true;
            return true;
        }
    }

    static void clearQueuedStart() {
        synchronized (START_LOCK) {
            startQueued = false;
        }
    }

    static boolean isForegroundRunning() {
        return foregroundRunning;
    }

    static Intent startIntent(Context context, String payload, String exportURI) {
        Intent intent = new Intent(context, ProbeForegroundService.class);
        intent.setAction(ACTION_START);
        intent.putExtra(EXTRA_PAYLOAD, payload);
        intent.putExtra(EXTRA_EXPORT_URI, exportURI == null ? "" : exportURI);
        return intent;
    }

    static Intent startScheduledIntent(Context context) {
        Intent intent = new Intent(context, ProbeForegroundService.class);
        intent.setAction(ACTION_START_SCHEDULED);
        return intent;
    }

    @Override
    public void onCreate() {
        super.onCreate();
        ensureNotificationChannel();
        CfstRuntime.registerAuxiliaryListener(notificationListener);
    }

    @Override
    public void onDestroy() {
        foregroundRunning = false;
        CfstRuntime.unregisterAuxiliaryListener(notificationListener);
        super.onDestroy();
    }

    @Override
    public int onStartCommand(Intent intent, int flags, int startId) {
        String action = intent == null ? "" : intent.getAction();
        if (!ACTION_START.equals(action) && !ACTION_START_SCHEDULED.equals(action)) {
            clearQueuedStart();
            stopSelf(startId);
            return START_NOT_STICKY;
        }
        if (ACTION_START.equals(action)) {
            currentTaskId = extractTaskId(intent.getStringExtra(EXTRA_PAYLOAD));
        } else {
            currentTaskId = "";
        }
        try {
            startForegroundCompat();
            foregroundRunning = true;
        } catch (SecurityException error) {
            Log.e(TAG, "Failed to promote probe service to foreground", error);
            foregroundRunning = false;
            clearQueuedStart();
            JSONObject payload = new JSONObject();
            try {
                payload.put("message", "Android 前台服务启动失败，请检查应用通知/前台服务权限后重试。");
                payload.put("recoverable", false);
            } catch (Exception ignored) {
                // Ignore JSON fallback failure and still stop the service safely.
            }
            CfstRuntime.emitSyntheticProbeEvent(currentTaskId, "probe.failed", payload);
            if (ACTION_START_SCHEDULED.equals(action)) {
                try {
                    SchedulerWorker.scheduleFromStatus(getApplicationContext(), CfstRuntime.service().refreshScheduler("{}"));
                } catch (Exception ignored) {
                    // Scheduler can be rearmed on next app start or config save.
                }
            }
            stopSelf(startId);
            return START_NOT_STICKY;
        }
        if (ACTION_START.equals(action)) {
            final String payload = intent.getStringExtra(EXTRA_PAYLOAD);
            final String exportURI = intent.getStringExtra(EXTRA_EXPORT_URI);
            synchronized (START_LOCK) {
                if (!startQueued && CfstRuntime.hasRunningOrPausedTask()) {
                    Log.w(TAG, "Ignored duplicated background task start request because another task is already queued or active.");
                    stopForeground(STOP_FOREGROUND_REMOVE);
                    stopSelf(startId);
                    return START_NOT_STICKY;
                }
                startQueued = true;
            }
            CfstRuntime.executor().execute(() -> {
                String taskIdForFailure = currentTaskId;
                try {
                    String response = CfstRuntime.service().runProbe(payload == null ? "{}" : payload);
                    if (exportURI != null && !exportURI.trim().isEmpty()) {
                        response = CfstPlugin.copyProbeExportToURIStatic(getApplicationContext(), response, exportURI);
                        recordAndroidExportResult(currentTaskId, response, exportURI);
                    }
                    if (response == null || response.trim().isEmpty()) {
                        Log.w(TAG, "Foreground task finished without command response.");
                    }
                } catch (Exception error) {
                    Log.e(TAG, "Foreground task execution failed", error);
                    emitForegroundTaskFailure(taskIdForFailure, error);
                } finally {
                    clearQueuedStart();
                    stopForeground(STOP_FOREGROUND_REMOVE);
                    stopSelf(startId);
                }
            });
        } else if (ACTION_START_SCHEDULED.equals(action)) {
            boolean scheduledStartAlreadyQueued = false;
            synchronized (START_LOCK) {
                if (startQueued) {
                    scheduledStartAlreadyQueued = true;
                } else {
                    startQueued = true;
                }
            }
            if (scheduledStartAlreadyQueued) {
                Log.w(TAG, "Ignored scheduled background task because another task start is already queued or active.");
                try {
                    SchedulerWorker.scheduleFromStatus(getApplicationContext(), CfstRuntime.service().refreshScheduler("{}"));
                } catch (Exception ignored) {
                    // Scheduler can be rearmed on next app start or config save.
                }
                stopForeground(STOP_FOREGROUND_REMOVE);
                stopSelf(startId);
                return START_NOT_STICKY;
            }
            CfstRuntime.executor().execute(() -> {
                String taskIdForFailure = currentTaskId;
                try {
                    String response = CfstRuntime.service().runScheduledProbe("{}");
                    SchedulerWorker.scheduleFromStatus(getApplicationContext(), response);
                    if (response == null || response.trim().isEmpty()) {
                        Log.w(TAG, "Scheduled foreground task finished without command response.");
                    }
                } catch (Exception error) {
                    Log.e(TAG, "Scheduled foreground task execution failed", error);
                    emitForegroundTaskFailure(taskIdForFailure, error);
                    try {
                        SchedulerWorker.scheduleFromStatus(getApplicationContext(), CfstRuntime.service().refreshScheduler("{}"));
                    } catch (Exception ignored) {
                        // Scheduler can be rearmed on next app start or config save.
                    }
                } finally {
                    clearQueuedStart();
                    stopForeground(STOP_FOREGROUND_REMOVE);
                    stopSelf(startId);
                }
            });
        }
        return START_NOT_STICKY;
    }

    private void recordAndroidExportResult(String fallbackTaskId, String responseJSON, String exportURI) {
        try {
            JSONObject command = new JSONObject(responseJSON == null ? "{}" : responseJSON);
            JSONObject data = command.optJSONObject("data");
            if (data == null) {
                return;
            }
            String status = firstNonEmpty(data.optString("android_export_status", ""), data.optString("androidExportStatus", ""));
            if (status.isEmpty()) {
                return;
            }
            String targetURI = firstNonEmpty(data.optString("android_export_uri", ""), data.optString("androidExportUri", ""), exportURI);
            String sourcePath = firstNonEmpty(data.optString("android_export_source_path", ""), data.optString("androidExportSourcePath", ""));
            String message = firstNonEmpty(data.optString("android_export_error", ""), data.optString("androidExportError", ""));
            boolean written = "written".equals(status);
            if (message.isEmpty()) {
                message = written ? "Android 系统导出文件已写入。" : "Android 系统导出文件失败。";
            }

            JSONObject payload = new JSONObject();
            payload.put("ok", written);
            payload.put("message", message);
            payload.put("source_path", sourcePath);
            payload.put("status", status);
            payload.put("target_uri", targetURI);
            payload.put("task_id", firstNonEmpty(command.optString("task_id", ""), fallbackTaskId));
            payload.put("written", exportedCountFromProbeData(data));
            CfstRuntime.service().recordAndroidExportResult(payload.toString());
        } catch (Exception error) {
            Log.e(TAG, "Failed to record Android export result", error);
            emitAndroidExportFailure(fallbackTaskId, exportURI, error.getMessage());
        }
    }

    private int exportedCountFromProbeData(JSONObject data) {
        JSONArray results = data.optJSONArray("results");
        if (results != null) {
            return results.length();
        }
        JSONObject summary = data.optJSONObject("summary");
        if (summary != null) {
            return Math.max(summary.optInt("passed", 0), summary.optInt("total", 0));
        }
        return data.optInt("exported", 0);
    }

    private void emitAndroidExportFailure(String taskId, String exportURI, String message) {
        try {
            JSONObject payload = new JSONObject();
            payload.put("message", message == null || message.trim().isEmpty() ? "Android 系统导出状态记录失败。" : message);
            payload.put("recoverable", true);
            payload.put("stage", "export");
            payload.put("target_path", exportURI == null ? "" : exportURI.trim());
            CfstRuntime.emitSyntheticProbeEvent(taskId, "probe.export_failed", payload);
        } catch (Exception ignored) {
            // Synthetic event fallback should never crash foreground cleanup.
        }
    }

    private void emitForegroundTaskFailure(String taskId, Exception error) {
        try {
            JSONObject payload = new JSONObject();
            String message = error == null ? "" : error.getMessage();
            if (message == null || message.trim().isEmpty()) {
                message = "后台任务执行失败。";
            }
            payload.put("message", message);
            payload.put("recoverable", false);
            CfstRuntime.emitSyntheticProbeEvent(taskId, "probe.failed", payload);
        } catch (Exception ignored) {
            // Synthetic event fallback should never crash foreground cleanup.
        }
    }

    private String firstNonEmpty(String... values) {
        for (String value : values) {
            if (value != null && !value.trim().isEmpty()) {
                return value.trim();
            }
        }
        return "";
    }

    @Override
    public IBinder onBind(Intent intent) {
        return null;
    }

    private void ensureNotificationChannel() {
        NotificationManager manager = (NotificationManager) getSystemService(Context.NOTIFICATION_SERVICE);
        if (manager == null || manager.getNotificationChannel(CHANNEL_ID) != null) {
            return;
        }
        NotificationChannel channel = new NotificationChannel(
            CHANNEL_ID,
            "CFST 后台任务",
            NotificationManager.IMPORTANCE_LOW
        );
        channel.setDescription("保持 CFST Android 长任务在前台服务中执行。");
        manager.createNotificationChannel(channel);
    }

    private Notification buildNotification(String title, String content) {
        PendingIntent openAppIntent = openAppIntent();
        return new NotificationCompat.Builder(this, CHANNEL_ID)
            .setContentTitle(title)
            .setContentText(content)
            .setContentIntent(openAppIntent)
            .addAction(android.R.drawable.ic_menu_view, "打开", openAppIntent)
            .setCategory(NotificationCompat.CATEGORY_SERVICE)
            .setOnlyAlertOnce(true)
            .setOngoing(true)
            .setSmallIcon(android.R.drawable.stat_sys_download_done)
            .build();
    }

    private PendingIntent openAppIntent() {
        Intent intent = new Intent(this, MainActivity.class);
        intent.setAction("io.github.axuitomo.cfstgui.action.OPEN_FROM_NOTIFICATION");
        intent.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK | Intent.FLAG_ACTIVITY_CLEAR_TOP | Intent.FLAG_ACTIVITY_SINGLE_TOP);
        return PendingIntent.getActivity(
            this,
            NOTIFICATION_ID,
            intent,
            PendingIntent.FLAG_UPDATE_CURRENT | PendingIntent.FLAG_IMMUTABLE
        );
    }

    private void startForegroundCompat() {
        Notification notification = buildNotification("任务运行中", "CFST 正在执行后台任务。");
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
            startForeground(NOTIFICATION_ID, notification, android.content.pm.ServiceInfo.FOREGROUND_SERVICE_TYPE_DATA_SYNC);
            return;
        }
        startForeground(NOTIFICATION_ID, notification);
    }

    private void updateNotification(String title, String content) {
        NotificationManager manager = (NotificationManager) getSystemService(Context.NOTIFICATION_SERVICE);
        if (manager == null) {
            return;
        }
        manager.notify(NOTIFICATION_ID, buildNotification(title, content));
    }

    private void handleProbeEvent(String eventJSON) {
        try {
            JSONObject event = new JSONObject(eventJSON);
            String taskId = event.optString("task_id", "").trim();
            if (!taskId.isEmpty()) {
                currentTaskId = taskId;
            }
            JSONObject payload = event.optJSONObject("payload");
            String name = event.optString("event", "");
            switch (name) {
                case "probe.preprocessed":
                    updateNotification("正在准备任务", formatPreprocessContent(payload));
                    break;
                case "probe.progress":
                    updateNotification(stageTitle(payload == null ? "" : payload.optString("stage", "")), formatProgressContent(payload));
                    break;
                case "probe.speed":
                    updateNotification("文件测速中", formatSpeedContent(payload));
                    break;
                case "probe.partial_export":
                    updateNotification("结果已落盘", formatExportContent(payload));
                    break;
                case "probe.cooling":
                    updateNotification("任务冷却中", payload == null ? "任务正在等待恢复或安全停止。" : payload.optString("reason", "任务正在冷却。"));
                    break;
                case "probe.completed":
                    updateNotification("任务已完成", formatCompletedContent(payload));
                    break;
                case "probe.failed":
                    updateNotification("任务失败", payload == null ? "后台探测任务失败。" : payload.optString("message", "后台探测任务失败。"));
                    break;
                default:
                    break;
            }
        } catch (Exception error) {
            Log.e(TAG, "Failed to update notification from probe event", error);
        }
    }

    private String formatPreprocessContent(JSONObject payload) {
        if (payload == null) {
            return "正在整理输入源与候选 IP。";
        }
        int accepted = payload.optInt("accepted", 0);
        int filtered = payload.optInt("filtered", 0);
        int invalid = payload.optInt("invalid", 0);
        return String.format(Locale.ROOT, "候选 %d，通过 %d，过滤 %d。", accepted + filtered + invalid, accepted, filtered + invalid);
    }

    private String formatProgressContent(JSONObject payload) {
        if (payload == null) {
            return "探测任务正在执行。";
        }
        int processed = payload.optInt("processed", 0);
        int total = payload.optInt("total", 0);
        int passed = payload.optInt("passed", 0);
        int failed = payload.optInt("failed", 0);
        if (total > 0) {
            return String.format(Locale.ROOT, "已处理 %d/%d，通过 %d，失败 %d。", processed, total, passed, failed);
        }
        return String.format(Locale.ROOT, "已处理 %d，通过 %d，失败 %d。", processed, passed, failed);
    }

    private String formatSpeedContent(JSONObject payload) {
        if (payload == null) {
            return "正在采集测速样本。";
        }
        String ip = payload.optString("ip", "当前 IP");
        double current = payload.optDouble("current_speed_mb_s", 0);
        double average = payload.optDouble("average_speed_mb_s", 0);
        if (average > 0) {
            return String.format(Locale.ROOT, "%s 当前 %.2f MB/s，均速 %.2f MB/s。", ip, current, average);
        }
        if (current > 0) {
            return String.format(Locale.ROOT, "%s 当前 %.2f MB/s。", ip, current);
        }
        return ip + " 正在测速中。";
    }

    private String formatExportContent(JSONObject payload) {
        if (payload == null) {
            return "已有部分结果写入磁盘。";
        }
        int written = payload.optInt("written", 0);
        String target = payload.optString("target_path", "");
        return target.isEmpty()
            ? String.format(Locale.ROOT, "已写出 %d 条结果。", written)
            : String.format(Locale.ROOT, "已写出 %d 条结果到导出文件。", written);
    }

    private String formatCompletedContent(JSONObject payload) {
        if (payload == null) {
            return "后台探测任务已完成。";
        }
        int results = payload.optInt("result_count", payload.optInt("passed", payload.optInt("exported", 0)));
        return results > 0
            ? String.format(Locale.ROOT, "任务完成，可用结果 %d 条。", results)
            : "任务完成，但当前没有可用结果。";
    }

    private String stageTitle(String stage) {
        String normalized = stage == null ? "" : stage.trim();
        switch (normalized) {
            case "stage0_pool":
                return "输入预处理";
            case "stage1_tcp":
                return "TCP测延迟";
            case "stage2_trace":
            case "stage2_head":
                return "追踪探测";
            case "stage3_get":
                return "文件测速";
            default:
                return "任务运行中";
        }
    }

    private String extractTaskId(String payloadJSON) {
        if (payloadJSON == null || payloadJSON.trim().isEmpty()) {
            return "";
        }
        try {
            JSONObject payload = new JSONObject(payloadJSON);
            return firstNonEmpty(
                payload.optString("task_id", ""),
                payload.optString("taskId", "")
            );
        } catch (Exception ignored) {
            return "";
        }
    }

}
