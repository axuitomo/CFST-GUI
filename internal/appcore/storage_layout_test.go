package appcore

import (
	"path/filepath"
	"testing"
)

func TestNewStorageLayoutUsesLayeredPaths(t *testing.T) {
	root := t.TempDir()
	layout := NewStorageLayout(root)
	checks := map[string]string{
		"config":   filepath.Join(root, "config", "config.json"),
		"logs":     filepath.Join(root, "logs", "cfip-log.txt"),
		"errors":   filepath.Join(root, "logs", "error-log.txt"),
		"database": filepath.Join(root, "hot.db"),
		"task":     filepath.Join(root, "task-data", "task-1.json"),
	}
	got := map[string]string{
		"config": layout.ConfigPath(), "logs": layout.DebugLogPath(), "errors": layout.ErrorLogPath(),
		"database": layout.HotDB(), "task": layout.TaskDataPath("task-1"),
	}
	for name, want := range checks {
		if got[name] != want {
			t.Errorf("%s path = %q, want %q", name, got[name], want)
		}
	}
}

func TestLegacyStorageLayoutRemainsCompatible(t *testing.T) {
	root := t.TempDir()
	layout := StorageLayout{Root: root, ConfigFileName: "desktop-config.json"}
	if got, want := layout.ConfigPath(), filepath.Join(root, "desktop-config.json"); got != want {
		t.Fatalf("config path = %q, want %q", got, want)
	}
	if got, want := layout.LegacyPath("config.json"), filepath.Join(root, "config.json"); got != want {
		t.Fatalf("legacy path = %q, want %q", got, want)
	}
}
