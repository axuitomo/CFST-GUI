package appcore

import (
	"testing"
	"time"
)

func TestServiceInvokeManagesSourceProfilesAndConfigTogether(t *testing.T) {
	now := time.Date(2026, 8, 8, 12, 30, 0, 0, time.UTC)
	service := NewService(ServiceOptions{
		Clock: func() time.Time { return now },
		Storage: StorageLayout{
			Root:               t.TempDir(),
			ConfigFileName:     "desktop-config.json",
			SourceProfilesFile: "source-profiles.json",
		},
		ConfigPolicy: ConfigPolicy{
			DefaultSnapshot: func() map[string]any { return map[string]any{"sources": []any{}} },
		},
	})

	saved := service.Invoke("source_profiles.save", `{
		"name":"primary",
		"profile_id":"profile-1",
		"sources":[{"id":"source-1","name":"inline","kind":"inline","content":"1.1.1.1","enabled":true}]
	}`)
	if !saved.OK || saved.Code != "SOURCE_PROFILE_SAVE_OK" {
		t.Fatalf("save result = %#v", saved)
	}
	store, ok := saved.Data.(SourceProfileStore)
	if !ok || store.ActiveProfileID != "profile-1" || len(store.Items) != 1 {
		t.Fatalf("saved store = %#v", saved.Data)
	}
	if store.Items[0].CreatedAt != now.Format(time.RFC3339) || store.SchemaVersion != DefaultSourceProfilesSchemaVersion {
		t.Fatalf("saved profile metadata = %#v", store.Items[0])
	}

	loaded, err := service.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	sources := SourcesFromAny(loaded.Snapshot["sources"])
	if len(sources) != 1 || sources[0].ID != "source-1" {
		t.Fatalf("config sources = %#v", sources)
	}

	updated := service.Invoke("source_profiles.update_current", `{
		"profile_id":"profile-1",
		"sources":[{"id":"source-2","name":"updated","kind":"inline","content":"2.2.2.2","enabled":true}]
	}`)
	if !updated.OK || updated.Code != "SOURCE_PROFILE_UPDATE_OK" {
		t.Fatalf("update result = %#v", updated)
	}

	deleted := service.Invoke("source_profiles.delete", `{"profile_id":"profile-1"}`)
	if !deleted.OK || deleted.Code != "SOURCE_PROFILE_DELETE_OK" {
		t.Fatalf("delete result = %#v", deleted)
	}
	store, ok = deleted.Data.(SourceProfileStore)
	if !ok || store.ActiveProfileID != DefaultSourceProfileID || len(store.Items) != 1 {
		t.Fatalf("deleted store = %#v", deleted.Data)
	}
}

func TestServiceInvokeSourceProfilesUsesTypedPayloadErrors(t *testing.T) {
	service := NewService(ServiceOptions{Storage: StorageLayout{Root: t.TempDir(), ConfigFileName: "config.json", SourceProfilesFile: "profiles.json"}})
	invalid := service.Invoke("source_profiles.save", `{`)
	if invalid.OK || invalid.Code != "SOURCE_PROFILE_INVALID" {
		t.Fatalf("invalid result = %#v", invalid)
	}
	missing := service.Invoke("source_profiles.switch", `{}`)
	if missing.OK || missing.Code != "SOURCE_PROFILE_INVALID" {
		t.Fatalf("missing profile result = %#v", missing)
	}
}
