package io.github.axuitomo.cfstgui;

import static org.junit.Assert.assertEquals;
import static org.junit.Assert.assertFalse;
import static org.junit.Assert.assertTrue;

import org.junit.Test;

public class AndroidStorageBridgeTest {

    @Test
    public void detectsSafTreeUriForPersistentExportDirectory() {
        assertTrue(AndroidStorageBridge.isTreeURIString("content://com.android.externalstorage.documents/tree/primary%3ADownload%2Fcf"));
        assertFalse(AndroidStorageBridge.isTreeURIString("content://com.android.externalstorage.documents/document/primary%3ADownload%2Fcf%2Fresult.csv"));
        assertFalse(AndroidStorageBridge.isTreeURIString("/sdcard/Download/cf/result.csv"));
    }

    @Test
    public void reportsPermissionLossForTreeUri() {
        assertEquals(
            "Android 未持有所选导出目录的持久化权限，请重新选择导出目录。",
            AndroidStorageBridge.persistentExportTargetError("content://com.android.externalstorage.documents/tree/primary%3ADownload%2Fcf")
        );
    }

    @Test
    public void reportsDocumentUriAsInvalidPersistentExportTarget() {
        assertEquals(
            "Android 导出目标不是 SAF 目录，请重新选择导出目录。",
            AndroidStorageBridge.persistentExportTargetError("content://com.android.externalstorage.documents/document/primary%3ADownload%2Fcf%2Fresult.csv")
        );
    }

    @Test
    public void sanitizesFileNameForTreeWrite() {
        assertEquals("result.csv", AndroidStorageBridge.safTargetFileName("../result.csv", ""));
        assertEquals("fallback.csv", AndroidStorageBridge.safTargetFileName("", "fallback.csv"));
        assertEquals("result.csv", AndroidStorageBridge.safTargetFileName("..", ""));
    }
}
