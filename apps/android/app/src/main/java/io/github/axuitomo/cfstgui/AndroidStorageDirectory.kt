package io.github.axuitomo.cfstgui

import android.content.Context
import com.getcapacitor.JSObject
import org.json.JSONArray

object AndroidStorageDirectory {
    fun interface RuntimeInitializer {
        fun init(runtimeDir: String): String
    }

    @JvmStatic
    fun commandForDeprecatedChange(context: Context, runtimeInitializer: RuntimeInitializer): JSObject {
        val next = AndroidStorageState.defaultStorageBootstrap(context)
        next.put("setup_completed", true)
        AndroidStorageState.writeStorageBootstrap(context, next)
        runtimeInitializer.init(AndroidStorageState.defaultRuntimeDir(context).absolutePath)

        val data = JSObject()
        data.put("migration", emptyMigrationPayload())
        data.put("storage", AndroidStorageState.currentStorageStatus(context))
        return AndroidPluginCommands.command(
            "STORAGE_SET_DEPRECATED",
            data,
            "当前版本不再支持自定义储存目录，Android 固定使用应用私有目录。",
            true,
        )
    }

    private fun emptyMigrationPayload(): JSObject {
        val migration = JSObject()
        migration.put("copied", JSONArray())
        migration.put("failed", JSONArray())
        migration.put("skipped", JSONArray())
        return migration
    }
}
