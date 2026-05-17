package appcore

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/probecore"
)

const (
	DefaultSourceProfileID             = "source-profile-default"
	DefaultProfilesSchemaVersion       = "cfst-gui-profiles-v1"
	DefaultSourceProfilesSchemaVersion = "cfst-gui-source-profiles-v1"
)

func LoadProfileStore(path string, schemaVersion string, sanitize func(map[string]any) map[string]any) (ProfileStore, error) {
	store := ProfileStore{
		Items:         []ProfileItem{},
		SchemaVersion: schemaVersion,
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return store, nil
		}
		return store, err
	}
	if _, err := UnmarshalJSONCompat(raw, &store); err != nil {
		return store, err
	}
	if store.Items == nil {
		store.Items = []ProfileItem{}
	}
	if store.SchemaVersion == "" {
		store.SchemaVersion = schemaVersion
	}
	for index := range store.Items {
		store.Items[index].ConfigSnapshot = sanitize(store.Items[index].ConfigSnapshot)
	}
	return store, nil
}

func SaveProfileStore(path string, store ProfileStore, schemaVersion string, sanitize func(map[string]any) map[string]any) error {
	store.SchemaVersion = schemaVersion
	store.UpdatedAt = time.Now().Format(time.RFC3339)
	if store.Items == nil {
		store.Items = []ProfileItem{}
	}
	for index := range store.Items {
		store.Items[index].ConfigSnapshot = sanitize(store.Items[index].ConfigSnapshot)
	}
	raw, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	return WriteFileAtomic(path, raw, 0o600)
}

func LoadSourceProfileStore(path string, schemaVersion string) (SourceProfileStore, error) {
	store := SourceProfileStore{
		Items:         []SourceProfileItem{},
		SchemaVersion: schemaVersion,
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return store, nil
		}
		return store, err
	}
	if _, err := UnmarshalJSONCompat(raw, &store); err != nil {
		return store, err
	}
	if store.Items == nil {
		store.Items = []SourceProfileItem{}
	}
	if store.SchemaVersion == "" {
		store.SchemaVersion = schemaVersion
	}
	return store, nil
}

func SaveSourceProfileStore(path string, store SourceProfileStore, schemaVersion string) error {
	store.SchemaVersion = schemaVersion
	store.UpdatedAt = time.Now().Format(time.RFC3339)
	if store.Items == nil {
		store.Items = []SourceProfileItem{}
	}
	raw, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	return WriteFileAtomic(path, raw, 0o600)
}

func BlankSourceProfileStore(now string, schemaVersion string) SourceProfileStore {
	now = strings.TrimSpace(now)
	if now == "" {
		now = time.Now().Format(time.RFC3339)
	}
	return SourceProfileStore{
		ActiveProfileID: DefaultSourceProfileID,
		Items: []SourceProfileItem{
			{
				CreatedAt: now,
				ID:        DefaultSourceProfileID,
				Name:      "默认输入源",
				Sources:   []Source{},
				UpdatedAt: now,
			},
		},
		SchemaVersion: schemaVersion,
		UpdatedAt:     now,
	}
}

func DefaultSourceProfileStoreFromSnapshot(snapshot map[string]any, defaultSnapshot map[string]any, schemaVersion string) SourceProfileStore {
	sources := SourcesFromAny(snapshot["sources"])
	if len(sources) == 0 {
		sources = SourcesFromAny(defaultSnapshot["sources"])
	}
	return SourceProfileStore{
		ActiveProfileID: DefaultSourceProfileID,
		Items: []SourceProfileItem{
			{
				ID:      DefaultSourceProfileID,
				Name:    "默认输入源",
				Sources: CloneSources(sources),
			},
		},
		SchemaVersion: schemaVersion,
	}
}

func NormalizeSourceProfileStoreForSave(store SourceProfileStore, schemaVersion string, now string, newProfileID func(index int) string) SourceProfileStore {
	if store.SchemaVersion == "" {
		store.SchemaVersion = schemaVersion
	}
	if strings.TrimSpace(store.UpdatedAt) == "" {
		store.UpdatedAt = now
	}
	if store.Items == nil {
		store.Items = []SourceProfileItem{}
	}
	for index := range store.Items {
		if strings.TrimSpace(store.Items[index].ID) == "" {
			if newProfileID != nil {
				store.Items[index].ID = newProfileID(index)
			}
			if strings.TrimSpace(store.Items[index].ID) == "" {
				store.Items[index].ID = fmt.Sprintf("source-profile-%d", time.Now().UnixNano()+int64(index))
			}
		}
		if strings.TrimSpace(store.Items[index].Name) == "" {
			store.Items[index].Name = fmt.Sprintf("输入源档案 %d", index+1)
		}
		if store.Items[index].Sources == nil {
			store.Items[index].Sources = []Source{}
		}
		if store.Items[index].CreatedAt == "" {
			store.Items[index].CreatedAt = now
		}
		if store.Items[index].UpdatedAt == "" {
			store.Items[index].UpdatedAt = now
		}
		store.Items[index].Sources = CloneSources(store.Items[index].Sources)
	}
	if strings.TrimSpace(store.ActiveProfileID) == "" && len(store.Items) > 0 {
		store.ActiveProfileID = store.Items[0].ID
	}
	if len(store.Items) > 0 {
		found := false
		for _, item := range store.Items {
			if item.ID == store.ActiveProfileID {
				found = true
				break
			}
		}
		if !found {
			store.ActiveProfileID = store.Items[0].ID
		}
	}
	return store
}

func ActiveSourceProfileSources(store SourceProfileStore) []Source {
	for _, item := range store.Items {
		if item.ID == store.ActiveProfileID {
			return CloneSources(item.Sources)
		}
	}
	if len(store.Items) == 0 {
		return []Source{}
	}
	return CloneSources(store.Items[0].Sources)
}

func IsBlankSourceProfilePlaceholder(store SourceProfileStore, defaultProfileID string) bool {
	if store.ActiveProfileID != defaultProfileID || len(store.Items) != 1 {
		return false
	}
	item := store.Items[0]
	return item.ID == defaultProfileID && item.Name == "默认输入源" && len(item.Sources) == 0
}

func SourceProfileStoreFromAny(value any) SourceProfileStore {
	if value == nil {
		return SourceProfileStore{}
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return SourceProfileStore{}
	}
	var store SourceProfileStore
	if err := json.Unmarshal(raw, &store); err != nil {
		return SourceProfileStore{}
	}
	return store
}

func SourcesFromAny(value any) []Source {
	if value == nil {
		return []Source{}
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return []Source{}
	}
	var sources []Source
	if err := json.Unmarshal(raw, &sources); err != nil {
		return []Source{}
	}
	if sources == nil {
		return []Source{}
	}
	return sources
}

func CloneSources(sources []Source) []Source {
	if sources == nil {
		return []Source{}
	}
	cloned := make([]Source, len(sources))
	copy(cloned, sources)
	return cloned
}

func ProfileStoreFromAny(value any) ProfileStore {
	if value == nil {
		return ProfileStore{}
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return ProfileStore{}
	}
	var store ProfileStore
	if err := json.Unmarshal(raw, &store); err != nil {
		return ProfileStore{}
	}
	return store
}

func UpdateCurrentProfileStore[TValue any](store ProfileStore, value TValue, profileID, name, now, defaultName string, forceNewID func(string) bool, newProfileID func() string, dropPlaceholder func(ProfileStore, string) bool, updateItem func(*ProfileItem, probecore.ProfileItemPatch[TValue]), newItem func(probecore.ProfileItemPatch[TValue]) ProfileItem) ProfileStore {
	return updateCurrentProfileStore(store, value, profileID, name, now, defaultName, forceNewID, newProfileID, dropPlaceholder, updateItem, newItem)
}

func updateCurrentProfileStore[TValue any](store ProfileStore, value TValue, profileID, name, now, defaultName string, forceNewID func(string) bool, newProfileID func() string, dropPlaceholder func(ProfileStore, string) bool, updateItem func(*ProfileItem, probecore.ProfileItemPatch[TValue]), newItem func(probecore.ProfileItemPatch[TValue]) ProfileItem) ProfileStore {
	pid := strings.TrimSpace(profileID)
	if pid == "" {
		pid = strings.TrimSpace(store.ActiveProfileID)
	}
	if dropPlaceholder != nil && dropPlaceholder(store, pid) {
		store.Items = []ProfileItem{}
	}
	items := store.Items
	if pid != "" {
		for index := range items {
			if strings.TrimSpace(items[index].ID) != pid {
				continue
			}
			if updateItem != nil {
				updateItem(&items[index], probecore.ProfileItemPatch[TValue]{
					ID:    pid,
					Name:  name,
					Now:   now,
					Value: value,
				})
			}
			store.Items = items
			store.ActiveProfileID = pid
			return store
		}
	}
	if pid == "" || (forceNewID != nil && forceNewID(pid)) {
		if newProfileID != nil {
			pid = strings.TrimSpace(newProfileID())
		}
	}
	if pid == "" {
		pid = "profile-current"
	}
	if strings.TrimSpace(name) == "" {
		name = defaultName
	}
	items = append(items, newItem(probecore.ProfileItemPatch[TValue]{
		ID:    pid,
		Name:  name,
		Now:   now,
		Value: value,
	}))
	store.Items = items
	store.ActiveProfileID = pid
	return store
}
