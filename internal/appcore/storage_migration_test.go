package appcore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRunStorageMigrationMovesLogsAndCopiesLegacyFiles(t *testing.T) {
	root := t.TempDir()
	layout := NewStorageLayout(root)
	mustWrite := func(path, body string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	mustWrite(layout.LegacyPath(FileDebugLog), "debug\n")
	mustWrite(layout.LegacyPath(FileErrorLog), "error\n")
	mustWrite(layout.LegacyPath(FileBootstrap), `{"storage_dir":"legacy-root","schema_version":"cfst-gui-storage-v1"}`)
	mustWrite(layout.LegacyPath(FileConfig), "config\n")
	if err := os.MkdirAll(layout.LegacyPath(DirExports), 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(filepath.Join(layout.LegacyPath(DirExports), "result.csv"), "id,value\n")

	now := time.Date(2026, time.August, 25, 12, 0, 0, 0, time.UTC)
	state, err := RunStorageMigration(layout, now)
	if err != nil {
		t.Fatal(err)
	}
	if state.MigrationSteps["logs_migrated"] != true || state.MigrationSteps["error_logs_migrated"] != true {
		t.Fatalf("startup migration steps = %#v", state.MigrationSteps)
	}
	if _, err := os.Stat(layout.LegacyPath(FileDebugLog)); !os.IsNotExist(err) {
		t.Fatalf("legacy debug log still exists, err=%v", err)
	}
	if body, err := os.ReadFile(layout.DebugLogPath()); err != nil || string(body) != "debug\n" {
		t.Fatalf("migrated debug log = %q, err=%v", body, err)
	}
	deadline := time.Now().Add(time.Second)
	for {
		if _, statErr := os.Stat(layout.ConfigPath()); statErr == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("background config migration did not complete")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if body, err := os.ReadFile(layout.ConfigPath()); err != nil || string(body) != "config\n" {
		t.Fatalf("migrated config = %q, err=%v", body, err)
	}
	bootstrap, err := os.ReadFile(layout.LegacyPath(FileBootstrap))
	if err != nil || !strings.Contains(string(bootstrap), `"storage_dir": "legacy-root"`) {
		t.Fatalf("bootstrap compatibility fields lost: %q, err=%v", bootstrap, err)
	}
	if _, err := os.Stat(filepath.Join(layout.ExportsRoot(), "result.csv")); err != nil {
		t.Fatalf("migrated export missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(layout.LegacyPath(DirExports), "result.csv")); err != nil {
		t.Fatalf("legacy export should remain during rollback window: %v", err)
	}
}
