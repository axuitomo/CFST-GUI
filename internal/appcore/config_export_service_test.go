package appcore

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestServiceConfigExportOwnsSerializationAndPublishing(t *testing.T) {
	now := time.Date(2026, time.August, 10, 9, 8, 7, 0, time.FixedZone("CST", 8*60*60))
	root := t.TempDir()
	service := NewService(ServiceOptions{
		AppVersion: "test-version",
		ArchiveStorageState: func() any {
			return map[string]any{"backend": "test", "current_dir": root}
		},
		Clock:   func() time.Time { return now },
		Storage: StorageLayout{Root: root, ConfigFileName: "config.json", SourceProfilesFile: "source-profiles.json"},
		ConfigPolicy: ConfigPolicy{
			DefaultSnapshot:  func() map[string]any { return map[string]any{} },
			SanitizeSnapshot: func(snapshot map[string]any) map[string]any { return snapshot },
		},
	})
	snapshot := map[string]any{
		"cloudflare": map[string]any{"api_token": "full-token"},
		"probe":      map[string]any{"tcp_port": 443},
	}
	if _, err := service.SaveConfig(snapshot); err != nil {
		t.Fatal(err)
	}
	if err := service.saveSourceProfiles(SourceProfileStore{
		ActiveProfileID: "profile-1",
		Items:           []SourceProfileItem{{ID: "profile-1", Name: "Profile 1"}},
	}); err != nil {
		t.Fatal(err)
	}

	contentResult := service.Invoke("config.export", `{}`)
	if !contentResult.OK || contentResult.Code != "CONFIG_EXPORT_OK" {
		t.Fatalf("content export = %#v", contentResult)
	}
	contentData := mapValue(contentResult.Data)
	var exported map[string]any
	if err := json.Unmarshal([]byte(stringValue(contentData["content"], "")), &exported); err != nil {
		t.Fatal(err)
	}
	if exported["app_version"] != "test-version" || mapValue(exported["storage"])["backend"] != "test" {
		t.Fatalf("export envelope = %#v", exported)
	}
	if mapValue(mapValue(exported["config_snapshot"])["cloudflare"])["api_token"] != "full-token" {
		t.Fatalf("config snapshot = %#v", exported["config_snapshot"])
	}
	if mapValue(exported["source_profiles"])["active_profile_id"] != "profile-1" {
		t.Fatalf("source profiles = %#v", exported["source_profiles"])
	}

	targetPath := filepath.Join(root, "published", "config.json")
	publishedResult := service.Invoke("config.export", encodeSourceServicePayload(t, map[string]any{"target_path": targetPath}))
	if !publishedResult.OK || mapValue(publishedResult.Data)["path"] != targetPath {
		t.Fatalf("published export = %#v", publishedResult)
	}
	if raw, err := os.ReadFile(targetPath); err != nil || len(raw) == 0 {
		t.Fatalf("published file bytes = %d, %v", len(raw), err)
	}
}

func TestServiceConfigBackupUsesStorageLayoutAndClock(t *testing.T) {
	now := time.Date(2026, time.August, 10, 9, 8, 7, 0, time.UTC)
	root := t.TempDir()
	service := NewService(ServiceOptions{
		Clock:   func() time.Time { return now },
		Storage: StorageLayout{Root: root, ConfigFileName: "config.json", SourceProfilesFile: "source-profiles.json"},
		ConfigPolicy: ConfigPolicy{
			DefaultSnapshot:  func() map[string]any { return map[string]any{} },
			SanitizeSnapshot: func(snapshot map[string]any) map[string]any { return snapshot },
		},
	})
	result := service.Invoke("config.backup", encodeSourceServicePayload(t, map[string]any{
		"config_snapshot": map[string]any{"probe": map[string]any{"tcp_port": 2053}},
	}))
	wantPath := filepath.Join(root, "backups", "config-20260810-090807.json")
	if !result.OK || mapValue(result.Data)["path"] != wantPath {
		t.Fatalf("backup result = %#v, want %q", result, wantPath)
	}
	raw, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatal(err)
	}
	var backup map[string]any
	if err := json.Unmarshal(raw, &backup); err != nil {
		t.Fatal(err)
	}
	if backup["backed_up_at"] != now.Format(time.RFC3339) || intValue(mapValue(mapValue(backup["config_snapshot"])["probe"])["tcp_port"], 0) != 2053 {
		t.Fatalf("backup envelope = %#v", backup)
	}
}

func TestServiceConfigExportAndBackupRejectMalformedPayloads(t *testing.T) {
	service := NewService(ServiceOptions{})
	if result := service.Invoke("config.export", "{"); result.OK || result.Code != "CONFIG_EXPORT_INVALID" {
		t.Fatalf("export malformed result = %#v", result)
	}
	if result := service.Invoke("config.backup", "{"); result.OK || result.Code != "CONFIG_BACKUP_INVALID" {
		t.Fatalf("backup malformed result = %#v", result)
	}
}
