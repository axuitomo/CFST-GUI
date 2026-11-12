package appcore

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/axuitomo/CFST-GUI/internal/probecore"
)

func TestSourceGroupModePreservesLegacySources(t *testing.T) {
	mixed := []Source{{IPMode: "mcis"}, {IPMode: "traverse"}}
	if got := sourceGroupIPMode("", mixed); got != "" {
		t.Fatalf("mixed legacy mode = %q", got)
	}
	if got := sourceGroupIPMode("", mixed[:1]); got != "mcis" {
		t.Fatalf("uniform legacy mode = %q", got)
	}
	if got := sourceGroupIPMode("", nil); got != "traverse" {
		t.Fatalf("new empty group mode = %q", got)
	}
	updated := sourcesWithGroupIPMode(mixed, "traverse")
	if mixed[0].IPMode != "mcis" || updated[0].IPMode != "traverse" {
		t.Fatalf("group mode must override a copy: original=%#v updated=%#v", mixed, updated)
	}
	if warnings := sourceGroupWarnings(map[string]any{}, mixed); len(warnings) != 1 {
		t.Fatalf("mixed legacy warnings = %#v", warnings)
	}
}

func TestSourceProfileGroupModePersistsAndSwitches(t *testing.T) {
	service := NewService(ServiceOptions{
		Storage:      StorageLayout{Root: t.TempDir(), ConfigFileName: "config.json", SourceProfilesFile: "profiles.json"},
		ConfigPolicy: ConfigPolicy{DefaultSnapshot: func() map[string]any { return map[string]any{"sources": []Source{}} }},
	})
	mcis := "mcis"
	traverse := "traverse"
	saved := service.SaveSourceProfile(SourceProfileSaveRequest{
		Name: "random", ProfileID: "random", IPMode: &mcis,
		Sources: []Source{{ID: "one", Kind: "inline", Content: "1.1.1.1", Enabled: true, IPMode: "traverse"}},
	})
	if !saved.OK {
		t.Fatal(saved.Message)
	}
	assertGroup := func(id, mode string, count int) {
		t.Helper()
		loaded, err := service.LoadConfig()
		if err != nil {
			t.Fatal(err)
		}
		sources := SourcesFromAny(loaded.Snapshot["sources"])
		if loaded.Snapshot["source_ip_mode"] != mode || len(sources) != count {
			t.Fatalf("snapshot = %#v", loaded.Snapshot)
		}
		for _, source := range sources {
			if source.IPMode != mode {
				t.Fatalf("source mode = %q, want %q", source.IPMode, mode)
			}
		}
		store := service.LoadSourceProfiles().Data.(SourceProfileStore)
		if store.ActiveProfileID != id || activeSourceProfileIPMode(store) != mode {
			t.Fatalf("active profile = %#v", store)
		}
	}
	assertGroup("random", "mcis", 1)
	// Older callers omit the group mode and still inherit the saved mode.
	updated := service.UpdateCurrentSourceProfile(SourceProfileUpdateRequest{
		ProfileID: "random", Sources: []Source{{ID: "two", Kind: "inline", Content: "1.0.0.1", IPMode: "traverse"}},
	})
	if !updated.OK {
		t.Fatal(updated.Message)
	}
	assertGroup("random", "mcis", 1)
	if result := service.SaveSourceProfile(SourceProfileSaveRequest{Name: "empty", ProfileID: "empty", IPMode: &traverse, Sources: []Source{}}); !result.OK {
		t.Fatal(result.Message)
	}
	assertGroup("empty", "traverse", 0)
	if result := service.SwitchSourceProfile(SourceProfileSelectRequest{ProfileID: "random"}); !result.OK {
		t.Fatal(result.Message)
	}
	assertGroup("random", "mcis", 1)
	if result := service.DeleteSourceProfile(SourceProfileSelectRequest{ProfileID: "random"}); !result.OK {
		t.Fatal(result.Message)
	}
	assertGroup("empty", "traverse", 0)
	// Empty groups retain an explicit mode across save and reload.
	if result := service.UpdateCurrentSourceProfile(SourceProfileUpdateRequest{ProfileID: "empty", IPMode: &mcis, Sources: []Source{}}); !result.OK {
		t.Fatal(result.Message)
	}
	assertGroup("empty", "mcis", 0)
}

func TestSourceGroupModeOverridesProbeAndPreview(t *testing.T) {
	service := newSourceServiceForTest(t, ServiceOptions{})
	original := Source{ID: "one", Kind: "inline", Content: "1.1.1.1", Enabled: true, IPMode: "mcis"}
	config := map[string]any{"source_ip_mode": "traverse"}
	payload, _, _, _, err := service.prepareProbePayload(ProbePayload{Config: config, Sources: []Source{original}})
	if err != nil {
		t.Fatal(err)
	}
	if payload.Sources[0].IPMode != "traverse" || original.IPMode != "mcis" {
		t.Fatalf("probe sources = %#v", payload.Sources)
	}
	preview := service.inspectSource(SourcePreviewRequest{Config: config, Source: original}, false)
	if !preview.OK {
		t.Fatal(preview.Message)
	}
	if summary := mapValue(mapValue(preview.Data)["summary"]); summary["mode"] != "traverse" || summary["total_count"] != 1 {
		t.Fatalf("preview summary = %#v", summary)
	}
	mixed := []Source{original, {ID: "two", IPMode: "traverse"}}
	legacy, _, warnings, _, err := service.prepareProbePayload(ProbePayload{Config: map[string]any{"sources": mixed}, Sources: mixed})
	if err != nil || legacy.Sources[0].IPMode != "mcis" || len(warnings) == 0 {
		t.Fatalf("legacy sources=%#v warnings=%#v err=%v", legacy.Sources, warnings, err)
	}
}

func TestSourceGroupModeSanitizationAndArchive(t *testing.T) {
	defaults := probecore.DefaultConfigSnapshot(probecore.ConfigSnapshotOptions{})
	if len(SourcesFromAny(defaults["sources"])) != 0 {
		t.Fatal("new installations must choose a source type before creating a source")
	}
	blankDraft := probecore.SanitizeConfigSnapshot(map[string]any{"sources": []map[string]any{{"kind": "url", "url": ""}}}, probecore.ConfigSnapshotOptions{})
	if len(SourcesFromAny(blankDraft["sources"])) != 1 {
		t.Fatal("existing blank source drafts must be retained")
	}
	mixed := []map[string]any{{"ip_mode": "mcis"}, {"ip_mode": "traverse"}}
	legacy := probecore.SanitizeConfigSnapshot(map[string]any{"sources": mixed}, probecore.ConfigSnapshotOptions{})
	if legacy["source_ip_mode"] != "" || SourcesFromAny(legacy["sources"])[0].IPMode != "mcis" {
		t.Fatalf("legacy snapshot = %#v", legacy)
	}
	unified := probecore.SanitizeConfigSnapshot(map[string]any{"source_ip_mode": "traverse", "sources": mixed}, probecore.ConfigSnapshotOptions{})
	for _, source := range SourcesFromAny(unified["sources"]) {
		if source.IPMode != "traverse" {
			t.Fatalf("unified source = %#v", source)
		}
	}
	store := DefaultSourceProfileStoreFromSnapshot(map[string]any{"source_ip_mode": "mcis", "sources": []Source{}}, nil, DefaultSourceProfilesSchemaVersion)
	store = NormalizeSourceProfileStoreForSave(store, DefaultSourceProfilesSchemaVersion, "now", nil)
	if store.Items[0].IPMode != "mcis" {
		t.Fatalf("archive empty group mode = %#v", store)
	}
}

func TestLoadSourceProfileStoreDeduplicatesIDsButPreservesNames(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profiles.json")
	data := `{"active_profile_id":"source-profile-recent-run","items":[{"id":"source-profile-recent-run","name":"最近运行输入源","sources":[{"id":"latest"}]},{"id":"source-profile-recent-run","name":"最近运行输入源","sources":[{"id":"stale"}]},{"id":"named-copy","name":"最近运行输入源","sources":[]}]}`
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}

	store, err := LoadSourceProfileStore(path, DefaultSourceProfilesSchemaVersion)
	if err != nil {
		t.Fatal(err)
	}
	if len(store.Items) != 2 || store.Items[0].Sources[0].ID != "latest" {
		t.Fatalf("source profiles = %#v, want first recent item plus distinct ID", store.Items)
	}
}

func TestMissingSourceProfileStoreInheritsCurrentGroupSettings(t *testing.T) {
	service := NewService(ServiceOptions{
		Storage:      StorageLayout{Root: t.TempDir(), ConfigFileName: "config.json", SourceProfilesFile: "profiles.json"},
		ConfigPolicy: ConfigPolicy{DefaultSnapshot: func() map[string]any { return map[string]any{"sources": []Source{}} }},
	})
	if _, err := service.SaveConfig(map[string]any{"sources": []Source{{
		ID: "legacy", Kind: "inline", Content: "1.1.1.1", Enabled: true,
		ColoFilter: "JP,HKG", ColoFilterMode: "deny",
	}}}); err != nil {
		t.Fatal(err)
	}

	store := service.LoadSourceProfiles().Data.(SourceProfileStore)
	profile := store.Items[0]
	if profile.ColoFilter != "JP,HKG" || profile.ColoFilterMode != "deny" {
		t.Fatalf("migrated group filter = %q/%q", profile.ColoFilter, profile.ColoFilterMode)
	}
	if len(profile.Sources) != 1 || profile.Sources[0].ColoFilter != "JP,HKG" {
		t.Fatalf("migrated group sources = %#v", profile.Sources)
	}
}

func TestSaveSourceProfileUsesGroupColoFilterForAllSources(t *testing.T) {
	service := NewService(ServiceOptions{
		Storage:      StorageLayout{Root: t.TempDir(), ConfigFileName: "config.json", SourceProfilesFile: "profiles.json"},
		ConfigPolicy: ConfigPolicy{DefaultSnapshot: func() map[string]any { return map[string]any{"sources": []Source{}} }},
	})
	result := service.SaveSourceProfile(SourceProfileSaveRequest{
		Name: "group", ProfileID: "group", ColoFilter: "JP,HKG", ColoFilterMode: "deny",
		Sources: []Source{
			{ID: "one", ColoFilter: "US", ColoFilterMode: "allow"},
			{ID: "two", ColoFilter: "EU", ColoFilterMode: "deny"},
		},
	})
	if !result.OK {
		t.Fatal(result.Message)
	}
	store := result.Data.(SourceProfileStore)
	profile := store.Items[0]
	if profile.ColoFilter != "JP,HKG" || profile.ColoFilterMode != "deny" {
		t.Fatalf("group filter = %q/%q", profile.ColoFilter, profile.ColoFilterMode)
	}
	for _, source := range profile.Sources {
		if source.ColoFilter != profile.ColoFilter || source.ColoFilterMode != profile.ColoFilterMode {
			t.Fatalf("source filter = %q/%q, want group filter", source.ColoFilter, source.ColoFilterMode)
		}
	}
}
