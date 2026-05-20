package io.github.axuitomo.cfstgui;

import static org.junit.Assert.assertEquals;
import static org.junit.Assert.assertThrows;
import static org.junit.Assert.assertTrue;

import java.util.List;
import org.junit.Test;

public class AndroidDirectoryOpenIntentsTest {
    private static final String TREE_URI = "content://com.android.externalstorage.documents/tree/primary%3ADownload%2Fcf";
    private static final String DOCUMENT_URI = "content://com.android.externalstorage.documents/tree/primary%3ADownload%2Fcf/document/primary%3ADownload%2Fcf";

    @Test
    public void systemStorageManagerSpecUsesTreePickerWithInitialUri() {
        AndroidDirectoryOpenIntents.IntentSpec spec = AndroidDirectoryOpenIntents.openDirectoryIntentSpecs(TREE_URI).get(0);

        assertEquals(AndroidDirectoryOpenIntents.ACTION_OPEN_DOCUMENT_TREE, spec.action);
        assertEquals(TREE_URI, spec.initialUri);
        assertTrue(hasFlags(spec.flags, AndroidDirectoryOpenIntents.TREE_OPEN_FLAGS));
    }

    @Test
    public void directoryViewSpecTargetsTreeScopedDirectoryDocumentUri() {
        AndroidDirectoryOpenIntents.IntentSpec spec = AndroidDirectoryOpenIntents.openDirectoryIntentSpecs(TREE_URI).get(1);

        assertEquals(AndroidDirectoryOpenIntents.ACTION_VIEW, spec.action);
        assertEquals(DOCUMENT_URI, spec.dataUri);
        assertEquals(AndroidDirectoryOpenIntents.MIME_TYPE_DIRECTORY, spec.mimeType);
        assertTrue(hasFlags(spec.flags, AndroidDirectoryOpenIntents.DIRECTORY_VIEW_FLAGS));
    }

    @Test
    public void directoryOpenFallbackOrderUsesSystemThenViewThenChooser() {
        List<AndroidDirectoryOpenIntents.IntentSpec> specs = AndroidDirectoryOpenIntents.openDirectoryIntentSpecs(TREE_URI);

        assertEquals(3, specs.size());
        assertEquals(AndroidDirectoryOpenIntents.ACTION_OPEN_DOCUMENT_TREE, specs.get(0).action);
        assertEquals(AndroidDirectoryOpenIntents.ACTION_VIEW, specs.get(1).action);
        assertEquals(AndroidDirectoryOpenIntents.ACTION_CHOOSER, specs.get(2).action);

        AndroidDirectoryOpenIntents.IntentSpec chooserTarget = specs.get(2).chooserTarget;
        assertEquals(AndroidDirectoryOpenIntents.ACTION_VIEW, chooserTarget.action);
        assertEquals(DOCUMENT_URI, chooserTarget.dataUri);
    }

    @Test
    public void permissionLossUsesExplicitStorageDirectoryMessage() {
        IllegalStateException error = assertThrows(
            IllegalStateException.class,
            () -> CfstPlugin.requireStorageTreeUriPermission(false)
        );

        assertEquals(CfstPlugin.STORAGE_DIRECTORY_PERMISSION_LOST_MESSAGE, error.getMessage());
    }

    private static boolean hasFlags(int actualFlags, int expectedFlags) {
        return (actualFlags & expectedFlags) == expectedFlags;
    }
}
