package io.github.axuitomo.cfstgui;

import android.content.Intent;
import android.net.Uri;
import android.provider.DocumentsContract;
import java.util.ArrayList;
import java.util.Collections;
import java.util.List;

final class AndroidDirectoryOpenIntents {
    static final String ACTION_OPEN_DOCUMENT_TREE = Intent.ACTION_OPEN_DOCUMENT_TREE;
    static final String ACTION_VIEW = Intent.ACTION_VIEW;
    static final String ACTION_CHOOSER = Intent.ACTION_CHOOSER;
    static final String CHOOSER_TITLE = "打开储存目录";
    static final String EXTRA_INITIAL_URI = DocumentsContract.EXTRA_INITIAL_URI;
    static final String EXTRA_INTENT = Intent.EXTRA_INTENT;
    static final String MIME_TYPE_DIRECTORY = DocumentsContract.Document.MIME_TYPE_DIR;
    static final int TREE_OPEN_FLAGS = Intent.FLAG_ACTIVITY_NEW_TASK
        | Intent.FLAG_GRANT_READ_URI_PERMISSION
        | Intent.FLAG_GRANT_WRITE_URI_PERMISSION
        | Intent.FLAG_GRANT_PERSISTABLE_URI_PERMISSION
        | Intent.FLAG_GRANT_PREFIX_URI_PERMISSION;
    static final int DIRECTORY_VIEW_FLAGS = Intent.FLAG_ACTIVITY_NEW_TASK
        | Intent.FLAG_GRANT_READ_URI_PERMISSION
        | Intent.FLAG_GRANT_WRITE_URI_PERMISSION
        | Intent.FLAG_GRANT_PREFIX_URI_PERMISSION;

    private AndroidDirectoryOpenIntents() {}

    static List<Intent> openDirectoryIntents(Uri treeUri) {
        List<Intent> intents = new ArrayList<>();
        for (IntentSpec spec : openDirectoryIntentSpecs(treeUri.toString())) {
            intents.add(spec.toIntent());
        }
        return Collections.unmodifiableList(intents);
    }

    static List<IntentSpec> openDirectoryIntentSpecs(String treeUri) {
        IntentSpec viewSpec = directoryViewIntentSpec(treeUri);
        List<IntentSpec> specs = new ArrayList<>();
        specs.add(systemStorageManagerIntentSpec(treeUri));
        specs.add(viewSpec);
        specs.add(directoryChooserIntentSpec(viewSpec));
        return Collections.unmodifiableList(specs);
    }

    static Intent systemStorageManagerIntent(Uri treeUri) {
        return systemStorageManagerIntentSpec(treeUri.toString()).toIntent();
    }

    static Intent directoryViewIntent(Uri treeUri) {
        return directoryViewIntentSpec(treeUri.toString()).toIntent();
    }

    static Intent directoryChooserIntent(Intent viewIntent) {
        Intent chooser = Intent.createChooser(viewIntent, CHOOSER_TITLE);
        chooser.addFlags(DIRECTORY_VIEW_FLAGS);
        return chooser;
    }

    static Uri directoryDocumentUri(Uri treeUri) {
        return Uri.parse(directoryDocumentUriString(treeUri.toString()));
    }

    private static IntentSpec systemStorageManagerIntentSpec(String treeUri) {
        return new IntentSpec(ACTION_OPEN_DOCUMENT_TREE, "", "", treeUri, TREE_OPEN_FLAGS, "", null);
    }

    private static IntentSpec directoryViewIntentSpec(String treeUri) {
        return new IntentSpec(ACTION_VIEW, directoryDocumentUriString(treeUri), MIME_TYPE_DIRECTORY, "", DIRECTORY_VIEW_FLAGS, "", null);
    }

    private static IntentSpec directoryChooserIntentSpec(IntentSpec viewSpec) {
        return new IntentSpec(ACTION_CHOOSER, "", "", "", DIRECTORY_VIEW_FLAGS, CHOOSER_TITLE, viewSpec);
    }

    private static String directoryDocumentUriString(String treeUri) {
        String normalized = treeUri == null ? "" : treeUri.trim();
        int schemeIndex = normalized.indexOf("://");
        int authorityEnd = schemeIndex < 0 ? -1 : normalized.indexOf('/', schemeIndex + 3);
        int treeIndex = normalized.indexOf("/tree/", authorityEnd);
        if (authorityEnd < 0 || treeIndex < 0) {
            return normalized;
        }
        String authorityPrefix = normalized.substring(0, authorityEnd);
        String treeDocumentId = normalized.substring(treeIndex + "/tree/".length());
        int queryIndex = firstSuffixIndex(treeDocumentId, '?', '#');
        if (queryIndex >= 0) {
            treeDocumentId = treeDocumentId.substring(0, queryIndex);
        }
        return authorityPrefix + "/tree/" + treeDocumentId + "/document/" + treeDocumentId;
    }

    private static int firstSuffixIndex(String value, char first, char second) {
        int firstIndex = value.indexOf(first);
        int secondIndex = value.indexOf(second);
        if (firstIndex < 0) {
            return secondIndex;
        }
        if (secondIndex < 0) {
            return firstIndex;
        }
        return Math.min(firstIndex, secondIndex);
    }

    static final class IntentSpec {
        final String action;
        final String dataUri;
        final String mimeType;
        final String initialUri;
        final int flags;
        final String chooserTitle;
        final IntentSpec chooserTarget;

        private IntentSpec(String action, String dataUri, String mimeType, String initialUri, int flags, String chooserTitle, IntentSpec chooserTarget) {
            this.action = action;
            this.dataUri = dataUri;
            this.mimeType = mimeType;
            this.initialUri = initialUri;
            this.flags = flags;
            this.chooserTitle = chooserTitle;
            this.chooserTarget = chooserTarget;
        }

        Intent toIntent() {
            if (ACTION_CHOOSER.equals(action)) {
                Intent chooser = Intent.createChooser(chooserTarget.toIntent(), chooserTitle);
                chooser.addFlags(flags);
                return chooser;
            }
            Intent intent = new Intent(action);
            if (!initialUri.isEmpty()) {
                intent.putExtra(EXTRA_INITIAL_URI, Uri.parse(initialUri));
            }
            if (!dataUri.isEmpty()) {
                intent.setDataAndType(Uri.parse(dataUri), mimeType);
            }
            intent.addFlags(flags);
            return intent;
        }
    }
}
