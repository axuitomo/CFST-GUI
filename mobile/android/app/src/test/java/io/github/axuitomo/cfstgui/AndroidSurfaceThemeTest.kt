package io.github.axuitomo.cfstgui

import android.content.Context
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.RuntimeEnvironment
import org.robolectric.annotation.Config

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [35])
class AndroidSurfaceThemeTest {
    @Before
    fun resetPreferences() {
        RuntimeEnvironment.getApplication()
            .getSharedPreferences("cfst_android_surface_theme", Context.MODE_PRIVATE)
            .edit()
            .clear()
            .commit()
    }

    @Test
    fun themeDefaultsToLightAndPersistsTheLastKnownValue() {
        val context = RuntimeEnvironment.getApplication()

        assertFalse(AndroidSurfaceTheme.dark(context))

        AndroidSurfaceTheme.setDark(context, true)
        assertTrue(AndroidSurfaceTheme.dark(context))

        AndroidSurfaceTheme.setDark(context, false)
        assertFalse(AndroidSurfaceTheme.dark(context))
    }
}
