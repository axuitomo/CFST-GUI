package mobileapi

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/axuitomo/CFST-GUI/internal/appcore"
	"github.com/axuitomo/CFST-GUI/internal/probecore"
)

func TestServiceStorageDirectoryIsDeprecatedNoop(t *testing.T) {
	service := NewService()
	decodeCommandForTest(t, service.Init(t.TempDir()))

	result := decodeCommandForTest(t, service.SetStorageDirectory(encodeJSON(map[string]any{
		"display_name": "Documents",
		"storage_uri":  "content://tree/documents",
	})))
	if !boolValue(result["ok"], false) {
		t.Fatalf("SetStorageDirectory failed: %#v", result)
	}
	if got := stringValue(result["code"], ""); got != "STORAGE_SET_DEPRECATED" {
		t.Fatalf("code = %q, want STORAGE_SET_DEPRECATED", got)
	}
	data := mapValue(result["data"])
	storage := mapValue(data["storage"])
	if got := stringValue(storage["storage_uri"], ""); got != "" {
		t.Fatalf("storage_uri = %q, want empty", got)
	}
	if boolValue(storage["setup_required"], true) {
		t.Fatalf("setup_required = true, want false")
	}

	load := decodeCommandForTest(t, service.LoadConfig())
	loadStorage := mapValue(mapValue(load["data"])["storage"])
	if got := stringValue(loadStorage["storage_uri"], ""); got != "" {
		t.Fatalf("load storage_uri = %q, want empty", got)
	}
}

func TestServiceExportConfigReturnsFullTokenContent(t *testing.T) {
	service := NewService()
	decodeCommandForTest(t, service.Init(t.TempDir()))
	snapshot := defaultConfigSnapshot()
	mapValue(snapshot["cloudflare"])["api_token"] = "mobile-secret-token"

	result := decodeCommandForTest(t, service.ExportConfig(encodeJSON(map[string]any{
		"config_snapshot": snapshot,
	})))
	if !boolValue(result["ok"], false) {
		t.Fatalf("ExportConfig failed: %#v", result)
	}
	content := stringValue(mapValue(result["data"])["content"], "")
	if !strings.Contains(content, "mobile-secret-token") {
		t.Fatalf("export content did not include full token: %s", content)
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		t.Fatalf("export content is not valid JSON: %v", err)
	}
}

func TestServiceExportDebugLogWritesConfiguredDirectory(t *testing.T) {
	service := NewService()
	decodeCommandForTest(t, service.Init(t.TempDir()))
	if err := os.WriteFile(service.debugLogPath(), []byte("mobile debug\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	targetDir := filepath.Join(t.TempDir(), "exports")

	result := decodeCommandForTest(t, service.ExportDebugLog(encodeJSON(map[string]any{
		"config": map[string]any{
			"export": map[string]any{
				"target_dir": targetDir,
			},
		},
		"file_name": "mobile-log.txt",
	})))
	if !boolValue(result["ok"], false) || stringValue(result["code"], "") != "DEBUG_LOG_EXPORT_OK" {
		t.Fatalf("ExportDebugLog failed: %#v", result)
	}
	targetPath := filepath.Join(targetDir, "mobile-log.txt")
	if got := stringValue(mapValue(result["data"])["path"], ""); got != targetPath {
		t.Fatalf("path = %q, want %q", got, targetPath)
	}
	raw, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("read exported log: %v", err)
	}
	if string(raw) != "mobile debug\n" {
		t.Fatalf("exported log = %q", string(raw))
	}
}

func TestServiceLoadConfigSanitizesLegacySnapshotWithoutWriting(t *testing.T) {
	service := NewService()
	decodeCommandForTest(t, service.Init(t.TempDir()))
	legacy := []byte(`{
  "config_snapshot": {
    "cloudflare": {
      "apiToken": "mobile-secret-token",
      "recordName": "mobile.example.com",
      "unknown_cloudflare": true
    },
    "probe": {
      "strategy": "full",
      "routines": 333,
      "retryMaxAttempts": 4,
      "cooldownMs": 555,
      "maxDelayMS": 1234,
      "ipText": "203.0.113.20",
      "unknown_probe": true
    },
    "backup": {
      "webdav": {
        "url": "https://dav.example.com/mobile",
        "remotePath": "mobile.zip",
        "unknown_webdav": true
      }
    },
    "scheduler": {
      "dailyTimes": "03:00; 04:00",
      "unknown_scheduler": true
    },
    "unknown_root": true
  }
}`)
	if err := os.WriteFile(service.configPath(), legacy, 0o600); err != nil {
		t.Fatal(err)
	}

	load := decodeCommandForTest(t, service.LoadConfig())
	if !boolValue(load["ok"], false) {
		t.Fatalf("LoadConfig failed: %#v", load)
	}
	afterLoad, err := os.ReadFile(service.configPath())
	if err != nil {
		t.Fatal(err)
	}
	if string(afterLoad) != string(legacy) {
		t.Fatalf("LoadConfig rewrote config file, want read-only compatibility")
	}

	snapshot := mapValue(mapValue(load["data"])["config_snapshot"])
	if _, exists := snapshot["unknown_root"]; exists {
		t.Fatalf("unknown_root was preserved: %#v", snapshot)
	}
	cloudflare := mapValue(snapshot["cloudflare"])
	if got := stringValue(cloudflare["api_token"], ""); got != "mobile-secret-token" {
		t.Fatalf("api_token = %q, want mobile token", got)
	}
	if _, exists := cloudflare["apiToken"]; exists {
		t.Fatalf("apiToken alias was preserved: %#v", cloudflare)
	}
	webdav := mapValue(mapValue(snapshot["backup"])["webdav"])
	if got := stringValue(webdav["server_url"], ""); got != "https://dav.example.com/mobile" {
		t.Fatalf("server_url = %q, want migrated url", got)
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
	sources, ok := snapshot["sources"].([]any)
	if !ok || len(sources) != 1 || stringValue(mapValue(sources[0])["content"], "") != "203.0.113.20" {
		t.Fatalf("sources = %#v, want migrated sourceText/ipText source", snapshot["sources"])
	}
}

func TestServiceLoadConfigIgnoresTrailingGarbageAndWarns(t *testing.T) {
	service := NewService()
	decodeCommandForTest(t, service.Init(t.TempDir()))

	snapshot := defaultConfigSnapshot()
	if err := service.writeConfigSnapshot(snapshot); err != nil {
		t.Fatalf("writeConfigSnapshot: %v", err)
	}
	raw, err := os.ReadFile(service.configPath())
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if err := os.WriteFile(service.configPath(), append(raw, []byte("4")...), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	load := decodeCommandForTest(t, service.LoadConfig())
	if !boolValue(load["ok"], false) {
		t.Fatalf("LoadConfig failed: %#v", load)
	}
	warnings := stringSliceForTest(load["warnings"])
	if !containsForTest(warnings, "尾部存在残留内容") {
		t.Fatalf("warnings = %#v, want trailing-content warning", warnings)
	}
}

func TestServiceSaveConfigSanitizesLegacySnapshotOnDisk(t *testing.T) {
	service := NewService()
	decodeCommandForTest(t, service.Init(t.TempDir()))

	save := decodeCommandForTest(t, service.SaveConfig(encodeJSON(map[string]any{
		"config_snapshot": map[string]any{
			"cloudflare": map[string]any{
				"apiToken": "mobile-secret-token",
				"obsolete": "drop-me",
			},
			"probe": map[string]any{
				"retryMaxAttempts": 5,
				"unknown_probe":    true,
			},
			"backup": map[string]any{
				"webdav": map[string]any{
					"url":            "https://dav.example.com/mobile",
					"timeoutSeconds": 45,
					"unknown_webdav": true,
				},
			},
			"unknown_root": true,
		},
	})))
	if !boolValue(save["ok"], false) {
		t.Fatalf("SaveConfig failed: %#v", save)
	}

	raw, err := os.ReadFile(service.configPath())
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
	if got := stringValue(cloudflare["api_token"], ""); got != "mobile-secret-token" {
		t.Fatalf("api_token = %q, want mobile token", got)
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
	if got := stringValue(webdav["server_url"], ""); got != "https://dav.example.com/mobile" {
		t.Fatalf("server_url = %q, want migrated url", got)
	}
	if got := intValue(webdav["timeout_seconds"], 0); got != 45 {
		t.Fatalf("timeout_seconds = %d, want 45", got)
	}
	if _, exists := webdav["unknown_webdav"]; exists {
		t.Fatalf("unknown_webdav was saved: %#v", webdav)
	}
}

func TestServiceSaveConfigPreservesThemeAndPortPolicy(t *testing.T) {
	service := NewService()
	decodeCommandForTest(t, service.Init(t.TempDir()))

	snapshot := defaultConfigSnapshot()
	probe := mapValue(snapshot["probe"])
	probe["port_policy"] = probecore.PortPolicyFixedGlobal
	ui := mapValue(snapshot["ui"])
	ui["theme_mode"] = "auto_time"
	ui["theme_light_start"] = "06:30"
	ui["theme_dark_start"] = "20:45"
	ui["utc_offset_minutes"] = 330

	save := decodeCommandForTest(t, service.SaveConfig(encodeJSON(map[string]any{
		"config_snapshot": snapshot,
	})))
	if !boolValue(save["ok"], false) {
		t.Fatalf("SaveConfig failed: %#v", save)
	}

	load := decodeCommandForTest(t, service.LoadConfig())
	if !boolValue(load["ok"], false) {
		t.Fatalf("LoadConfig failed: %#v", load)
	}

	loaded := mapValue(mapValue(load["data"])["config_snapshot"])
	loadedProbe := mapValue(loaded["probe"])
	if got := stringValue(loadedProbe["port_policy"], ""); got != probecore.PortPolicyFixedGlobal {
		t.Fatalf("port_policy = %q, want %q", got, probecore.PortPolicyFixedGlobal)
	}
	loadedUI := mapValue(loaded["ui"])
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

func TestServiceImportConfigArchiveSanitizesLegacySnapshot(t *testing.T) {
	service := NewService()
	decodeCommandForTest(t, service.Init(t.TempDir()))
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
	archive, err := zipMobileSingleFile(configArchiveEntryName, raw)
	if err != nil {
		t.Fatal(err)
	}

	result := decodeCommandForTest(t, service.ImportConfigArchive(encodeJSON(map[string]any{
		"content_base64":          base64.StdEncoding.EncodeToString(archive),
		"current_config_snapshot": defaultConfigSnapshot(),
	})))
	if !boolValue(result["ok"], false) {
		t.Fatalf("ImportConfigArchive failed: %#v", result)
	}
	savedSnapshot, err := service.loadConfigSnapshotFromDisk()
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

func TestServiceImportConfigArchivePreservesSnapshotSourcesWithSourceProfiles(t *testing.T) {
	service := NewService()
	decodeCommandForTest(t, service.Init(t.TempDir()))
	currentSources := []desktopSource{
		{
			Enabled: true,
			ID:      "source-current",
			IPMode:  "traverse",
			Kind:    "url",
			Name:    "Current Sources",
			URL:     "https://current.example/top10.txt",
		},
	}
	staleProfileSources := []desktopSource{
		{
			Enabled: true,
			ID:      "source-stale",
			IPMode:  "traverse",
			Kind:    "url",
			Name:    "Stale Profile Sources",
			URL:     "https://stale.example/top10.txt",
		},
	}
	snapshot := defaultConfigSnapshot()
	snapshot["sources"] = currentSources
	body := map[string]any{
		"config_snapshot": snapshot,
		"source_profiles": mobileSourceProfileStore{
			ActiveProfileID: "source-profile-stale",
			Items: []mobileSourceProfileItem{
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
	archive, err := zipMobileSingleFile(configArchiveEntryName, raw)
	if err != nil {
		t.Fatal(err)
	}

	result := decodeCommandForTest(t, service.ImportConfigArchive(encodeJSON(map[string]any{
		"content_base64":          base64.StdEncoding.EncodeToString(archive),
		"current_config_snapshot": defaultConfigSnapshot(),
	})))
	if !boolValue(result["ok"], false) {
		t.Fatalf("ImportConfigArchive failed: %#v", result)
	}
	savedSnapshot, err := service.loadConfigSnapshotFromDisk()
	if err != nil {
		t.Fatalf("load saved snapshot: %v", err)
	}
	savedSources := mobileSourcesFromAny(savedSnapshot["sources"])
	if len(savedSources) != 1 || savedSources[0].URL != "https://current.example/top10.txt" {
		t.Fatalf("saved sources = %#v, want current snapshot sources", savedSources)
	}
	store, err := service.loadSourceProfileStore()
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

func TestServiceImportConfigArchiveWithoutSourceProfilesCreatesDefaultFromSnapshotSources(t *testing.T) {
	service := NewService()
	decodeCommandForTest(t, service.Init(t.TempDir()))
	snapshot := defaultConfigSnapshot()
	snapshot["sources"] = []desktopSource{
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
	archive, err := zipMobileSingleFile(configArchiveEntryName, raw)
	if err != nil {
		t.Fatal(err)
	}

	result := decodeCommandForTest(t, service.ImportConfigArchive(encodeJSON(map[string]any{
		"content_base64":          base64.StdEncoding.EncodeToString(archive),
		"current_config_snapshot": defaultConfigSnapshot(),
	})))
	if !boolValue(result["ok"], false) {
		t.Fatalf("ImportConfigArchive failed: %#v", result)
	}
	store, err := service.loadSourceProfileStore()
	if err != nil {
		t.Fatalf("load source profiles: %v", err)
	}
	if store.ActiveProfileID != defaultSourceProfileID || len(store.Items) != 1 {
		t.Fatalf("source profile store = %#v, want generated default profile", store)
	}
	if len(store.Items[0].Sources) != 1 || store.Items[0].Sources[0].URL != "https://current.example/top10.txt" {
		t.Fatalf("default source profile sources = %#v, want snapshot sources", store.Items[0].Sources)
	}
	pipelineStore, err := service.loadPipelineProfileStore()
	if err != nil {
		t.Fatalf("load pipeline profiles: %v", err)
	}
	if pipelineStore.ActiveProfileID == "" || len(pipelineStore.Items) != 1 || !pipelineStore.Items[0].Enabled {
		t.Fatalf("pipeline profile store = %#v, want generated default profile", pipelineStore)
	}
	returnedPipelineStore := appcore.PipelineProfileStoreFromAny(mapValue(mapValue(result["data"])["pipeline_profiles"]))
	if returnedPipelineStore.ActiveProfileID != pipelineStore.ActiveProfileID || len(returnedPipelineStore.Items) != len(pipelineStore.Items) {
		t.Fatalf("returned pipeline store = %#v, want persisted store %#v", returnedPipelineStore, pipelineStore)
	}
}

func TestServiceImportConfigArchiveRollsBackWhenSourceProfileSaveFails(t *testing.T) {
	service := NewService()
	decodeCommandForTest(t, service.Init(t.TempDir()))
	oldSnapshot := defaultConfigSnapshot()
	oldSnapshot["cloudflare"] = map[string]any{"api_token": "old-token"}
	if err := service.writeConfigSnapshot(oldSnapshot); err != nil {
		t.Fatalf("writeConfigSnapshot: %v", err)
	}
	oldSourceProfiles := mobileSourceProfileStore{
		ActiveProfileID: "source-profile-old",
		Items: []mobileSourceProfileItem{
			{
				ID:      "source-profile-old",
				Name:    "旧输入源档案",
				Sources: []desktopSource{{ID: "source-old", Kind: "url", URL: "https://old.example/top10.txt"}},
			},
		},
		SchemaVersion: sourceProfilesSchemaVersion,
	}
	if err := service.saveSourceProfileStore(oldSourceProfiles); err != nil {
		t.Fatalf("saveSourceProfileStore: %v", err)
	}
	oldPipelineProfiles := pipelineProfileStore{
		ActiveProfileID: "pipeline-old",
		Items: []pipelineProfile{
			{
				ConfigSnapshot: map[string]any{"cloudflare": map[string]any{"record_name": "old.example.com"}},
				Domain:         "old.example.com",
				Enabled:        true,
				ID:             "pipeline-old",
				Name:           "旧策略",
				Region:         "旧地域",
			},
		},
		SchemaVersion: pipelineProfilesSchemaVersion,
	}
	if err := service.savePipelineProfileStore(oldPipelineProfiles); err != nil {
		t.Fatalf("savePipelineProfileStore: %v", err)
	}
	raw, err := json.Marshal(map[string]any{
		"config_snapshot": map[string]any{
			"cloudflare": map[string]any{"api_token": "new-token"},
		},
		"source_profiles": mobileSourceProfileStore{
			ActiveProfileID: "source-profile-new",
			Items: []mobileSourceProfileItem{
				{
					ID:      "source-profile-new",
					Name:    "新输入源档案",
					Sources: []desktopSource{{ID: "source-new", Kind: "url", URL: "https://new.example/top10.txt"}},
				},
			},
			SchemaVersion: sourceProfilesSchemaVersion,
		},
		"pipeline_profiles": pipelineProfileStore{
			ActiveProfileID: "pipeline-new",
			Items: []pipelineProfile{
				{
					ConfigSnapshot: map[string]any{"cloudflare": map[string]any{"record_name": "new.example.com"}},
					Domain:         "new.example.com",
					Enabled:        true,
					ID:             "pipeline-new",
					Name:           "新策略",
					Region:         "新地域",
				},
			},
			SchemaVersion: pipelineProfilesSchemaVersion,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	archive, err := zipMobileSingleFile(configArchiveEntryName, raw)
	if err != nil {
		t.Fatal(err)
	}
	originalHook := mobileSaveSourceProfileStoreForImport
	mobileSaveSourceProfileStoreForImport = func(s *Service, store mobileSourceProfileStore) error {
		return errors.New("inject source profile save failure")
	}
	t.Cleanup(func() {
		mobileSaveSourceProfileStoreForImport = originalHook
	})

	result := decodeCommandForTest(t, service.ImportConfigArchive(encodeJSON(map[string]any{
		"content_base64":          base64.StdEncoding.EncodeToString(archive),
		"current_config_snapshot": oldSnapshot,
	})))
	if boolValue(result["ok"], false) {
		t.Fatalf("ImportConfigArchive unexpectedly succeeded: %#v", result)
	}
	savedSnapshot, err := service.loadConfigSnapshotFromDisk()
	if err != nil {
		t.Fatalf("loadConfigSnapshotFromDisk: %v", err)
	}
	if got := stringValue(mapValue(savedSnapshot["cloudflare"])["api_token"], ""); got != "old-token" {
		t.Fatalf("saved config api_token = %q, want old-token", got)
	}
	restoredSourceProfiles, err := service.loadSourceProfileStore()
	if err != nil {
		t.Fatalf("loadSourceProfileStore: %v", err)
	}
	if restoredSourceProfiles.ActiveProfileID != "source-profile-old" {
		t.Fatalf("restored source profiles active = %q, want source-profile-old", restoredSourceProfiles.ActiveProfileID)
	}
	if got := restoredSourceProfiles.Items[0].Sources[0].URL; got != "https://old.example/top10.txt" {
		t.Fatalf("restored source profile url = %q, want old url", got)
	}
	restoredPipelineProfiles, err := service.loadPipelineProfileStore()
	if err != nil {
		t.Fatalf("loadPipelineProfileStore: %v", err)
	}
	if restoredPipelineProfiles.ActiveProfileID != "pipeline-old" || len(restoredPipelineProfiles.Items) != 1 || restoredPipelineProfiles.Items[0].Domain != "old.example.com" {
		t.Fatalf("restored pipeline profiles = %#v, want old store", restoredPipelineProfiles)
	}
}

func TestServiceSaveSourceProfileAllowsBlankActiveAndWritesConfig(t *testing.T) {
	service := NewService()
	decodeCommandForTest(t, service.Init(t.TempDir()))

	result := decodeCommandForTest(t, service.SaveSourceProfile(encodeJSON(map[string]any{
		"name":       "空白档案",
		"profile_id": "blank",
		"set_active": true,
		"sources":    []desktopSource{},
	})))
	if !boolValue(result["ok"], false) {
		t.Fatalf("SaveSourceProfile failed: %#v", result)
	}
	store := mapValue(result["data"])
	items := store["items"].([]any)
	if stringValue(store["active_profile_id"], "") != "blank" || len(items) != 1 {
		t.Fatalf("source profile store = %#v, want blank active profile", store)
	}
	if sources := mapValue(items[0])["sources"].([]any); len(sources) != 0 {
		t.Fatalf("profile sources = %#v, want empty", sources)
	}
	if sources := savedMobileConfigSourcesForTest(t, service); len(sources) != 0 {
		t.Fatalf("saved config sources = %#v, want empty", sources)
	}
}

func TestServiceDeleteSourceProfileSwitchesActiveSources(t *testing.T) {
	service := NewService()
	decodeCommandForTest(t, service.Init(t.TempDir()))
	firstSources := []desktopSource{mobileSourceProfileTestSource("source-one", "Source One")}
	secondSources := []desktopSource{mobileSourceProfileTestSource("source-two", "Source Two")}

	if result := decodeCommandForTest(t, service.SaveSourceProfile(encodeJSON(map[string]any{"name": "one", "profile_id": "one", "set_active": true, "sources": firstSources}))); !boolValue(result["ok"], false) {
		t.Fatalf("save first source profile failed: %#v", result)
	}
	if result := decodeCommandForTest(t, service.SaveSourceProfile(encodeJSON(map[string]any{"name": "two", "profile_id": "two", "set_active": true, "sources": secondSources}))); !boolValue(result["ok"], false) {
		t.Fatalf("save second source profile failed: %#v", result)
	}

	result := decodeCommandForTest(t, service.DeleteSourceProfile(encodeJSON(map[string]any{"profile_id": "two"})))
	if !boolValue(result["ok"], false) {
		t.Fatalf("DeleteSourceProfile failed: %#v", result)
	}
	store := mapValue(result["data"])
	if got := stringValue(store["active_profile_id"], ""); got != "one" {
		t.Fatalf("active profile = %q, want one", got)
	}
	sources := savedMobileConfigSourcesForTest(t, service)
	if len(sources) != 1 || sources[0].Name != "Source One" {
		t.Fatalf("saved config sources = %#v, want source one", sources)
	}
}

func TestServiceDeleteLastSourceProfileCreatesBlankDefault(t *testing.T) {
	service := NewService()
	decodeCommandForTest(t, service.Init(t.TempDir()))

	if result := decodeCommandForTest(t, service.SaveSourceProfile(encodeJSON(map[string]any{
		"name":       "only",
		"profile_id": "only",
		"set_active": true,
		"sources":    []desktopSource{mobileSourceProfileTestSource("only-source", "Only Source")},
	}))); !boolValue(result["ok"], false) {
		t.Fatalf("save only source profile failed: %#v", result)
	}

	result := decodeCommandForTest(t, service.DeleteSourceProfile(encodeJSON(map[string]any{"profile_id": "only"})))
	if !boolValue(result["ok"], false) {
		t.Fatalf("DeleteSourceProfile failed: %#v", result)
	}
	store := mapValue(result["data"])
	items := store["items"].([]any)
	if stringValue(store["active_profile_id"], "") != defaultSourceProfileID || len(items) != 1 {
		t.Fatalf("source profile store = %#v, want blank default profile", store)
	}
	if sources := mapValue(items[0])["sources"].([]any); len(sources) != 0 {
		t.Fatalf("default profile sources = %#v, want empty", sources)
	}
	if sources := savedMobileConfigSourcesForTest(t, service); len(sources) != 0 {
		t.Fatalf("saved config sources = %#v, want empty", sources)
	}
}

func TestServiceUpdateCurrentSourceProfileOverwritesActiveOrCreatesWhenMissing(t *testing.T) {
	service := NewService()
	decodeCommandForTest(t, service.Init(t.TempDir()))

	firstSources := []desktopSource{mobileSourceProfileTestSource("source-one", "Source One")}
	created := decodeCommandForTest(t, service.UpdateCurrentSourceProfile(encodeJSON(map[string]any{
		"name":    "当前输入源",
		"sources": firstSources,
	})))
	if !boolValue(created["ok"], false) {
		t.Fatalf("UpdateCurrentSourceProfile create failed: %#v", created)
	}
	createdData := mapValue(created["data"])
	createdStore := mapValue(createdData["source_profiles"])
	activeID := stringValue(createdStore["active_profile_id"], "")
	items := createdStore["items"].([]any)
	if activeID == "" || len(items) != 1 {
		t.Fatalf("source profile store = %#v, want one active profile", createdStore)
	}

	secondSources := []desktopSource{mobileSourceProfileTestSource("source-two", "Source Two")}
	updated := decodeCommandForTest(t, service.UpdateCurrentSourceProfile(encodeJSON(map[string]any{
		"sources": secondSources,
	})))
	if !boolValue(updated["ok"], false) {
		t.Fatalf("UpdateCurrentSourceProfile update failed: %#v", updated)
	}
	updatedStore := mapValue(mapValue(updated["data"])["source_profiles"])
	if got := stringValue(updatedStore["active_profile_id"], ""); got != activeID {
		t.Fatalf("active profile id = %q, want %q", got, activeID)
	}
	if sources := savedMobileConfigSourcesForTest(t, service); len(sources) != 1 || sources[0].Name != "Source Two" {
		t.Fatalf("saved config sources = %#v, want Source Two", sources)
	}

	store := mobileSourceProfileStoreFromAny(updatedStore)
	store.ActiveProfileID = "missing-active"
	if err := service.saveSourceProfileStore(store); err != nil {
		t.Fatalf("save source profile store: %v", err)
	}
	thirdSources := []desktopSource{mobileSourceProfileTestSource("source-three", "Source Three")}
	createdMissing := decodeCommandForTest(t, service.UpdateCurrentSourceProfile(encodeJSON(map[string]any{
		"name":    "缺失输入源补建",
		"sources": thirdSources,
	})))
	if !boolValue(createdMissing["ok"], false) {
		t.Fatalf("UpdateCurrentSourceProfile missing active failed: %#v", createdMissing)
	}
	missingStore := mapValue(mapValue(createdMissing["data"])["source_profiles"])
	if got := stringValue(missingStore["active_profile_id"], ""); got != "missing-active" {
		t.Fatalf("active profile id = %q, want missing-active recreated", got)
	}
	if sources := savedMobileConfigSourcesForTest(t, service); len(sources) != 1 || sources[0].Name != "Source Three" {
		t.Fatalf("saved config sources = %#v, want Source Three", sources)
	}
}

func mobileSourceProfileTestSource(id, name string) desktopSource {
	return desktopSource{
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

func savedMobileConfigSourcesForTest(t *testing.T, service *Service) []desktopSource {
	t.Helper()
	snapshot, err := service.loadConfigSnapshotFromDisk()
	if err != nil {
		t.Fatalf("load saved snapshot: %v", err)
	}
	return mobileSourcesFromAny(snapshot["sources"])
}
