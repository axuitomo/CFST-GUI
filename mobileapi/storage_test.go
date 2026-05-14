package mobileapi

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestServiceStorageDirectoryStoresAndroidURI(t *testing.T) {
	service := NewService()
	decodeCommandForTest(t, service.Init(t.TempDir()))

	result := decodeCommandForTest(t, service.SetStorageDirectory(encodeJSON(map[string]any{
		"display_name": "Documents",
		"storage_uri":  "content://tree/documents",
	})))
	if !boolValue(result["ok"], false) {
		t.Fatalf("SetStorageDirectory failed: %#v", result)
	}
	data := mapValue(result["data"])
	storage := mapValue(data["storage"])
	if got := stringValue(storage["storage_uri"], ""); got != "content://tree/documents" {
		t.Fatalf("storage_uri = %q", got)
	}

	load := decodeCommandForTest(t, service.LoadConfig())
	loadStorage := mapValue(mapValue(load["data"])["storage"])
	if got := stringValue(loadStorage["storage_uri"], ""); got != "content://tree/documents" {
		t.Fatalf("load storage_uri = %q", got)
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

func TestServiceProfilesSwitchWritesConfig(t *testing.T) {
	service := NewService()
	decodeCommandForTest(t, service.Init(t.TempDir()))
	snapshot := defaultConfigSnapshot()
	mapValue(snapshot["cloudflare"])["record_name"] = "mobile.example.com"

	save := decodeCommandForTest(t, service.SaveCurrentProfile(encodeJSON(map[string]any{
		"config_snapshot": snapshot,
		"name":            "Mobile",
	})))
	if !boolValue(save["ok"], false) {
		t.Fatalf("SaveCurrentProfile failed: %#v", save)
	}
	store := mapValue(save["data"])
	items := store["items"].([]any)
	profileID := stringValue(mapValue(items[0])["id"], "")

	switched := decodeCommandForTest(t, service.SwitchProfile(encodeJSON(map[string]any{"profile_id": profileID})))
	if !boolValue(switched["ok"], false) {
		t.Fatalf("SwitchProfile failed: %#v", switched)
	}
	load := decodeCommandForTest(t, service.LoadConfig())
	cloudflare := mapValue(mapValue(mapValue(load["data"])["config_snapshot"])["cloudflare"])
	if got := stringValue(cloudflare["record_name"], ""); got != "mobile.example.com" {
		t.Fatalf("record_name = %q", got)
	}
}

func TestServiceUpdateCurrentProfileOverwritesActiveOrCreatesWhenMissing(t *testing.T) {
	service := NewService()
	decodeCommandForTest(t, service.Init(t.TempDir()))
	first := defaultConfigSnapshot()
	mapValue(first["cloudflare"])["record_name"] = "first-mobile.example.com"

	created := decodeCommandForTest(t, service.UpdateCurrentProfile(encodeJSON(map[string]any{
		"config_snapshot": first,
		"name":            "当前配置",
	})))
	if !boolValue(created["ok"], false) {
		t.Fatalf("UpdateCurrentProfile create failed: %#v", created)
	}
	store := mobileProfileStoreFromAnyForTest(t, created["data"])
	if store.ActiveProfileID == "" || len(store.Items) != 1 {
		t.Fatalf("created store = %#v, want one active profile", store)
	}
	profileID := store.ActiveProfileID

	second := defaultConfigSnapshot()
	mapValue(second["cloudflare"])["record_name"] = "second-mobile.example.com"
	updated := decodeCommandForTest(t, service.UpdateCurrentProfile(encodeJSON(map[string]any{
		"config_snapshot": second,
	})))
	if !boolValue(updated["ok"], false) {
		t.Fatalf("UpdateCurrentProfile update failed: %#v", updated)
	}
	store = mobileProfileStoreFromAnyForTest(t, updated["data"])
	if store.ActiveProfileID != profileID || len(store.Items) != 1 {
		t.Fatalf("updated store = %#v, want same active profile", store)
	}
	if got := stringValue(mapValue(store.Items[0].ConfigSnapshot["cloudflare"])["record_name"], ""); got != "second-mobile.example.com" {
		t.Fatalf("record_name = %q, want overwritten snapshot", got)
	}

	store.ActiveProfileID = "missing-mobile"
	if err := service.saveProfileStore(store); err != nil {
		t.Fatal(err)
	}
	third := defaultConfigSnapshot()
	mapValue(third["cloudflare"])["record_name"] = "third-mobile.example.com"
	createdMissing := decodeCommandForTest(t, service.UpdateCurrentProfile(encodeJSON(map[string]any{
		"config_snapshot": third,
		"name":            "缺失档案补建",
	})))
	if !boolValue(createdMissing["ok"], false) {
		t.Fatalf("UpdateCurrentProfile missing active failed: %#v", createdMissing)
	}
	store = mobileProfileStoreFromAnyForTest(t, createdMissing["data"])
	if store.ActiveProfileID != "missing-mobile" || len(store.Items) != 2 {
		t.Fatalf("missing-active store = %#v, want newly created active profile", store)
	}
}

func mobileProfileStoreFromAnyForTest(t *testing.T, value any) mobileProfileStore {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal profile store: %v", err)
	}
	var store mobileProfileStore
	if err := json.Unmarshal(raw, &store); err != nil {
		t.Fatalf("unmarshal profile store: %v", err)
	}
	return store
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
