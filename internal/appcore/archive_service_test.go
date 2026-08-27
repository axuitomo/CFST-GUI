package appcore

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestServiceArchiveExportSupportsFileAndPublishingTargets(t *testing.T) {
	service := newArchiveServiceForTest(t, ServiceOptions{})
	snapshot := archiveSnapshotForTest("local", "")
	if _, err := service.SaveConfig(snapshot); err != nil {
		t.Fatal(err)
	}
	targetPath := filepath.Join(t.TempDir(), "config.zip")
	fileResult := service.Invoke("archive.export", encodeArchivePayload(t, map[string]any{
		"config_snapshot": snapshot,
		"target_path":     targetPath,
	}))
	if !fileResult.OK || fileResult.Code != "CONFIG_ARCHIVE_EXPORT_OK" {
		t.Fatalf("file export = %#v", fileResult)
	}
	raw, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseConfigArchive(raw)
	if err != nil || len(mapValue(parsed["config_snapshot"])) == 0 {
		t.Fatalf("parsed archive = %#v, %v", parsed, err)
	}

	publishResult := service.Invoke("archive.export", encodeArchivePayload(t, map[string]any{
		"config_snapshot": snapshot,
		"target_uri":      "content://exports/config.zip",
	}))
	if !publishResult.OK {
		t.Fatalf("publish export = %#v", publishResult)
	}
	publishData := mapValue(publishResult.Data)
	published, err := base64.StdEncoding.DecodeString(stringValue(publishData["content_base64"], ""))
	if err != nil || len(published) == 0 {
		t.Fatalf("published archive bytes = %d, %v", len(published), err)
	}
	if publishData["target_uri"] != "content://exports/config.zip" {
		t.Fatalf("target_uri = %#v", publishData["target_uri"])
	}
}

func TestServiceArchiveImportPreservesLocalTargetProfilesAndCreatesBackup(t *testing.T) {
	service := newArchiveServiceForTest(t, ServiceOptions{})
	current := archiveSnapshotForTest("local.csv", "content://local/exports")
	if _, err := service.SaveConfig(current); err != nil {
		t.Fatal(err)
	}
	oldStore := SourceProfileStore{
		ActiveProfileID: "old-profile",
		Items:           []SourceProfileItem{{ID: "old-profile", Name: "Old"}},
		SchemaVersion:   DefaultSourceProfilesSchemaVersion,
	}
	if err := service.saveSourceProfiles(oldStore); err != nil {
		t.Fatal(err)
	}
	incoming := archiveSnapshotForTest("restored.csv", "content://remote/exports")
	importedStore := SourceProfileStore{
		ActiveProfileID: "imported-profile",
		Items: []SourceProfileItem{{
			ID:      "imported-profile",
			Name:    "Imported",
			Sources: []Source{{ID: "source-imported", Kind: "url", URL: "https://example.com/ips.txt"}},
		}},
		SchemaVersion: DefaultSourceProfilesSchemaVersion,
	}
	archive, _, err := BuildConfigArchive(incoming, importedStore, nil, "test", ConfigSchemaVersion, time.Now().Format(time.RFC3339))
	if err != nil {
		t.Fatal(err)
	}
	result := service.Invoke("archive.import", encodeArchivePayload(t, map[string]any{
		"content_base64":          base64.StdEncoding.EncodeToString(archive),
		"current_config_snapshot": current,
	}))
	if !result.OK || result.Code != "CONFIG_ARCHIVE_IMPORT_OK" {
		t.Fatalf("archive import = %#v", result)
	}
	loaded, err := service.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	exportConfig := mapValue(loaded.Snapshot["export"])
	if exportConfig["file_name"] != "restored.csv" || exportConfig["target_uri"] != "content://local/exports" {
		t.Fatalf("preserved export config = %#v", exportConfig)
	}
	store, err := LoadSourceProfileStore(service.StorageLayout().SourceProfilesPath(), DefaultSourceProfilesSchemaVersion)
	if err != nil || store.ActiveProfileID != "imported-profile" {
		t.Fatalf("source profiles = %#v, %v", store, err)
	}
	backupPath := stringValue(mapValue(result.Data)["backup_path"], "")
	if info, err := os.Stat(backupPath); err != nil || info.IsDir() {
		t.Fatalf("backup_path = %q, %v", backupPath, err)
	}
}

func TestServiceArchiveImportRollsBackConfigAndProfiles(t *testing.T) {
	root := t.TempDir()
	layout := archiveLayoutForTest(root)
	seed := newArchiveServiceAt(t, layout, ServiceOptions{})
	oldSnapshot := archiveSnapshotForTest("old.csv", "content://old")
	oldSnapshot["marker"] = "old"
	if _, err := seed.SaveConfig(oldSnapshot); err != nil {
		t.Fatal(err)
	}
	oldStore := SourceProfileStore{ActiveProfileID: "old", Items: []SourceProfileItem{{ID: "old", Name: "Old"}}, SchemaVersion: DefaultSourceProfilesSchemaVersion}
	if err := seed.saveSourceProfiles(oldStore); err != nil {
		t.Fatal(err)
	}
	service := newArchiveServiceAt(t, layout, ServiceOptions{
		ArchiveSaveSourceProfiles: func(SourceProfileStore) error { return errors.New("injected source profile failure") },
	})
	incoming := archiveSnapshotForTest("new.csv", "content://new")
	incoming["marker"] = "new"
	archive, _, err := BuildConfigArchive(incoming, SourceProfileStore{ActiveProfileID: "new", Items: []SourceProfileItem{{ID: "new", Name: "New"}}}, nil, "test", ConfigSchemaVersion, time.Now().Format(time.RFC3339))
	if err != nil {
		t.Fatal(err)
	}
	result := service.Invoke("archive.import", encodeArchivePayload(t, map[string]any{
		"content_base64":          base64.StdEncoding.EncodeToString(archive),
		"current_config_snapshot": oldSnapshot,
	}))
	if result.OK || result.Code != "CONFIG_ARCHIVE_IMPORT_SOURCE_PROFILE_SAVE_FAILED" {
		t.Fatalf("archive import = %#v", result)
	}
	loaded, err := service.LoadConfig()
	if err != nil || loaded.Snapshot["marker"] != "old" {
		t.Fatalf("rolled back config = %#v, %v", loaded.Snapshot, err)
	}
	store, err := LoadSourceProfileStore(layout.SourceProfilesPath(), DefaultSourceProfilesSchemaVersion)
	if err != nil || store.ActiveProfileID != "old" {
		t.Fatalf("rolled back profiles = %#v, %v", store, err)
	}
}

func TestServiceWebDAVTestBackupAndRestore(t *testing.T) {
	var remoteArchive []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		user, password, _ := request.BasicAuth()
		if user != "user" || password != "password" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch request.Method {
		case http.MethodHead:
			w.WriteHeader(http.StatusNotFound)
		case http.MethodPut:
			defer request.Body.Close()
			remoteArchive, _ = io.ReadAll(request.Body)
			w.WriteHeader(http.StatusCreated)
		case http.MethodGet:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(remoteArchive)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()

	service := newArchiveServiceForTest(t, ServiceOptions{})
	snapshot := archiveSnapshotForTest("local.csv", "content://local")
	snapshot["marker"] = "remote"
	snapshot["backup"] = map[string]any{"webdav": map[string]any{
		"password":    "password",
		"remote_path": "backups/config.zip",
		"server_url":  server.URL,
		"username":    "user",
	}}
	if _, err := service.SaveConfig(snapshot); err != nil {
		t.Fatal(err)
	}
	if result := service.Invoke("webdav.test", `{}`); !result.OK || result.Code != "WEBDAV_TEST_OK" {
		t.Fatalf("webdav test = %#v", result)
	}
	if result := service.Invoke("webdav.backup", `{}`); !result.OK || result.Code != "WEBDAV_BACKUP_OK" {
		t.Fatalf("webdav backup = %#v", result)
	}
	if len(remoteArchive) == 0 {
		t.Fatal("webdav backup did not upload archive")
	}
	loaded, err := service.LoadConfig()
	if err != nil || stringValue(mapValue(mapValue(loaded.Snapshot["backup"])["webdav"])["last_backup_at"], "") == "" {
		t.Fatalf("backup timestamp = %#v, %v", loaded.Snapshot, err)
	}
	loaded.Snapshot["marker"] = "local-change"
	if _, err := service.SaveConfig(loaded.Snapshot); err != nil {
		t.Fatal(err)
	}
	webdav := mapValue(mapValue(loaded.Snapshot["backup"])["webdav"])
	restoreResult := service.Invoke("webdav.restore", encodeArchivePayload(t, map[string]any{
		"current_config_snapshot": loaded.Snapshot,
		"webdav":                  webdav,
	}))
	if !restoreResult.OK || restoreResult.Code != "CONFIG_ARCHIVE_IMPORT_OK" {
		t.Fatalf("webdav restore = %#v", restoreResult)
	}
	loaded, err = service.LoadConfig()
	if err != nil || loaded.Snapshot["marker"] != "remote" {
		t.Fatalf("restored config = %#v, %v", loaded.Snapshot, err)
	}
	if stringValue(mapValue(mapValue(loaded.Snapshot["backup"])["webdav"])["last_restore_at"], "") == "" {
		t.Fatalf("restore timestamp missing: %#v", loaded.Snapshot)
	}
}

func newArchiveServiceForTest(t *testing.T, overrides ServiceOptions) *Service {
	t.Helper()
	return newArchiveServiceAt(t, archiveLayoutForTest(t.TempDir()), overrides)
}

func newArchiveServiceAt(t *testing.T, layout StorageLayout, overrides ServiceOptions) *Service {
	t.Helper()
	options := overrides
	options.AppVersion = "test"
	options.Clock = func() time.Time { return time.Date(2026, time.August, 9, 13, 0, 0, 0, time.UTC) }
	options.Storage = layout
	options.ConfigPolicy = ConfigPolicy{
		DefaultSnapshot: func() map[string]any { return archiveSnapshotForTest("result.csv", "") },
		SanitizeSnapshot: func(snapshot map[string]any) map[string]any {
			raw, _ := json.Marshal(snapshot)
			cloned := map[string]any{}
			_ = json.Unmarshal(raw, &cloned)
			return cloned
		},
	}
	return NewService(options)
}

func archiveLayoutForTest(root string) StorageLayout {
	return StorageLayout{Root: root, ConfigFileName: "config.json", DraftFileName: "draft.json", SchedulerFileName: "scheduler.json", SourceProfilesFile: "profiles.json"}
}

func archiveSnapshotForTest(fileName, targetURI string) map[string]any {
	return map[string]any{
		"export":  map[string]any{"file_name": fileName, "target_dir": "", "target_uri": targetURI},
		"sources": []any{},
	}
}

func encodeArchivePayload(t *testing.T, payload map[string]any) string {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}
