package main

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func isolateStorageForTest(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", root)
	t.Setenv("CFST_GUI_PORTABLE_ROOT", "")
	return filepath.Join(root, "CFST-GUI")
}

func TestStorageRootDefaultsAndCanBeMarkedSetupComplete(t *testing.T) {
	defaultRoot := isolateStorageForTest(t)

	status := resolveStorageState()
	if status.CurrentDir != defaultRoot {
		t.Fatalf("CurrentDir = %q, want %q", status.CurrentDir, defaultRoot)
	}
	if !status.SetupRequired {
		t.Fatal("SetupRequired = false, want true before bootstrap")
	}

	updated, _, err := setStorageDirectory(map[string]any{"use_default": true})
	if err != nil {
		t.Fatalf("setStorageDirectory default returned error: %v", err)
	}
	if updated.SetupRequired {
		t.Fatal("SetupRequired = true, want false after default confirmation")
	}
	if _, err := os.Stat(filepath.Join(defaultRoot, storageBootstrapFileName)); err != nil {
		t.Fatalf("storage bootstrap not written: %v", err)
	}
}

func TestSetStorageDirectoryCopiesKnownFilesWithoutDeletingOldRoot(t *testing.T) {
	oldRoot := isolateStorageForTest(t)
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

	newRoot := filepath.Join(t.TempDir(), "new-storage")
	status, migration, err := setStorageDirectory(map[string]any{
		"migrate":     true,
		"storage_dir": newRoot,
	})
	if err != nil {
		t.Fatalf("setStorageDirectory custom returned error: %v", err)
	}
	if status.CurrentDir != newRoot {
		t.Fatalf("CurrentDir = %q, want %q", status.CurrentDir, newRoot)
	}
	if len(migration.Copied) == 0 {
		t.Fatalf("migration.Copied is empty, want copied known files")
	}
	if _, err := os.Stat(filepath.Join(newRoot, "desktop-config.json")); err != nil {
		t.Fatalf("desktop config was not copied: %v", err)
	}
	if _, err := os.Stat(filepath.Join(newRoot, sourceProfilesFileName)); err != nil {
		t.Fatalf("source profiles were not copied: %v", err)
	}
	if _, err := os.Stat(filepath.Join(oldRoot, "desktop-config.json")); err != nil {
		t.Fatalf("old root should retain files: %v", err)
	}
	if _, err := os.Stat(filepath.Join(oldRoot, sourceProfilesFileName)); err != nil {
		t.Fatalf("old root should retain source profiles: %v", err)
	}
}

func TestExportConfigIncludesFullCloudflareToken(t *testing.T) {
	isolateStorageForTest(t)
	app := NewApp()
	targetPath := filepath.Join(t.TempDir(), "cfst-gui-config.json")
	snapshot := defaultDesktopConfigSnapshot()
	cloudflare := mapValue(snapshot["cloudflare"])
	cloudflare["api_token"] = "secret-token-value"

	result := app.ExportConfig(map[string]any{
		"config_snapshot": snapshot,
		"path":            targetPath,
	})
	if !result.OK {
		t.Fatalf("ExportConfig failed: %#v", result)
	}
	raw, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("read export: %v", err)
	}
	var exported map[string]any
	if err := json.Unmarshal(raw, &exported); err != nil {
		t.Fatalf("parse export: %v", err)
	}
	exportedSnapshot := mapValue(exported["config_snapshot"])
	exportedCloudflare := mapValue(exportedSnapshot["cloudflare"])
	if got := stringValue(exportedCloudflare["api_token"], ""); got != "secret-token-value" {
		t.Fatalf("api_token = %q, want full token", got)
	}
}

func TestImportConfigArchivePreservesSnapshotSourcesWithSourceProfiles(t *testing.T) {
	isolateStorageForTest(t)
	app := NewApp()
	currentSources := []DesktopSource{
		{
			Enabled: true,
			ID:      "source-current",
			IPMode:  "traverse",
			Kind:    "url",
			Name:    "Current Sources",
			URL:     "https://current.example/top10.txt",
		},
	}
	staleProfileSources := []DesktopSource{
		{
			Enabled: true,
			ID:      "source-stale",
			IPMode:  "traverse",
			Kind:    "url",
			Name:    "Stale Profile Sources",
			URL:     "https://stale.example/top10.txt",
		},
	}
	snapshot := defaultDesktopConfigSnapshot()
	snapshot["sources"] = currentSources
	body := map[string]any{
		"config_snapshot": snapshot,
		"source_profiles": sourceProfileStore{
			ActiveProfileID: "source-profile-stale",
			Items: []sourceProfileItem{
				{
					ID:      "source-profile-stale",
					Name:    "旧输入源档案",
					Sources: staleProfileSources,
				},
			},
			SchemaVersion: sourceProfilesSchemaVersion,
		},
	}
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	archive, err := zipSingleFile(configArchiveEntryName, raw)
	if err != nil {
		t.Fatal(err)
	}

	result := app.ImportConfigArchive(map[string]any{
		"content_base64":          base64.StdEncoding.EncodeToString(archive),
		"current_config_snapshot": defaultDesktopConfigSnapshot(),
	})
	if !result.OK {
		t.Fatalf("ImportConfigArchive failed: %#v", result)
	}
	savedSnapshot, err := loadDesktopConfigSnapshotFromDisk()
	if err != nil {
		t.Fatalf("load saved snapshot: %v", err)
	}
	savedSources := desktopSourcesFromAny(savedSnapshot["sources"])
	if len(savedSources) != 1 || savedSources[0].URL != "https://current.example/top10.txt" {
		t.Fatalf("saved sources = %#v, want current snapshot sources", savedSources)
	}
	store, err := loadSourceProfileStore()
	if err != nil {
		t.Fatalf("load source profiles: %v", err)
	}
	if store.ActiveProfileID != "source-profile-stale" || len(store.Items) != 1 {
		t.Fatalf("source profile store = %#v, want imported stale profile active", store)
	}
	if len(store.Items[0].Sources) != 1 || store.Items[0].Sources[0].URL != "https://stale.example/top10.txt" {
		t.Fatalf("source profile sources = %#v, want imported profile sources", store.Items[0].Sources)
	}
}

func TestImportConfigArchiveWithoutSourceProfilesCreatesDefaultFromSnapshotSources(t *testing.T) {
	isolateStorageForTest(t)
	app := NewApp()
	snapshot := defaultDesktopConfigSnapshot()
	snapshot["sources"] = []DesktopSource{
		{
			Enabled: true,
			ID:      "source-current",
			IPMode:  "traverse",
			Kind:    "url",
			Name:    "Current Sources",
			URL:     "https://current.example/top10.txt",
		},
	}
	raw, err := json.Marshal(map[string]any{"config_snapshot": snapshot})
	if err != nil {
		t.Fatal(err)
	}
	archive, err := zipSingleFile(configArchiveEntryName, raw)
	if err != nil {
		t.Fatal(err)
	}

	result := app.ImportConfigArchive(map[string]any{
		"content_base64":          base64.StdEncoding.EncodeToString(archive),
		"current_config_snapshot": defaultDesktopConfigSnapshot(),
	})
	if !result.OK {
		t.Fatalf("ImportConfigArchive failed: %#v", result)
	}
	store, err := loadSourceProfileStore()
	if err != nil {
		t.Fatalf("load source profiles: %v", err)
	}
	if store.ActiveProfileID != defaultSourceProfileID || len(store.Items) != 1 {
		t.Fatalf("source profile store = %#v, want generated default profile", store)
	}
	if len(store.Items[0].Sources) != 1 || store.Items[0].Sources[0].URL != "https://current.example/top10.txt" {
		t.Fatalf("default source profile sources = %#v, want snapshot sources", store.Items[0].Sources)
	}
}

func TestProfilesSaveAndSwitchWritesDesktopConfig(t *testing.T) {
	isolateStorageForTest(t)
	app := NewApp()
	snapshot := defaultDesktopConfigSnapshot()
	mapValue(snapshot["cloudflare"])["record_name"] = "one.example.com"

	save := app.SaveCurrentProfile(map[string]any{
		"config_snapshot": snapshot,
		"name":            "Profile One",
	})
	if !save.OK {
		t.Fatalf("SaveCurrentProfile failed: %#v", save)
	}
	store := save.Data.(profileStore)
	if len(store.Items) != 1 {
		t.Fatalf("profiles = %#v, want one item", store.Items)
	}

	switched := app.SwitchProfile(map[string]any{"profile_id": store.Items[0].ID})
	if !switched.OK {
		t.Fatalf("SwitchProfile failed: %#v", switched)
	}
	raw, err := os.ReadFile(desktopConfigFilePath())
	if err != nil {
		t.Fatalf("read desktop config: %v", err)
	}
	if !strings.Contains(string(raw), "one.example.com") {
		t.Fatalf("desktop config did not contain switched profile snapshot: %s", raw)
	}
}

func TestRenderExportFileTemplateSanitizesPathCharacters(t *testing.T) {
	got := renderExportFileTemplate("result-{date}-{time}-{task_id}-{profile}.csv", "task/1", "A:B", time.Date(2026, 5, 2, 3, 4, 5, 0, time.UTC))
	want := "result-20260502-030405-task_1-A_B.csv"
	if got != want {
		t.Fatalf("rendered template = %q, want %q", got, want)
	}
}

func TestColoDictionaryBridgeDoesNotSetFixedTimeouts(t *testing.T) {
	for _, path := range []string{"desktop_colo_dictionary.go", filepath.Join("mobileapi", "colo_dictionary.go")} {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		source := string(raw)
		if strings.Contains(source, "WithTimeout") || strings.Contains(source, "Timeout:") {
			t.Fatalf("%s still configures a fixed COLO update timeout", path)
		}
	}
}
