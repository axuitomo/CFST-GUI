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

class AndroidKeepAliveForegroundService : Service() {
    private val uploadNotificationListener = CfstRuntime.ProbeEventListener { eventJSON ->
        if (AndroidUploadNotificationState.recordFromEvent(this, eventJSON)) {
            updateNotification()
        }
    }

    override fun onCreate() {
        super.onCreate()
        ensureNotificationChannel()
        CfstRuntime.registerAuxiliaryListener(uploadNotificationListener)
    }

    override fun onDestroy() {
        running = false
        CfstRuntime.unregisterAuxiliaryListener(uploadNotificationListener)
        super.onDestroy()
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        if (intent?.action != ACTION_START) {
            stopSelf(startId)
            return START_NOT_STICKY
        }
        return try {
            startForegroundCompat()
            running = true
            START_STICKY
        } catch (error: SecurityException) {
            Log.e(TAG, "Failed to start Android keep-alive foreground service.", error)
            running = false
            stopSelf(startId)
            START_NOT_STICKY
        }
    }

    override fun onBind(intent: Intent?): IBinder? = null

    private fun ensureNotificationChannel() {
        val manager = getSystemService(Context.NOTIFICATION_SERVICE) as? NotificationManager
        if (manager == null || manager.getNotificationChannel(CHANNEL_ID) != null) {
            return
        }
        val channel = NotificationChannel(
            CHANNEL_ID,
            "CFST 后台保活",
            NotificationManager.IMPORTANCE_LOW,
        )
        channel.description = "保持 CFST Android 后台服务常驻，提高定时任务稳定性。"
        manager.createNotificationChannel(channel)
    }

    private fun buildNotification(content: String): Notification {
        val openAppIntent = openAppIntent()
        return NotificationCompat.Builder(this, CHANNEL_ID)
            .setContentTitle("CFST 后台保活已开启")
            .setContentText(content)
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
            action = "io.github.axuitomo.cfstgui.action.OPEN_FROM_KEEP_ALIVE"
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
        val notification = buildNotification(AndroidUploadNotificationState.notificationText(this))
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
            startForeground(NOTIFICATION_ID, notification, ServiceInfo.FOREGROUND_SERVICE_TYPE_DATA_SYNC)
            return
        }
        startForeground(NOTIFICATION_ID, notification)
    }

    private fun updateNotification() {
        val manager = getSystemService(Context.NOTIFICATION_SERVICE) as? NotificationManager ?: return
        manager.notify(NOTIFICATION_ID, buildNotification(AndroidUploadNotificationState.notificationText(this)))
    }

    companion object {
        private const val ACTION_START = "io.github.axuitomo.cfstgui.action.START_KEEP_ALIVE"
        private const val CHANNEL_ID = "cfst_keep_alive"
        private const val NOTIFICATION_ID = 7030
        private const val TAG = "AndroidKeepAlive"

        @Volatile
        private var running = false

        @JvmStatic
        fun isRunning(): Boolean = running

        @JvmStatic
        fun startIntent(context: Context): Intent =
            Intent(context, AndroidKeepAliveForegroundService::class.java).apply {
                action = ACTION_START
            }
    }
}
