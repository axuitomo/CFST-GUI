package io.github.axuitomo.cfstgui

import android.content.Context
import androidx.core.content.edit

/**
 * 记住前端当前的主题（浅色 / 深色），供启动窗口主题、Activity 窗口背景和 WebView 底色复用。
 * 主题由前端配置决定，原生只做缓存：冷启动和 WebView 重建时读缓存，保证过渡帧不闪白。
 */
object AndroidSurfaceTheme {
    private const val PREFS_NAME = "cfst_android_surface_theme"
    private const val KEY_DARK = "dark"

    @JvmStatic
    fun dark(context: Context): Boolean {
        return context.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE).getBoolean(KEY_DARK, false)
    }

    @JvmStatic
    fun setDark(context: Context, dark: Boolean) {
        context.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE).edit {
            putBoolean(KEY_DARK, dark)
        }
    }
}
