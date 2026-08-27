package app

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
)

func isolateStorageForTest(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	setConfigHomeForTest(t, root)
	t.Setenv("CFST_GUI_PORTABLE_ROOT", "")
	return filepath.Join(root, "CFST-GUI")
}

func setConfigHomeForTest(t *testing.T, root string) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", root)
	if runtime.GOOS == "windows" {
		t.Setenv("APPDATA", root)
	}
}

func TestStorageRootDefaultsAndCanBeMarkedSetupComplete(t *testing.T) {
	defaultRoot := isolateStorageForTest(t)

	status := resolveStorageState()
	if status.CurrentDir != defaultRoot {
		t.Fatalf("CurrentDir = %q, want %q", status.CurrentDir, defaultRoot)
	}
	if status.SetupRequired {
		t.Fatal("SetupRequired = true, want false")
	}

	updated, _, err := setStorageDirectory(map[string]any{"use_default": true})
	if err != nil {
		t.Fatalf("setStorageDirectory: %v", err)
	}
	if updated.SetupRequired {
		t.Fatal("SetupRequired = true after default confirmation")
	}
	if _, err := os.Stat(filepath.Join(defaultRoot, storageBootstrapFileName)); err != nil {
		t.Fatalf("storage bootstrap not written: %v", err)
	}
}

func TestLegacyStorageDirectoryMigratesKnownFilesWithoutDeletingOldRoot(t *testing.T) {
	defaultRoot := isolateStorageForTest(t)
	oldRoot := filepath.Join(t.TempDir(), "legacy-storage")
	if err := os.MkdirAll(filepath.Join(oldRoot, "exports"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"desktop-config.json", "config.json", "result.csv", sourceProfilesFileName, filepath.Join("exports", "old.csv")} {
		path := filepath.Join(oldRoot, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(name), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	if err := os.MkdirAll(defaultRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	bootstrap := `{"schema_version":"cfst-gui-storage-v1","setup_completed":true,"storage_dir":` + strconv.Quote(oldRoot) + `}`
	if err := os.WriteFile(storageBootstrapPath(), []byte(bootstrap), 0o600); err != nil {
		t.Fatal(err)
	}

	status := resolveStorageState()
	if status.CurrentDir != defaultRoot {
		t.Fatalf("CurrentDir = %q, want %q", status.CurrentDir, defaultRoot)
	}
	if status.LegacyStorageDir != oldRoot || !status.LegacyStorageMigrationAttempted || !status.LegacyStorageMigrationCompleted {
		t.Fatalf("legacy migration status = %#v", status)
	}
	for _, name := range []string{"desktop-config.json", sourceProfilesFileName} {
		if _, err := os.Stat(filepath.Join(defaultRoot, name)); err != nil {
			t.Fatalf("migrated file %q missing: %v", name, err)
		}
		if _, err := os.Stat(filepath.Join(oldRoot, name)); err != nil {
			t.Fatalf("legacy file %q should remain: %v", name, err)
		}
	}
}
