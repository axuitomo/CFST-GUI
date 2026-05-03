package mobileapi

import (
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
