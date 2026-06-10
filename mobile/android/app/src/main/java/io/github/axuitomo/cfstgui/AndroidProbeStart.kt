package io.github.axuitomo.cfstgui

import android.content.Context
import android.content.Intent
import com.getcapacitor.JSObject

object AndroidProbeStart {
    fun interface StartGate {
        fun markStartQueuedIfIdle(): Boolean
    }

    fun interface ForegroundServiceStarter {
        fun startForegroundService(intent: Intent)
    }

    fun interface ExportTargetValidator {
        fun requireWritableTarget(context: Context, exportURI: String)
    }

    @JvmStatic
    fun startProbe(context: Context, payload: String, taskId: String): JSObject {
        return startProbe(
            context,
            payload,
            taskId,
            StartGate { ProbeForegroundService.markStartQueuedIfIdle() },
            ExportTargetValidator { targetContext, exportURI ->
                AndroidStorageBridge.ensureWritablePersistentExportTarget(targetContext, exportURI)
            },
            ForegroundServiceStarter { intent -> context.startForegroundService(intent) },
        )
    }

    @JvmStatic
    fun startProbe(
        context: Context,
        payload: String,
        taskId: String,
        startGate: StartGate,
        starter: ForegroundServiceStarter,
    ): JSObject {
        return startProbe(
            context,
            payload,
            taskId,
            startGate,
            ExportTargetValidator { targetContext, exportURI ->
                AndroidStorageBridge.ensureWritablePersistentExportTarget(targetContext, exportURI)
            },
            starter,
        )
    }

    @JvmStatic
    fun startProbe(
        context: Context,
        payload: String,
        taskId: String,
        startGate: StartGate,
        targetValidator: ExportTargetValidator,
        starter: ForegroundServiceStarter,
    ): JSObject {
        if (!startGate.markStartQueuedIfIdle()) {
            return commandForAlreadyRunning(taskId)
        }
        try {
            val exportURI = AndroidPayloads.extractExportTargetURI(payload)
            targetValidator.requireWritableTarget(context, exportURI)
            val normalizedPayload = AndroidPayloads.withAndroidExportURI(payload, exportURI)
            starter.startForegroundService(ProbeForegroundService.startIntent(context, normalizedPayload, exportURI))
            return commandForAccepted(taskId, exportURI)
        } catch (error: Exception) {
            ProbeForegroundService.clearQueuedStart()
            throw error
        }
    }

    @JvmStatic
    fun commandForAlreadyRunning(taskId: String?): JSObject {
        val data = JSObject()
        data.put("accepted", false)
        data.put("task_id", taskId.orEmpty())
        return AndroidPluginCommands.command(
            "PROBE_ALREADY_RUNNING",
            data,
            "当前已有探测任务运行或暂停，请完成后再启动新任务。",
            false,
        )
    }

    @JvmStatic
    fun commandForAccepted(taskId: String?, exportURI: String?): JSObject {
        val data = JSObject()
        data.put("accepted", true)
        data.put("export_path", exportURI.orEmpty())
        data.put("task_id", taskId.orEmpty())
        return AndroidPluginCommands.command("PROBE_ACCEPTED", data, "移动端探测任务已提交到前台服务。", true)
    }
}
