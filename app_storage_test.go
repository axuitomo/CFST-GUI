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

func TestLoadDesktopConfigSanitizesLegacySnapshotWithoutWriting(t *testing.T) {
	root := isolateStorageForTest(t)
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	legacy := []byte(`{
  "config_snapshot": {
    "cloudflare": {
      "apiToken": "secret-token-value",
      "recordName": "legacy.example.com",
      "unknown_cloudflare": true
    },
    "probe": {
      "strategy": "full",
      "routines": 321,
      "retryMaxAttempts": 4,
      "cooldownMs": 555,
      "maxDelayMS": 1234,
      "ipText": "203.0.113.10",
      "unknown_probe": true
    },
    "backup": {
      "webdav": {
        "url": "https://dav.example.com/root",
        "remotePath": "legacy.zip",
        "unknown_webdav": true
      }
    },
    "scheduler": {
      "dailyTimes": "01:00; 02:00",
      "unknown_scheduler": true
    },
    "unknown_root": true
  }
}`)
	if err := os.WriteFile(desktopConfigFilePath(), legacy, 0o600); err != nil {
		t.Fatal(err)
	}

	app := NewApp()
	result := app.LoadDesktopConfig()
	if !result.OK {
		t.Fatalf("LoadDesktopConfig failed: %#v", result)
	}
	afterLoad, err := os.ReadFile(desktopConfigFilePath())
	if err != nil {
		t.Fatal(err)
	}
	if string(afterLoad) != string(legacy) {
		t.Fatalf("LoadDesktopConfig rewrote config file, want read-only compatibility")
	}

	snapshot := mapValue(mapValue(result.Data)["config_snapshot"])
	if _, exists := snapshot["unknown_root"]; exists {
		t.Fatalf("unknown_root was preserved in snapshot: %#v", snapshot)
	}
	cloudflare := mapValue(snapshot["cloudflare"])
	if got := stringValue(cloudflare["api_token"], ""); got != "secret-token-value" {
		t.Fatalf("api_token = %q, want legacy token", got)
	}
	if _, exists := cloudflare["apiToken"]; exists {
		t.Fatalf("apiToken alias was preserved: %#v", cloudflare)
	}
	webdav := mapValue(mapValue(snapshot["backup"])["webdav"])
	if got := stringValue(webdav["server_url"], ""); got != "https://dav.example.com/root" {
		t.Fatalf("server_url = %q, want legacy url", got)
	}
	if _, exists := webdav["unknown_webdav"]; exists {
		t.Fatalf("unknown_webdav was preserved: %#v", webdav)
	}
	probe := mapValue(snapshot["probe"])
	if got := intValue(mapValue(probe["retry_policy"])["max_attempts"], 0); got != 4 {
		t.Fatalf("retry max_attempts = %d, want 4", got)
	}
	if got := intValue(mapValue(probe["cooldown_policy"])["cooldown_ms"], 0); got != 555 {
		t.Fatalf("cooldown_ms = %d, want 555", got)
	}
	if got := intValue(mapValue(probe["thresholds"])["max_tcp_latency_ms"], 0); got != 1234 {
		t.Fatalf("max_tcp_latency_ms = %d, want 1234", got)
	}
	if _, exists := probe["unknown_probe"]; exists {
		t.Fatalf("unknown_probe was preserved: %#v", probe)
	}
	sources := snapshot["sources"].([]map[string]any)
	if len(sources) != 1 || stringValue(sources[0]["content"], "") != "203.0.113.10" {
		t.Fatalf("sources = %#v, want migrated sourceText/ipText source", sources)
	}
}

func TestSaveDesktopConfigSanitizesLegacySnapshotOnDisk(t *testing.T) {
	isolateStorageForTest(t)
	app := NewApp()

	result := app.SaveDesktopConfig(map[string]any{
		"config_snapshot": map[string]any{
			"cloudflare": map[string]any{
				"apiToken": "secret-token-value",
				"obsolete": "drop-me",
			},
			"probe": map[string]any{
				"retryMaxAttempts": 5,
				"unknown_probe":    true,
			},
			"backup": map[string]any{
				"webdav": map[string]any{
					"url":             "https://dav.example.com/root",
					"timeoutSeconds":  45,
					"unknown_webdav":  true,
					"legacy_password": "drop-me",
				},
			},
			"unknown_root": true,
		},
	})
	if !result.OK {
		t.Fatalf("SaveDesktopConfig failed: %#v", result)
	}

	raw, err := os.ReadFile(desktopConfigFilePath())
	if err != nil {
		t.Fatal(err)
	}
	var saved map[string]any
	if err := json.Unmarshal(raw, &saved); err != nil {
		t.Fatal(err)
	}
	snapshot := mapValue(saved["config_snapshot"])
	if _, exists := snapshot["unknown_root"]; exists {
		t.Fatalf("unknown_root was saved: %#v", snapshot)
	}
	cloudflare := mapValue(snapshot["cloudflare"])
	if got := stringValue(cloudflare["api_token"], ""); got != "secret-token-value" {
		t.Fatalf("api_token = %q, want secret token", got)
	}
	if _, exists := cloudflare["apiToken"]; exists {
		t.Fatalf("apiToken alias was saved: %#v", cloudflare)
	}
	probe := mapValue(snapshot["probe"])
	if got := intValue(mapValue(probe["retry_policy"])["max_attempts"], 0); got != 5 {
		t.Fatalf("retry max_attempts = %d, want 5", got)
	}
	if _, exists := probe["unknown_probe"]; exists {
		t.Fatalf("unknown_probe was saved: %#v", probe)
	}
	webdav := mapValue(mapValue(snapshot["backup"])["webdav"])
	if got := stringValue(webdav["server_url"], ""); got != "https://dav.example.com/root" {
		t.Fatalf("server_url = %q, want migrated url", got)
	}
	if got := intValue(webdav["timeout_seconds"], 0); got != 45 {
		t.Fatalf("timeout_seconds = %d, want 45", got)
	}
	if _, exists := webdav["unknown_webdav"]; exists {
		t.Fatalf("unknown_webdav was saved: %#v", webdav)
	}
}

func TestImportConfigArchiveSanitizesLegacySnapshot(t *testing.T) {
	isolateStorageForTest(t)
	app := NewApp()
	body := map[string]any{
		"config_snapshot": map[string]any{
			"cloudflare": map[string]any{
				"apiToken": "archive-token",
			},
			"probe": map[string]any{
				"retryMaxAttempts": 7,
				"unknown_probe":    true,
			},
			"unknown_root": true,
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
	if _, exists := savedSnapshot["unknown_root"]; exists {
		t.Fatalf("unknown_root was saved after import: %#v", savedSnapshot)
	}
	if got := stringValue(mapValue(savedSnapshot["cloudflare"])["api_token"], ""); got != "archive-token" {
		t.Fatalf("api_token = %q, want archive token", got)
	}
	if got := intValue(mapValue(mapValue(savedSnapshot["probe"])["retry_policy"])["max_attempts"], 0); got != 7 {
		t.Fatalf("retry max_attempts = %d, want 7", got)
	}
}

func TestLoadAndSwitchProfileSanitizesLegacySnapshots(t *testing.T) {
	root := isolateStorageForTest(t)
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	rawProfiles := []byte(`{
  "active_profile_id": "legacy-profile",
  "items": [
    {
      "id": "legacy-profile",
      "name": "Legacy Profile",
      "created_at": "2026-01-01T00:00:00Z",
      "updated_at": "2026-01-01T00:00:00Z",
      "config_snapshot": {
        "cloudflare": {
          "apiToken": "profile-token",
          "recordName": "profile.example.com",
          "unknown_cloudflare": true
        },
        "probe": {
          "retryMaxAttempts": 6,
          "unknown_probe": true
        },
        "unknown_root": true
      }
    }
  ],
  "schema_version": "legacy"
}`)
	if err := os.WriteFile(profilesPath(), rawProfiles, 0o600); err != nil {
		t.Fatal(err)
	}

	app := NewApp()
	loaded := app.LoadProfiles()
	if !loaded.OK {
		t.Fatalf("LoadProfiles failed: %#v", loaded)
	}
	store := loaded.Data.(profileStore)
	if len(store.Items) != 1 {
		t.Fatalf("profiles = %#v, want one item", store.Items)
	}
	snapshot := store.Items[0].ConfigSnapshot
	if _, exists := snapshot["unknown_root"]; exists {
		t.Fatalf("unknown_root was returned from profile: %#v", snapshot)
	}
	if got := stringValue(mapValue(snapshot["cloudflare"])["api_token"], ""); got != "profile-token" {
		t.Fatalf("profile api_token = %q, want profile token", got)
	}

	switched := app.SwitchProfile(map[string]any{"profile_id": "legacy-profile"})
	if !switched.OK {
		t.Fatalf("SwitchProfile failed: %#v", switched)
	}
	raw, err := os.ReadFile(desktopConfigFilePath())
	if err != nil {
		t.Fatal(err)
	}
	var saved map[string]any
	if err := json.Unmarshal(raw, &saved); err != nil {
		t.Fatal(err)
	}
	savedSnapshot := mapValue(saved["config_snapshot"])
	if _, exists := savedSnapshot["unknown_root"]; exists {
		t.Fatalf("unknown_root was saved after switch: %#v", savedSnapshot)
	}
	if _, exists := mapValue(savedSnapshot["cloudflare"])["apiToken"]; exists {
		t.Fatalf("apiToken alias was saved after switch: %#v", savedSnapshot)
	}
	if got := intValue(mapValue(mapValue(savedSnapshot["probe"])["retry_policy"])["max_attempts"], 0); got != 6 {
		t.Fatalf("profile retry max_attempts = %d, want 6", got)
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
