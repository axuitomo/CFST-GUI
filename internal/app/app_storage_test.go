package app

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/probecore"
)

func isolateStorageForTest(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", root)
	t.Setenv("CFST_GUI_PORTABLE_ROOT", "")
	return filepath.Join(root, "CFST-GUI")
}

func rewriteSavedAtForTest(t *testing.T, path string, savedAt time.Time) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config file %q failed: %v", path, err)
	}
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("unmarshal config file %q failed: %v", path, err)
	}
	body["saved_at"] = savedAt.Format(time.RFC3339)
	encoded, err := json.MarshalIndent(body, "", "  ")
	if err != nil {
		t.Fatalf("marshal config file %q failed: %v", path, err)
	}
	if err := os.WriteFile(path, encoded, 0o600); err != nil {
		t.Fatalf("write config file %q failed: %v", path, err)
	}
}

func TestStorageRootDefaultsAndCanBeMarkedSetupComplete(t *testing.T) {
	defaultRoot := isolateStorageForTest(t)

	status := resolveStorageState()
	if status.CurrentDir != defaultRoot {
		t.Fatalf("CurrentDir = %q, want %q", status.CurrentDir, defaultRoot)
	}
	if status.SetupRequired {
		t.Fatal("SetupRequired = true, want false because storage setup is no longer user-configurable")
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
	if err := os.WriteFile(storageBootstrapPath(), []byte(`{"schema_version":"cfst-gui-storage-v1","setup_completed":true,"storage_dir":`+strconv.Quote(oldRoot)+`}`), 0o600); err != nil {
		t.Fatal(err)
	}

	status := resolveStorageState()
	if status.CurrentDir != defaultRoot {
		t.Fatalf("CurrentDir = %q, want fixed default %q", status.CurrentDir, defaultRoot)
	}
	if status.LegacyStorageDir != oldRoot || !status.LegacyStorageMigrationAttempted || !status.LegacyStorageMigrationCompleted {
		t.Fatalf("legacy migration status = %#v, want completed migration from %q", status, oldRoot)
	}
	if _, err := os.Stat(filepath.Join(defaultRoot, "desktop-config.json")); err != nil {
		t.Fatalf("desktop config was not copied: %v", err)
	}
	if _, err := os.Stat(filepath.Join(defaultRoot, sourceProfilesFileName)); err != nil {
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

func TestExportDebugLogWritesConfiguredExportDirectory(t *testing.T) {
	root := isolateStorageForTest(t)
	app := NewApp()
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(debugLogFilePath()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(debugLogFilePath(), []byte("debug line\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	targetDir := filepath.Join(t.TempDir(), "exports")

	result := app.ExportDebugLog(map[string]any{
		"config": map[string]any{
			"export": map[string]any{
				"target_dir": targetDir,
			},
		},
		"file_name": "debug.txt",
	})
	if !result.OK || result.Code != "DEBUG_LOG_EXPORT_OK" {
		t.Fatalf("ExportDebugLog failed: %#v", result)
	}
	data := mapValue(result.Data)
	targetPath := filepath.Join(targetDir, "debug.txt")
	if got := stringValue(data["path"], ""); got != targetPath {
		t.Fatalf("path = %q, want %q", got, targetPath)
	}
	if got := stringValue(firstNonNil(data["log_dir"], data["logDir"]), ""); got != logDirectoryPath() {
		t.Fatalf("log_dir = %q, want %q", got, logDirectoryPath())
	}
	raw, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("read exported log: %v", err)
	}
	if string(raw) != "debug line\n" {
		t.Fatalf("exported log = %q", string(raw))
	}
}

func TestExportDiagnosticBundleWritesConfiguredExportDirectory(t *testing.T) {
	root := isolateStorageForTest(t)
	app := NewApp()
	if err := os.MkdirAll(filepath.Join(root, "logs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(debugLogFilePath(), []byte("debug line\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(errorLogFilePath(), []byte("error line\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "logs", "app-2026-06-15.jsonl"), []byte("app line\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(desktopConfigFilePath(), []byte("secret config\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	targetDir := filepath.Join(t.TempDir(), "exports")

	result := app.ExportDiagnosticBundle(map[string]any{
		"config": map[string]any{
			"export": map[string]any{
				"target_dir": targetDir,
			},
		},
		"file_name": "diagnostics.zip",
	})
	if !result.OK || result.Code != "DIAGNOSTIC_BUNDLE_EXPORT_OK" {
		t.Fatalf("ExportDiagnosticBundle failed: %#v", result)
	}
	data := mapValue(result.Data)
	targetPath := filepath.Join(targetDir, "diagnostics.zip")
	if got := stringValue(data["path"], ""); got != targetPath {
		t.Fatalf("path = %q, want %q", got, targetPath)
	}
	entries := readDiagnosticZipEntriesForAppTest(t, targetPath)
	for _, name := range []string{"manifest.json", "logs/app-2026-06-15.jsonl", "logs/cfip-log.txt", "logs/error-log.txt"} {
		if _, ok := entries[name]; !ok {
			t.Fatalf("diagnostic bundle missing %s; entries=%v", name, entries)
		}
	}
	if _, ok := entries["logs/desktop-config.json"]; ok {
		t.Fatalf("diagnostic bundle included desktop config")
	}
}

func TestExportDiagnosticBundleEmpty(t *testing.T) {
	isolateStorageForTest(t)
	app := NewApp()

	result := app.ExportDiagnosticBundle(map[string]any{})
	if result.OK || result.Code != "DIAGNOSTIC_BUNDLE_EMPTY" {
		t.Fatalf("ExportDiagnosticBundle = %#v, want empty failure", result)
	}
}

func TestDesktopLogPathsUseLogDirectory(t *testing.T) {
	root := isolateStorageForTest(t)
	if got := debugLogFilePath(); got != filepath.Join(root, "logs", "cfip-log.txt") {
		t.Fatalf("debugLogFilePath = %q, want logs/cfip-log.txt under %q", got, root)
	}
	if got := errorLogFilePath(); got != filepath.Join(root, "logs", "error-log.txt") {
		t.Fatalf("errorLogFilePath = %q, want logs/error-log.txt under %q", got, root)
	}
	if got := runtimeLogFilePath(); !strings.HasPrefix(got, filepath.Join(root, "logs", "app-")) || !strings.HasSuffix(got, ".jsonl") {
		t.Fatalf("runtimeLogFilePath = %q, want daily app log under %q", got, filepath.Join(root, "logs"))
	}
}

func TestRecordFrontendRuntimeErrorWritesErrorLog(t *testing.T) {
	isolateStorageForTest(t)
	app := NewApp()

	empty := app.RecordFrontendRuntimeError(nil)
	if !empty.OK {
		t.Fatalf("RecordFrontendRuntimeError(nil) = %#v, want ok", empty)
	}
	result := app.RecordFrontendRuntimeError(map[string]any{
		"event":   "probe.completed",
		"message": "completion refresh failed",
		"source":  "probe-event-listener",
		"task_id": "frontend-task",
	})
	if !result.OK {
		t.Fatalf("RecordFrontendRuntimeError = %#v, want ok", result)
	}

	entries := readDebugLogEntries(t, errorLogFilePath())
	if len(entries) != 2 {
		t.Fatalf("error log entries = %d, want 2: %#v", len(entries), entries)
	}
	for _, entry := range entries {
		if got := stringValue(entry["event"], ""); got != "frontend.runtime_error" {
			t.Fatalf("event = %q, want frontend.runtime_error in %#v", got, entry)
		}
	}
	if got := stringValue(entries[0]["message"], ""); got != "前端运行时错误。" {
		t.Fatalf("default message = %q, want fallback", got)
	}
	if got := stringValue(entries[1]["message"], ""); got != "completion refresh failed" {
		t.Fatalf("message = %q, want completion refresh failed", got)
	}
	if got := stringValue(entries[1]["task_id"], ""); got != "frontend-task" {
		t.Fatalf("task_id = %q, want frontend-task", got)
	}

	runtimeEntries := readDebugLogEntries(t, runtimeLogFilePath())
	if len(runtimeEntries) != 2 {
		t.Fatalf("runtime log entries = %d, want 2: %#v", len(runtimeEntries), runtimeEntries)
	}
	if got := stringValue(runtimeEntries[1]["event"], ""); got != "frontend.runtime_error" {
		t.Fatalf("runtime event = %q, want frontend.runtime_error", got)
	}
	if got := stringValue(runtimeEntries[1]["level"], ""); got != "error" {
		t.Fatalf("runtime level = %q, want error", got)
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

func TestSaveDesktopConfigPreservesThemeAndPortPolicy(t *testing.T) {
	isolateStorageForTest(t)
	app := NewApp()

	snapshot := defaultDesktopConfigSnapshot()
	probe := mapValue(snapshot["probe"])
	probe["port_policy"] = probecore.PortPolicyFixedGlobal
	ui := mapValue(snapshot["ui"])
	ui["theme_mode"] = "auto_time"
	ui["theme_light_start"] = "06:30"
	ui["theme_dark_start"] = "20:45"
	ui["utc_offset_minutes"] = 330

	result := app.SaveDesktopConfig(map[string]any{
		"config_snapshot": snapshot,
	})
	if !result.OK {
		t.Fatalf("SaveDesktopConfig failed: %#v", result)
	}

	loaded := app.LoadDesktopConfig()
	if !loaded.OK {
		t.Fatalf("LoadDesktopConfig failed: %#v", loaded)
	}

	loadedSnapshot := mapValue(mapValue(loaded.Data)["config_snapshot"])
	loadedProbe := mapValue(loadedSnapshot["probe"])
	if got := stringValue(loadedProbe["port_policy"], ""); got != probecore.PortPolicyFixedGlobal {
		t.Fatalf("port_policy = %q, want %q", got, probecore.PortPolicyFixedGlobal)
	}
	loadedUI := mapValue(loadedSnapshot["ui"])
	if got := stringValue(loadedUI["theme_mode"], ""); got != "auto_time" {
		t.Fatalf("theme_mode = %q, want auto_time", got)
	}
	if got := stringValue(loadedUI["theme_light_start"], ""); got != "06:30" {
		t.Fatalf("theme_light_start = %q, want 06:30", got)
	}
	if got := stringValue(loadedUI["theme_dark_start"], ""); got != "20:45" {
		t.Fatalf("theme_dark_start = %q, want 20:45", got)
	}
	if got := intValue(loadedUI["utc_offset_minutes"], 0); got != 330 {
		t.Fatalf("utc_offset_minutes = %d, want 330", got)
	}
}

func TestDesktopDraftRecoverStatusAndClearsAfterSave(t *testing.T) {
	isolateStorageForTest(t)
	app := NewApp()
	savedAt := time.Date(2026, 5, 9, 10, 0, 0, 0, time.FixedZone("test", 8*60*60))
	savedSnapshot := defaultDesktopConfigSnapshot()
	mapValue(savedSnapshot["cloudflare"])["record_name"] = "saved.example.com"
	if result := app.SaveDesktopConfig(map[string]any{"config_snapshot": savedSnapshot}); !result.OK {
		t.Fatalf("SaveDesktopConfig failed: %#v", result)
	}
	rewriteSavedAtForTest(t, desktopConfigFilePath(), savedAt)

	draftSnapshot := defaultDesktopConfigSnapshot()
	mapValue(draftSnapshot["cloudflare"])["record_name"] = "draft.example.com"
	draft := app.SaveDesktopDraft(map[string]any{"config_snapshot": draftSnapshot})
	if !draft.OK {
		t.Fatalf("SaveDesktopDraft failed: %#v", draft)
	}
	rewriteSavedAtForTest(t, desktopDraftFilePath(), savedAt.Add(time.Second))
	statusResult := app.LoadDesktopDraft()
	if !statusResult.OK {
		t.Fatalf("LoadDesktopDraft failed: %#v", statusResult)
	}
	status := mapValue(statusResult.Data)
	if !boolValue(status["exists"], false) || !boolValue(status["is_newer_than_saved"], false) {
		t.Fatalf("draft status = %#v, want newer draft", status)
	}

	loaded := app.LoadDesktopConfig()
	if !loaded.OK {
		t.Fatalf("LoadDesktopConfig failed: %#v", loaded)
	}
	loadedDraft := mapValue(mapValue(loaded.Data)["draft_status"])
	if !boolValue(loadedDraft["is_newer_than_saved"], false) {
		t.Fatalf("loaded draft status = %#v, want recoverable draft", loadedDraft)
	}

	if result := app.SaveDesktopConfig(map[string]any{"config_snapshot": draftSnapshot}); !result.OK {
		t.Fatalf("SaveDesktopConfig after draft failed: %#v", result)
	}
	afterSave := app.LoadDesktopDraft()
	if !afterSave.OK {
		t.Fatalf("LoadDesktopDraft failed: %#v", afterSave)
	}
	if boolValue(mapValue(afterSave.Data)["exists"], false) {
		t.Fatalf("draft still exists after formal save: %#v", afterSave.Data)
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

func TestImportConfigArchivePreservesLocalExportTarget(t *testing.T) {
	isolateStorageForTest(t)
	app := NewApp()
	current := defaultDesktopConfigSnapshot()
	currentExport := mapValue(current["export"])
	currentExport["target_dir"] = `C:\CFST\exports`
	currentExport["target_uri"] = ""
	current["export"] = currentExport

	incoming := defaultDesktopConfigSnapshot()
	incomingExport := mapValue(incoming["export"])
	incomingExport["file_name"] = "restored.csv"
	incomingExport["target_dir"] = ""
	incomingExport["target_uri"] = "content://android/tree/exports"
	incoming["export"] = incomingExport
	raw, err := json.Marshal(map[string]any{"config_snapshot": incoming})
	if err != nil {
		t.Fatal(err)
	}
	archive, err := zipSingleFile(configArchiveEntryName, raw)
	if err != nil {
		t.Fatal(err)
	}

	result := app.ImportConfigArchive(map[string]any{
		"content_base64":          base64.StdEncoding.EncodeToString(archive),
		"current_config_snapshot": current,
	})
	if !result.OK {
		t.Fatalf("ImportConfigArchive failed: %#v", result)
	}
	savedSnapshot, err := loadDesktopConfigSnapshotFromDisk()
	if err != nil {
		t.Fatalf("load saved snapshot: %v", err)
	}
	exportCfg := mapValue(savedSnapshot["export"])
	if got := stringValue(exportCfg["file_name"], ""); got != "restored.csv" {
		t.Fatalf("file_name = %q, want restored.csv", got)
	}
	if got := stringValue(exportCfg["target_dir"], ""); got != `C:\CFST\exports` {
		t.Fatalf("target_dir = %q, want local export target", got)
	}
	if got := stringValue(exportCfg["target_uri"], ""); got != "" {
		t.Fatalf("target_uri = %q, want local empty URI", got)
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

func TestImportConfigArchiveRollsBackWhenSourceProfileSaveFails(t *testing.T) {
	isolateStorageForTest(t)
	app := NewApp()
	oldSnapshot := defaultDesktopConfigSnapshot()
	oldSnapshot["cloudflare"] = map[string]any{"api_token": "old-token"}
	if err := writeDesktopConfigSnapshot(desktopConfigFilePath(), oldSnapshot); err != nil {
		t.Fatalf("writeDesktopConfigSnapshot: %v", err)
	}
	oldSourceProfiles := sourceProfileStore{
		ActiveProfileID: "source-profile-old",
		Items: []sourceProfileItem{
			{
				ID:      "source-profile-old",
				Name:    "旧输入源档案",
				Sources: []DesktopSource{{ID: "source-old", Kind: "url", URL: "https://old.example/top10.txt"}},
			},
		},
		SchemaVersion: sourceProfilesSchemaVersion,
	}
	if err := saveSourceProfileStore(oldSourceProfiles); err != nil {
		t.Fatalf("saveSourceProfileStore: %v", err)
	}
	raw, err := json.Marshal(map[string]any{
		"config_snapshot": map[string]any{
			"cloudflare": map[string]any{"api_token": "new-token"},
		},
		"source_profiles": sourceProfileStore{
			ActiveProfileID: "source-profile-new",
			Items: []sourceProfileItem{
				{
					ID:      "source-profile-new",
					Name:    "新输入源档案",
					Sources: []DesktopSource{{ID: "source-new", Kind: "url", URL: "https://new.example/top10.txt"}},
				},
			},
			SchemaVersion: sourceProfilesSchemaVersion,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	archive, err := zipSingleFile(configArchiveEntryName, raw)
	if err != nil {
		t.Fatal(err)
	}
	originalHook := saveDesktopSourceProfileStoreForImport
	saveDesktopSourceProfileStoreForImport = func(store sourceProfileStore) error {
		return errors.New("inject source profile save failure")
	}
	t.Cleanup(func() {
		saveDesktopSourceProfileStoreForImport = originalHook
	})

	result := app.ImportConfigArchive(map[string]any{
		"content_base64":          base64.StdEncoding.EncodeToString(archive),
		"current_config_snapshot": oldSnapshot,
	})
	if result.OK {
		t.Fatalf("ImportConfigArchive unexpectedly succeeded: %#v", result)
	}
	savedSnapshot, err := loadDesktopConfigSnapshotFromDisk()
	if err != nil {
		t.Fatalf("loadDesktopConfigSnapshotFromDisk: %v", err)
	}
	if got := stringValue(mapValue(savedSnapshot["cloudflare"])["api_token"], ""); got != "old-token" {
		t.Fatalf("saved config api_token = %q, want old-token", got)
	}
	restoredSourceProfiles, err := loadSourceProfileStore()
	if err != nil {
		t.Fatalf("loadSourceProfileStore: %v", err)
	}
	if restoredSourceProfiles.ActiveProfileID != "source-profile-old" {
		t.Fatalf("restored source profiles active = %q, want source-profile-old", restoredSourceProfiles.ActiveProfileID)
	}
	if got := restoredSourceProfiles.Items[0].Sources[0].URL; got != "https://old.example/top10.txt" {
		t.Fatalf("restored source profile url = %q, want old url", got)
	}
}

func TestSaveSourceProfileAllowsBlankActiveAndWritesConfig(t *testing.T) {
	isolateStorageForTest(t)
	app := NewApp()

	result := app.SaveSourceProfile(map[string]any{
		"name":       "空白档案",
		"profile_id": "blank",
		"set_active": true,
		"sources":    []DesktopSource{},
	})
	if !result.OK {
		t.Fatalf("SaveSourceProfile failed: %#v", result)
	}
	store, ok := result.Data.(sourceProfileStore)
	if !ok {
		t.Fatalf("result data = %T, want sourceProfileStore", result.Data)
	}
	if store.ActiveProfileID != "blank" || len(store.Items) != 1 || len(store.Items[0].Sources) != 0 {
		t.Fatalf("source profile store = %#v, want blank active profile", store)
	}
	if sources := savedDesktopConfigSourcesForTest(t); len(sources) != 0 {
		t.Fatalf("saved config sources = %#v, want empty", sources)
	}
}

func TestDeleteSourceProfileSwitchesActiveSources(t *testing.T) {
	isolateStorageForTest(t)
	app := NewApp()
	firstSources := []DesktopSource{sourceProfileTestSource("source-one", "Source One")}
	secondSources := []DesktopSource{sourceProfileTestSource("source-two", "Source Two")}

	if result := app.SaveSourceProfile(map[string]any{"name": "one", "profile_id": "one", "set_active": true, "sources": firstSources}); !result.OK {
		t.Fatalf("save first source profile failed: %#v", result)
	}
	if result := app.SaveSourceProfile(map[string]any{"name": "two", "profile_id": "two", "set_active": true, "sources": secondSources}); !result.OK {
		t.Fatalf("save second source profile failed: %#v", result)
	}

	result := app.DeleteSourceProfile(map[string]any{"profile_id": "two"})
	if !result.OK {
		t.Fatalf("DeleteSourceProfile failed: %#v", result)
	}
	store, ok := result.Data.(sourceProfileStore)
	if !ok {
		t.Fatalf("result data = %T, want sourceProfileStore", result.Data)
	}
	if store.ActiveProfileID != "one" {
		t.Fatalf("active profile = %q, want one", store.ActiveProfileID)
	}
	sources := savedDesktopConfigSourcesForTest(t)
	if len(sources) != 1 || stringValue(mapValue(sources[0])["name"], "") != "Source One" {
		t.Fatalf("saved config sources = %#v, want source one", sources)
	}
}

func TestDeleteLastSourceProfileCreatesBlankDefault(t *testing.T) {
	isolateStorageForTest(t)
	app := NewApp()

	if result := app.SaveSourceProfile(map[string]any{
		"name":       "only",
		"profile_id": "only",
		"set_active": true,
		"sources":    []DesktopSource{sourceProfileTestSource("only-source", "Only Source")},
	}); !result.OK {
		t.Fatalf("save only source profile failed: %#v", result)
	}

	result := app.DeleteSourceProfile(map[string]any{"profile_id": "only"})
	if !result.OK {
		t.Fatalf("DeleteSourceProfile failed: %#v", result)
	}
	store, ok := result.Data.(sourceProfileStore)
	if !ok {
		t.Fatalf("result data = %T, want sourceProfileStore", result.Data)
	}
	if store.ActiveProfileID != defaultSourceProfileID || len(store.Items) != 1 || len(store.Items[0].Sources) != 0 {
		t.Fatalf("source profile store = %#v, want blank default profile", store)
	}
	if sources := savedDesktopConfigSourcesForTest(t); len(sources) != 0 {
		t.Fatalf("saved config sources = %#v, want empty", sources)
	}
}

func TestUpdateCurrentSourceProfileOverwritesActiveOrCreatesWhenMissing(t *testing.T) {
	isolateStorageForTest(t)
	app := NewApp()
	firstSources := []DesktopSource{sourceProfileTestSource("source-one", "Source One")}

	created := app.UpdateCurrentSourceProfile(map[string]any{"sources": firstSources, "name": "当前输入源"})
	if !created.OK {
		t.Fatalf("UpdateCurrentSourceProfile create failed: %#v", created)
	}
	store := mapValue(created.Data)["source_profiles"].(sourceProfileStore)
	if store.ActiveProfileID == "" || len(store.Items) != 1 || len(store.Items[0].Sources) != 1 {
		t.Fatalf("created source store = %#v, want one active profile", store)
	}
	profileID := store.ActiveProfileID

	secondSources := []DesktopSource{sourceProfileTestSource("source-two", "Source Two")}
	updated := app.UpdateCurrentSourceProfile(map[string]any{"sources": secondSources})
	if !updated.OK {
		t.Fatalf("UpdateCurrentSourceProfile update failed: %#v", updated)
	}
	store = mapValue(updated.Data)["source_profiles"].(sourceProfileStore)
	if store.ActiveProfileID != profileID || len(store.Items) != 1 {
		t.Fatalf("updated source store = %#v, want same active source profile", store)
	}
	if got := store.Items[0].Sources[0].Name; got != "Source Two" {
		t.Fatalf("source name = %q, want overwritten sources", got)
	}

	store.ActiveProfileID = "missing-source"
	if err := saveSourceProfileStore(store); err != nil {
		t.Fatal(err)
	}
	thirdSources := []DesktopSource{sourceProfileTestSource("source-three", "Source Three")}
	createdMissing := app.UpdateCurrentSourceProfile(map[string]any{"sources": thirdSources, "name": "缺失输入源补建"})
	if !createdMissing.OK {
		t.Fatalf("UpdateCurrentSourceProfile missing active failed: %#v", createdMissing)
	}
	store = mapValue(createdMissing.Data)["source_profiles"].(sourceProfileStore)
	if store.ActiveProfileID != "missing-source" || len(store.Items) != 2 {
		t.Fatalf("missing-active source store = %#v, want newly created active source profile", store)
	}
	savedSources := savedDesktopConfigSourcesForTest(t)
	if len(savedSources) != 1 || stringValue(mapValue(savedSources[0])["name"], "") != "Source Three" {
		t.Fatalf("saved config sources = %#v, want latest source profile sources", savedSources)
	}
}

func sourceProfileTestSource(id, name string) DesktopSource {
	return DesktopSource{
		ColoFilterMode: "allow",
		Content:        "1.1.1.1",
		Enabled:        true,
		ID:             id,
		IPLimit:        1,
		IPMode:         "traverse",
		Kind:           "inline",
		Name:           name,
	}
}

func readDiagnosticZipEntriesForAppTest(t *testing.T, path string) map[string][]byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read diagnostic bundle: %v", err)
	}
	reader, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatalf("read diagnostic zip: %v", err)
	}
	entries := make(map[string][]byte)
	for _, file := range reader.File {
		handle, err := file.Open()
		if err != nil {
			t.Fatalf("open %s: %v", file.Name, err)
		}
		body, err := io.ReadAll(handle)
		_ = handle.Close()
		if err != nil {
			t.Fatalf("read %s: %v", file.Name, err)
		}
		entries[file.Name] = body
	}
	return entries
}

func savedDesktopConfigSourcesForTest(t *testing.T) []any {
	t.Helper()
	raw, err := os.ReadFile(desktopConfigFilePath())
	if err != nil {
		t.Fatalf("read desktop config: %v", err)
	}
	var saved map[string]any
	if err := json.Unmarshal(raw, &saved); err != nil {
		t.Fatalf("parse desktop config: %v", err)
	}
	sources, ok := mapValue(saved["config_snapshot"])["sources"].([]any)
	if !ok {
		t.Fatalf("saved sources missing: %#v", saved)
	}
	return sources
}

func TestRenderExportFileTemplateSanitizesPathCharacters(t *testing.T) {
	got := renderExportFileTemplate("result-{date}-{time}-{task_id}-{profile}.csv", "task/1", "A:B", time.Date(2026, 5, 2, 3, 4, 5, 0, time.UTC))
	want := "result-20260502-030405-task_1-A_B.csv"
	if got != want {
		t.Fatalf("rendered template = %q, want %q", got, want)
	}
}

func TestColoDictionaryBridgeDoesNotSetFixedTimeouts(t *testing.T) {
	for _, path := range []string{"desktop_colo_dictionary.go", filepath.Join("..", "..", "mobileapi", "colo_dictionary.go")} {
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
