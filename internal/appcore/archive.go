package appcore

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/archivecore"
	"github.com/axuitomo/CFST-GUI/internal/probecore"
)

type WebDAVConfig = archivecore.WebDAVConfig

const (
	ConfigArchiveEntryName      = archivecore.ConfigArchiveEntryName
	DefaultConfigArchiveName    = archivecore.DefaultConfigArchiveName
	DefaultWebDAVTimeoutSeconds = archivecore.DefaultWebDAVTimeoutSeconds
)

func BuildConfigArchive(snapshot map[string]any, profiles ProfileStore, sourceProfiles SourceProfileStore, storage any, appVersion, schemaVersion string, exportedAt string) ([]byte, map[string]any, error) {
	body := map[string]any{
		"app_version":     appVersion,
		"config_snapshot": snapshot,
		"exported_at":     exportedAt,
		"profiles":        profiles,
		"schema_version":  schemaVersion,
		"source_profiles": sourceProfiles,
		"storage":         storage,
	}
	raw, err := json.MarshalIndent(body, "", "  ")
	if err != nil {
		return nil, nil, err
	}
	archive, err := archivecore.ZipSingleFile(ConfigArchiveEntryName, raw)
	if err != nil {
		return nil, nil, err
	}
	return archive, body, nil
}

func ParseConfigArchive(raw []byte) (map[string]any, error) {
	return archivecore.ParseConfigArchive(raw)
}

func ArchivePayloadBytes(payload map[string]any) ([]byte, string, error) {
	return archivecore.ArchivePayloadBytes(payload)
}

func NormalizeProfileStoreForArchive(store ProfileStore, schemaVersion string, now string) ProfileStore {
	return probecore.NormalizeProfileStoreForArchive(probecore.ArchiveProfileNormalizeOptions[ProfileStore, ProfileItem]{
		Store:         store,
		SchemaVersion: schemaVersion,
		Now:           now,
		Items: func(store ProfileStore) []ProfileItem {
			return store.Items
		},
		SetItems: func(store *ProfileStore, items []ProfileItem) {
			store.Items = items
		},
		ActiveID: func(store ProfileStore) string {
			return store.ActiveProfileID
		},
		SetActiveID: func(store *ProfileStore, id string) {
			store.ActiveProfileID = id
		},
		Schema: func(store ProfileStore) string {
			return store.SchemaVersion
		},
		SetSchema: func(store *ProfileStore, schema string) {
			store.SchemaVersion = schema
		},
		UpdatedAt: func(store ProfileStore) string {
			return store.UpdatedAt
		},
		SetUpdatedAt: func(store *ProfileStore, updatedAt string) {
			store.UpdatedAt = updatedAt
		},
		ItemID: func(item ProfileItem) string {
			return item.ID
		},
		NewItemID: func(index int) string {
			return fmt.Sprintf("profile-%d", time.Now().UnixNano()+int64(index))
		},
		NormalizeItem: func(item *ProfileItem, patch probecore.ArchiveProfileItemPatch) {
			if strings.TrimSpace(item.ID) == "" {
				item.ID = patch.DefaultID
			}
			if strings.TrimSpace(item.Name) == "" {
				item.Name = patch.DefaultName
			}
			if item.ConfigSnapshot == nil {
				item.ConfigSnapshot = map[string]any{}
			}
			if item.CreatedAt == "" {
				item.CreatedAt = patch.Now
			}
			if item.UpdatedAt == "" {
				item.UpdatedAt = patch.Now
			}
		},
	})
}

func ProfilesForArchiveImport(body map[string]any, schemaVersion string, now string) (ProfileStore, bool, error) {
	raw, ok := firstPresent(body, "profiles", "Profiles")
	if !ok {
		return ProfileStore{}, false, nil
	}
	store := ProfileStoreFromAny(raw)
	store = NormalizeProfileStoreForArchive(store, schemaVersion, now)
	return store, true, nil
}

func SourceProfilesForArchiveImport(body map[string]any, snapshot map[string]any, schemaVersion string, defaultSnapshot func() map[string]any, now string) SourceProfileStore {
	raw, ok := firstPresent(body, "source_profiles", "sourceProfiles")
	if !ok {
		return NormalizeSourceProfileStoreForSave(DefaultSourceProfileStoreFromSnapshot(snapshot, defaultSnapshot(), schemaVersion), schemaVersion, now, nil)
	}
	store := SourceProfileStoreFromAny(raw)
	if len(store.Items) == 0 {
		store = DefaultSourceProfileStoreFromSnapshot(snapshot, defaultSnapshot(), schemaVersion)
	}
	return NormalizeSourceProfileStoreForSave(store, schemaVersion, now, nil)
}

func WriteLocalArchiveBackup(root string, snapshot map[string]any, reason string, build func(map[string]any) ([]byte, map[string]any, error)) (string, error) {
	raw, _, err := build(snapshot)
	if err != nil {
		return "", err
	}
	name := fmt.Sprintf("cfst-gui-%s-%s.zip", probecore.SanitizeTemplateFileName(reason), time.Now().Format("20060102-150405"))
	targetPath := filepath.Join(root, "backups", name)
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(targetPath, raw, 0o600); err != nil {
		return "", err
	}
	return targetPath, nil
}
