package io.github.axuitomo.cfstgui

import android.content.Context
import android.content.Intent
import android.net.Uri
import com.getcapacitor.JSObject

object AndroidExternalNavigation {
    fun interface ActivityStarter {
        fun startActivity(intent: Intent)
    }

    @JvmStatic
    fun openReleasePageCommand(context: Context): JSObject {
        return openReleasePageCommand(context, ActivityStarter { intent -> context.startActivity(intent) })
    }

    @JvmStatic
    fun openReleasePageCommand(context: Context, starter: ActivityStarter): JSObject {
        val intent = Intent(Intent.ACTION_VIEW, Uri.parse(AndroidUpdateRelease.RELEASE_PAGE_URL)).apply {
            addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
        }
        starter.startActivity(intent)
        val data = JSObject()
        data.put("release_url", AndroidUpdateRelease.RELEASE_PAGE_URL)
        return AndroidPluginCommands.command("RELEASE_OPENED", data, "已打开发行页。", true)
    }

    @JvmStatic
    fun openPathCommand(context: Context, targetPath: String?): JSObject {
        AndroidTargetOpener.openTargetPath(context, targetPath)
        return openPathCommandData(targetPath)
    }

    @JvmStatic
    fun openPathCommand(context: Context, targetPath: String?, starter: AndroidTargetOpener.IntentStarter): JSObject {
        AndroidTargetOpener.openTargetPath(context, targetPath, starter)
        return openPathCommandData(targetPath)
    }

    private fun openPathCommandData(targetPath: String?): JSObject {
        val data = JSObject()
        data.put("target_path", targetPath?.trim().orEmpty())
        return AndroidPluginCommands.command("OPEN_PATH_OK", data, "已打开目标。", true)
    }
}
