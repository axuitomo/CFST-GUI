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
import java.io.File
import java.io.FileOutputStream
import java.nio.charset.StandardCharsets
import java.time.Instant
import java.time.LocalDate
import java.util.Locale
import java.util.concurrent.atomic.AtomicBoolean
import kotlin.concurrent.thread
import org.json.JSONObject

class LogMonitorForegroundService : Service() {
    private val stopping = AtomicBoolean(false)
    private var worker: Thread? = null
    private var runtimeDir: File? = null

    override fun onCreate() {
        super.onCreate()
        ensureNotificationChannel()
    }

    override fun onDestroy() {
        running = false
        stopping.set(true)
        worker?.interrupt()
        worker = null
        super.onDestroy()
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        if (intent?.action != ACTION_START) {
            stopSelf(startId)
            return START_NOT_STICKY
        }
        val dir = File(intent.getStringExtra(EXTRA_RUNTIME_DIR).orEmpty())
        if (dir.path.isBlank()) {
            stopSelf(startId)
            return START_NOT_STICKY
        }
        runtimeDir = dir
        return try {
            startForegroundCompat()
            running = true
            startMonitorWorker(dir)
            START_STICKY
        } catch (error: SecurityException) {
            Log.e(TAG, "Failed to start log monitor foreground service.", error)
            running = false
            writeMonitorLog(dir, "monitor.start_failed", JSONObject().put("message", error.message.orEmpty()))
            stopSelf(startId)
            START_NOT_STICKY
        }
    }

    override fun onBind(intent: Intent?): IBinder? = null

    private fun startMonitorWorker(dir: File) {
        val current = worker
        if (current != null && current.isAlive) {
            return
        }
        stopping.set(false)
        worker = thread(start = true, name = "cfst-log-monitor") {
            val state = MonitorState()
            writeMonitorLog(dir, "monitor.started", JSONObject().put("stale_after_ms", STALE_AFTER_MS))
            while (!stopping.get()) {
                val shouldStop = state.check(dir, Instant.now())
                if (shouldStop) {
                    break
                }
                try {
                    Thread.sleep(POLL_INTERVAL_MS)
                } catch (_: InterruptedException) {
                    break
                }
            }
            stopSelf()
        }
    }

    private fun ensureNotificationChannel() {
        val manager = getSystemService(Context.NOTIFICATION_SERVICE) as? NotificationManager
        if (manager == null || manager.getNotificationChannel(CHANNEL_ID) != null) {
            return
        }
        val channel = NotificationChannel(
            CHANNEL_ID,
            "CFST 日志监控",
            NotificationManager.IMPORTANCE_LOW,
        )
        channel.description = "记录 CFST 主进程心跳、卡住和异常退出状态。"
        manager.createNotificationChannel(channel)
    }

    private fun buildNotification(): Notification {
        val openAppIntent = openAppIntent()
        return NotificationCompat.Builder(this, CHANNEL_ID)
            .setContentTitle("CFST 进程监控运行中")
            .setContentText("正在记录主进程心跳和异常状态。")
            .setContentIntent(openAppIntent)
            .addAction(android.R.drawable.ic_menu_view, "打开", openAppIntent)
            .setCategory(NotificationCompat.CATEGORY_SERVICE)
            .setOnlyAlertOnce(true)
            .setOngoing(true)
            .setPriority(NotificationCompat.PRIORITY_LOW)
            .setSmallIcon(android.R.drawable.stat_notify_sync)
            .build()
    }

    private fun openAppIntent(): PendingIntent {
        val intent = Intent(this, MainActivity::class.java).apply {
            action = "io.github.axuitomo.cfstgui.action.OPEN_FROM_LOG_MONITOR"
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
        val notification = buildNotification()
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
            startForeground(NOTIFICATION_ID, notification, ServiceInfo.FOREGROUND_SERVICE_TYPE_DATA_SYNC)
            return
        }
        startForeground(NOTIFICATION_ID, notification)
    }

    private class Heartbeat(
        val lastSeenAt: Instant?,
        val pid: Int,
        val state: String,
    )

    private class MonitorState {
        private var heartbeatUnavailable = false
        private var hung = false

        fun check(runtimeDir: File, now: Instant): Boolean {
            val heartbeatFile = File(File(runtimeDir, "logs"), "main-heartbeat.json")
            val heartbeat = readHeartbeat(heartbeatFile)
            if (heartbeat == null) {
                if (!heartbeatUnavailable) {
                    heartbeatUnavailable = true
                    writeMonitorLog(runtimeDir, "main.heartbeat_unavailable", JSONObject().put("heartbeat_path", heartbeatFile.absolutePath))
                }
                return false
            }
            heartbeatUnavailable = false

            if (heartbeat.state == "shutdown") {
                writeMonitorLog(runtimeDir, "main.exited", JSONObject().put("pid", heartbeat.pid).put("state", heartbeat.state))
                return true
            }
            if (!processAlive(heartbeat.pid)) {
                writeMonitorLog(runtimeDir, "main.crashed_or_killed", JSONObject().put("pid", heartbeat.pid).put("state", heartbeat.state))
                return true
            }
            val staleDuration = if (heartbeat.lastSeenAt == null) STALE_AFTER_MS + 1 else now.toEpochMilli() - heartbeat.lastSeenAt.toEpochMilli()
            if (staleDuration > STALE_AFTER_MS) {
                if (!hung) {
                    hung = true
                    writeMonitorLog(
                        runtimeDir,
                        "main.hung",
                        JSONObject()
                            .put("pid", heartbeat.pid)
                            .put("stale_after_ms", STALE_AFTER_MS)
                            .put("stale_duration_ms", staleDuration)
                            .put("state", heartbeat.state),
                    )
                }
                return false
            }
            if (hung) {
                hung = false
                writeMonitorLog(runtimeDir, "main.recovered", JSONObject().put("pid", heartbeat.pid).put("state", heartbeat.state))
            }
            return false
        }

        private fun readHeartbeat(file: File): Heartbeat? {
            return try {
                val json = JSONObject(file.readText(StandardCharsets.UTF_8))
                val lastSeen = json.optString("last_seen_at", "").trim().let { value ->
                    if (value.isEmpty()) null else Instant.parse(value)
                }
                Heartbeat(
                    lastSeenAt = lastSeen,
                    pid = json.optInt("pid", 0),
                    state = json.optString("state", "running").trim().lowercase(Locale.ROOT),
                )
            } catch (_: Exception) {
                null
            }
        }

        private fun processAlive(pid: Int): Boolean {
            if (pid <= 0) {
                return true
            }
            return File("/proc/$pid").exists()
        }
    }

    companion object {
        private const val ACTION_START = "io.github.axuitomo.cfstgui.action.START_LOG_MONITOR"
        private const val CHANNEL_ID = "cfst_log_monitor"
        private const val EXTRA_RUNTIME_DIR = "runtime_dir"
        private const val NOTIFICATION_ID = 7040
        private const val POLL_INTERVAL_MS = 2_000L
        private const val STALE_AFTER_MS = 10_000L
        private const val TAG = "LogMonitorService"

        @Volatile
        private var running = false

        @JvmStatic
        fun isRunning(): Boolean = running

        @JvmStatic
        fun startIfConfigured(context: Context, runtimeDir: String): Boolean {
            val dir = File(runtimeDir.trim())
            if (dir.path.isBlank()) {
                return false
            }
            if (!monitorEnabled(dir)) {
                stop(context, runtimeDir)
                return false
            }
            return try {
                context.startForegroundService(startIntent(context, runtimeDir))
                true
            } catch (error: SecurityException) {
                Log.e(TAG, "Log monitor service could not be started.", error)
                writeMonitorLog(dir, "monitor.start_failed", JSONObject().put("message", error.message.orEmpty()))
                false
            } catch (error: IllegalStateException) {
                Log.e(TAG, "Log monitor service could not be started.", error)
                writeMonitorLog(dir, "monitor.start_failed", JSONObject().put("message", error.message.orEmpty()))
                false
            }
        }

        @JvmStatic
        fun stop(context: Context, runtimeDir: String): Boolean {
            running = false
            return context.stopService(startIntent(context, runtimeDir))
        }

        @JvmStatic
        fun startIntent(context: Context, runtimeDir: String): Intent =
            Intent(context, LogMonitorForegroundService::class.java).apply {
                action = ACTION_START
                putExtra(EXTRA_RUNTIME_DIR, runtimeDir)
            }

        @JvmStatic
        fun monitorEnabled(runtimeDir: File): Boolean {
            val configFile = File(runtimeDir, "mobile-config.json")
            if (!configFile.exists()) {
                return true
            }
            return try {
                val root = JSONObject(configFile.readText(StandardCharsets.UTF_8))
                val snapshot = root.optJSONObject("config_snapshot") ?: root
                val logging = snapshot.optJSONObject("logging") ?: return true
                if (logging.has("monitor_enabled")) {
                    logging.optBoolean("monitor_enabled", true)
                } else {
                    logging.optBoolean("monitorEnabled", true)
                }
            } catch (_: Exception) {
                true
            }
        }

        @JvmStatic
        fun checkHeartbeatForTest(runtimeDir: File, now: Instant): Boolean {
            return MonitorState().check(runtimeDir, now)
        }

        @JvmStatic
        fun writeMonitorLog(runtimeDir: File, event: String, fields: JSONObject = JSONObject()) {
            try {
                val logDir = File(runtimeDir, "logs")
                if (!logDir.exists()) {
                    logDir.mkdirs()
                }
                val entry = JSONObject()
                val data = JSONObject()
                entry.put("schema_version", "cfst-log-v1")
                entry.put("ts", Instant.now().toString())
                entry.put("level", "info")
                entry.put("channel", "monitor")
                entry.put("event", event.ifBlank { "monitor.event" })
                entry.put("data", data)
                val keys = fields.keys()
                while (keys.hasNext()) {
                    val key = keys.next()
                    if (key == "ts" || key == "event" || key == "schema_version" || key == "channel" || key.isBlank()) {
                        continue
                    }
                    val value = sanitizeMonitorValue(key, fields.opt(key))
                    if (key == "message" || key == "task_id" || key == "stage") {
                        entry.put(key, value)
                    } else if (key != "level") {
                        data.put(key, value)
                    }
                }
                val file = File(logDir, "monitor-${LocalDate.now()}.jsonl")
                FileOutputStream(file, true).use { output ->
                    output.write((entry.toString() + "\n").toByteArray(StandardCharsets.UTF_8))
                    output.fd.sync()
                }
            } catch (error: Exception) {
                Log.e(TAG, "Failed to write monitor log.", error)
            }
        }

        private fun sanitizeMonitorValue(key: String, value: Any?): Any? {
            val normalized = key.lowercase(Locale.ROOT)
            if (normalized.contains("token") || normalized.contains("password") || normalized.contains("secret")) {
                return "<redacted>"
            }
            return value
        }
    }
}
