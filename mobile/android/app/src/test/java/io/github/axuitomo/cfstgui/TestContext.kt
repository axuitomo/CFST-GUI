package io.github.axuitomo.cfstgui

import android.content.ContextWrapper

class TestContext(private val packageNameValue: String) : ContextWrapper(null) {
    override fun getPackageName(): String {
        return packageNameValue
    }
}
