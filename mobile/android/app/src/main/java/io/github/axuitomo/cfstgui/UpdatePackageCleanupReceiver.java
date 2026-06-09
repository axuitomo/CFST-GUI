package io.github.axuitomo.cfstgui;

import android.content.BroadcastReceiver;
import android.content.Context;
import android.content.Intent;
import android.util.Log;
import java.io.File;

public class UpdatePackageCleanupReceiver extends BroadcastReceiver {
    private static final String TAG = "UpdatePackageCleanup";

    @Override
    public void onReceive(Context context, Intent intent) {
        if (context == null || intent == null || !Intent.ACTION_MY_PACKAGE_REPLACED.equals(intent.getAction())) {
            return;
        }
        try {
            CfstPlugin.cleanupAndroidUpdatePackages(new File(context.getFilesDir(), "updates"));
        } catch (Exception error) {
            Log.e(TAG, "Failed to clean Android update APK after package replacement.", error);
        }
    }
}
