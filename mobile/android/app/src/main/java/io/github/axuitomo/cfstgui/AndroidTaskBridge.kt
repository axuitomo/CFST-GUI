package io.github.axuitomo.cfstgui

import android.content.Context

object AndroidTaskBridge {
    fun interface PayloadAction {
        fun call(payloadJSON: String): String
    }

    @JvmStatic
    fun cancelProbe(context: Context, payloadJSON: String, action: PayloadAction): String {
        return AndroidPluginCommands.finalizeServiceResponse(context, action.call(payloadJSON))
    }

    @JvmStatic
    fun resumeProbe(context: Context, payloadJSON: String, action: PayloadAction): String {
        return AndroidPluginCommands.finalizeServiceResponse(context, action.call(payloadJSON))
    }

    @JvmStatic
    fun listResultFile(context: Context, payloadJSON: String, action: PayloadAction): String {
        val rewrittenPayload = AndroidPayloads.withPrivateResultFilePath(context, payloadJSON)
        return AndroidPluginCommands.finalizeServiceResponse(context, action.call(rewrittenPayload))
    }
}
