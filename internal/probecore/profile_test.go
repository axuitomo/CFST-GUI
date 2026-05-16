package probecore

import (
	"testing"
	"time"
)

type profileTestItem struct {
	CreatedAt string
	ID        string
	Name      string
	UpdatedAt string
	Value     string
}

type profileTestStore struct {
	ActiveProfileID string
	Items           []profileTestItem
	SchemaVersion   string
	UpdatedAt       string
}

func TestUpdateCurrentProfileStoreUpdatesActiveOrCreatesMissing(t *testing.T) {
	store := profileTestStore{
		ActiveProfileID: "active",
		Items: []profileTestItem{
			{CreatedAt: "old", ID: "active", Name: "Active", UpdatedAt: "old", Value: "old-value"},
		},
	}

	store, action := updateProfileTestStore(store, "", "", "new-value")
	if action != "updated" {
		t.Fatalf("action = %q, want updated", action)
	}
	if len(store.Items) != 1 || store.Items[0].Value != "new-value" || store.Items[0].Name != "Active" {
		t.Fatalf("updated store = %#v, want active item overwritten without blanking name", store)
	}

	store.ActiveProfileID = "missing"
	store, action = updateProfileTestStore(store, "", "Missing", "missing-value")
	if action != "created" {
		t.Fatalf("action = %q, want created", action)
	}
	if store.ActiveProfileID != "missing" || len(store.Items) != 2 || store.Items[1].Value != "missing-value" {
		t.Fatalf("missing-active store = %#v, want new active item with missing id", store)
	}
}

func updateProfileTestStore(store profileTestStore, profileID, name, value string) (profileTestStore, string) {
	return UpdateCurrentProfileStore(CurrentProfileUpdateOptions[profileTestStore, profileTestItem, string]{
		Store:       store,
		Value:       value,
		ProfileID:   profileID,
		Name:        name,
		Now:         "now",
		DefaultName: "Current",
		Items: func(store profileTestStore) []profileTestItem {
			return store.Items
		},
		SetItems: func(store *profileTestStore, items []profileTestItem) {
			store.Items = items
		},
		ActiveID: func(store profileTestStore) string {
			return store.ActiveProfileID
		},
		SetActiveID: func(store *profileTestStore, profileID string) {
			store.ActiveProfileID = profileID
		},
		ItemID: func(item profileTestItem) string {
			return item.ID
		},
		UpdateItem: func(item *profileTestItem, patch ProfileItemPatch[string]) {
			item.Value = patch.Value
			if patch.Name != "" {
				item.Name = patch.Name
			}
			if item.Name == "" {
				item.Name = "Current"
			}
			if item.CreatedAt == "" {
				item.CreatedAt = patch.Now
			}
			item.UpdatedAt = patch.Now
		},
		NewItem: func(patch ProfileItemPatch[string]) profileTestItem {
			return profileTestItem{
				CreatedAt: patch.Now,
				ID:        patch.ID,
				Name:      patch.Name,
				UpdatedAt: patch.Now,
				Value:     patch.Value,
			}
		},
		NewProfileID: func() string {
			return "generated"
		},
	})
}

func TestNormalizeProfileStoreForArchiveFillsDefaultsAndActiveID(t *testing.T) {
	store := profileTestStore{
		ActiveProfileID: "missing",
		Items: []profileTestItem{
			{Value: "one"},
			{ID: "existing", Name: "Existing", CreatedAt: "created", Value: "two"},
		},
	}

	normalized := NormalizeProfileStoreForArchive(ArchiveProfileNormalizeOptions[profileTestStore, profileTestItem]{
		Store:         store,
		SchemaVersion: "schema-v1",
		Now:           "now",
		Items: func(store profileTestStore) []profileTestItem {
			return store.Items
		},
		SetItems: func(store *profileTestStore, items []profileTestItem) {
			store.Items = items
		},
		ActiveID: func(store profileTestStore) string {
			return store.ActiveProfileID
		},
		SetActiveID: func(store *profileTestStore, id string) {
			store.ActiveProfileID = id
		},
		Schema: func(store profileTestStore) string {
			return store.SchemaVersion
		},
		SetSchema: func(store *profileTestStore, schema string) {
			store.SchemaVersion = schema
		},
		UpdatedAt: func(store profileTestStore) string {
			return store.UpdatedAt
		},
		SetUpdatedAt: func(store *profileTestStore, updatedAt string) {
			store.UpdatedAt = updatedAt
		},
		ItemID: func(item profileTestItem) string {
			return item.ID
		},
		NewItemID: func(index int) string {
			return "generated-id"
		},
		NormalizeItem: func(item *profileTestItem, patch ArchiveProfileItemPatch) {
			if item.ID == "" {
				item.ID = patch.DefaultID
			}
			if item.Name == "" {
				item.Name = patch.DefaultName
			}
			if item.CreatedAt == "" {
				item.CreatedAt = patch.Now
			}
			if item.UpdatedAt == "" {
				item.UpdatedAt = patch.Now
			}
		},
	})

	if normalized.SchemaVersion != "schema-v1" || normalized.UpdatedAt != "now" {
		t.Fatalf("store metadata = %#v", normalized)
	}
	if normalized.Items[0].ID != "generated-id" || normalized.Items[0].Name != "配置档案 1" {
		t.Fatalf("first item = %#v", normalized.Items[0])
	}
	if normalized.Items[1].CreatedAt != "created" || normalized.Items[1].UpdatedAt != "now" {
		t.Fatalf("second item timestamps = %#v", normalized.Items[1])
	}
	if normalized.ActiveProfileID != "generated-id" {
		t.Fatalf("ActiveProfileID = %q, want first item after missing active id", normalized.ActiveProfileID)
	}
}

func TestRenderExportFileTemplateSanitizesFileName(t *testing.T) {
	got := RenderExportFileTemplate("result-{date}-{time}-{task_id}-{profile}.csv", "task/1", "A:B", time.Date(2026, 5, 2, 3, 4, 5, 0, time.UTC))
	want := "result-20260502-030405-task_1-A_B.csv"
	if got != want {
		t.Fatalf("RenderExportFileTemplate = %q, want %q", got, want)
	}
}
