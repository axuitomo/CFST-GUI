package mobileapi

import (
	"encoding/base64"
	"encoding/json"
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
