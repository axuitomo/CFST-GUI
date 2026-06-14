package io.github.axuitomo.cfstgui

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.util.Log

class UpdatePackageCleanupReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context?, intent: Intent?) {
        if (context == null || intent?.action != Intent.ACTION_MY_PACKAGE_REPLACED) {
            return
        }
        try {
            AndroidUpdateInstaller.cleanupDownloadedPackages(context)
        } catch (error: Exception) {
            Log.e(TAG, "Failed to clean Android update APK after package replacement.", error)
        }
    }

    private companion object {
        private const val TAG = "UpdatePackageCleanup"
    }
}
