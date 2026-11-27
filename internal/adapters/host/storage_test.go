package host

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func isolateStorageForTest(t *testing.T) string {
	t.Helper()
	// 后台预热 goroutine 可能在测试结束后仍在写，testing 的 TempDir RemoveAll
	// 偶尔会撞上「目录非空」；这里容忍清理失败，别把竞态记成测试失败。
	root, err := os.MkdirTemp("", "cfst-app-test-")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	setConfigHomeForTest(t, root)
	t.Setenv("CFST_GUI_PORTABLE_ROOT", "")
	return filepath.Join(root, "CFST-GUI")
}

func setConfigHomeForTest(t *testing.T, root string) {
	t.Helper()
	// WebView2 profile 走本机缓存目录（见 desktop/gui.go），不跟着配置目录走；
	// 不隔离它，测试会写进开发机真实的 %LOCALAPPDATA%。
	ApplyTestConfigHome(t.Setenv, root)
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
