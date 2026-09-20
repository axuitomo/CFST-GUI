package io.github.axuitomo.cfstgui

import android.content.pm.ApplicationInfo
import android.graphics.Color
import android.graphics.drawable.ColorDrawable
import android.os.Build
import android.os.Bundle
import android.util.Log
import android.view.WindowManager
import android.webkit.RenderProcessGoneDetail
import android.webkit.WebSettings
import android.webkit.WebView
import androidx.appcompat.app.AppCompatDelegate
import androidx.core.content.ContextCompat
import androidx.core.view.WindowCompat
import androidx.core.view.WindowInsetsCompat
import com.getcapacitor.BridgeActivity
import com.getcapacitor.WebViewListener
import java.io.File

class MainActivity : BridgeActivity() {
    private var rendererGone = false
    private var resumed = false

    override fun onCreate(savedInstanceState: Bundle?) {
        AppCompatDelegate.setDefaultNightMode(AppCompatDelegate.MODE_NIGHT_NO)
        // 主题必须在 super.onCreate 之前确定；真实主题来自前端配置，原生只读缓存值。
        setTheme(launchThemeRes())
        registerPlugin(CfstPlugin::class.java)
        val webViewDebuggingEnabled = applicationInfo.flags and ApplicationInfo.FLAG_DEBUGGABLE != 0
        WebView.setWebContentsDebuggingEnabled(webViewDebuggingEnabled)
        super.onCreate(savedInstanceState)
        bridge?.addWebViewListener(
            object : WebViewListener() {
                override fun onRenderProcessGone(webView: WebView, detail: RenderProcessGoneDetail): Boolean {
                    // An unhandled renderer exit makes WebViewClient kill this app process, which would
                    // abort a running probe and mark its task snapshot as recovery_required.
                    Log.w(TAG, "WebView renderer gone (crashed=${detail.didCrash()}), scheduling WebView rebuild")
                    rendererGone = true
                    // Rebuild outside the WebView callback; the dead WebView is only replaced once visible.
                    webView.post { rebuildWebViewIfNeeded() }
                    return true
                }
            },
        )
        if (webViewDebuggingEnabled) {
            WebViewDevToolsRelay.start()
        }
        cleanupExportCache()
        applyAndroidWindowInsets()
        applyWebViewColorPolicy()
    }

    override fun onResume() {
        super.onResume()
        resumed = true
        rebuildWebViewIfNeeded()
    }

    override fun onPause() {
        resumed = false
        super.onPause()
    }

    override fun onWindowFocusChanged(hasFocus: Boolean) {
        super.onWindowFocusChanged(hasFocus)
        if (hasFocus) {
            applyAndroidWindowInsets()
        }
    }

    private fun rebuildWebViewIfNeeded() {
        if (!rendererGone || !resumed || isFinishing) {
            return
        }
        // The WebView whose renderer exited cannot be reused, so a new activity recreates the
        // Capacitor bridge and WebView while this process, and the Go core running the probe, stay alive.
        rendererGone = false
        recreate()
    }

    private fun applyAndroidWindowInsets() {
        WindowCompat.setDecorFitsSystemWindows(window, false)
        window.statusBarColor = Color.TRANSPARENT
        window.navigationBarColor = Color.TRANSPARENT
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.P) {
            window.attributes = window.attributes.apply {
                layoutInDisplayCutoutMode = WindowManager.LayoutParams.LAYOUT_IN_DISPLAY_CUTOUT_MODE_SHORT_EDGES
            }
        }
        val controller = WindowCompat.getInsetsController(window, window.decorView)
        controller.isAppearanceLightStatusBars = true
        controller.isAppearanceLightNavigationBars = true
        controller.show(WindowInsetsCompat.Type.systemBars())
    }

    @Suppress("DEPRECATION")
    private fun applyWebViewColorPolicy() {
        val webView = bridge?.webView ?: return
        // Match the app background (res/values/colors.xml) so the frames before
        // index.html paints are not Chromium's default white.
        refreshSurfaceColors()
        val webSettings = webView.settings
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
            webSettings.forceDark = WebSettings.FORCE_DARK_OFF
        }
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
            webSettings.isAlgorithmicDarkeningAllowed = false
        }
    }

    /** 启动窗口主题：与缓存主题一致，避免深色主题下冷启动/relaunch 先闪一帧白色窗口。 */
    private fun launchThemeRes(): Int {
        return if (AndroidSurfaceTheme.dark(this)) {
            R.style.AppTheme_NoActionBarLaunch_Dark
        } else {
            R.style.AppTheme_NoActionBarLaunch
        }
    }

    /** 刷新窗口与 WebView 底色；启动流程和前端主题切换都会调用。 */
    fun refreshSurfaceColors() {
        val colorRes = if (AndroidSurfaceTheme.dark(this)) R.color.app_background_dark else R.color.app_background
        val background = ContextCompat.getColor(this, colorRes)
        window.setBackgroundDrawable(ColorDrawable(background))
        bridge?.webView?.setBackgroundColor(background)
    }

    private fun cleanupExportCache() {
        val exportDir = File(cacheDir, "export")
        exportDir.listFiles()?.forEach { it.delete() }
    }

    companion object {
        private const val TAG = "MainActivity"
    }
}
