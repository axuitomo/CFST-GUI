package appcore

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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

func BuildConfigArchive(snapshot map[string]any, sourceProfiles SourceProfileStore, pipelineProfiles PipelineProfileStore, pipelineWorkspace PipelineWorkspace, storage any, appVersion, schemaVersion string, exportedAt string) ([]byte, map[string]any, error) {
	body := map[string]any{
		"app_version":        appVersion,
		"config_snapshot":    snapshot,
		"exported_at":        exportedAt,
		"pipeline_profiles":  pipelineProfiles,
		"pipeline_workspace": pipelineWorkspace,
		"schema_version":     schemaVersion,
		"source_profiles":    sourceProfiles,
		"storage":            storage,
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

func PipelineProfilesForArchiveImport(body map[string]any, snapshot map[string]any, schemaVersion string, defaultSnapshot func() map[string]any, now string, sanitize func(map[string]any) map[string]any) (PipelineProfileStore, bool, error) {
	raw, ok := firstPresent(body, "pipeline_profiles", "pipelineProfiles")
	if !ok {
		return NormalizePipelineProfileStoreForSave(DefaultPipelineProfileStoreFromSnapshot(snapshot, schemaVersion, now, sanitize), schemaVersion, now, sanitize, nil), false, nil
	}
	store := PipelineProfileStoreFromAny(raw)
	if len(store.Items) == 0 {
		defaultValue := snapshot
		if len(defaultValue) == 0 && defaultSnapshot != nil {
			defaultValue = defaultSnapshot()
		}
		store = DefaultPipelineProfileStoreFromSnapshot(defaultValue, schemaVersion, now, sanitize)
	}
	return NormalizePipelineProfileStoreForSave(store, schemaVersion, now, sanitize, nil), true, nil
}

func PipelineWorkspaceForArchiveImport(body map[string]any, snapshot map[string]any, workspaceSchemaVersion string, profileSchemaVersion string, defaultSnapshot func() map[string]any, now string, sanitize func(map[string]any) map[string]any) (PipelineWorkspace, PipelineProfileStore, error) {
	if raw, ok := firstPresent(body, "pipeline_workspace", "pipelineWorkspace"); ok {
		workspace := PipelineWorkspaceFromAny(raw)
		if len(workspace.Templates) > 0 || len(workspace.Targets) > 0 {
			workspace = NormalizePipelineWorkspaceForSave(workspace, workspaceSchemaVersion, now, sanitize, nil, nil)
			return workspace, LegacyPipelineProfileStoreFromWorkspace(workspace, profileSchemaVersion, now, sanitize), nil
		}
	}
	profiles, _, err := PipelineProfilesForArchiveImport(body, snapshot, profileSchemaVersion, defaultSnapshot, now, sanitize)
	if err != nil {
		return PipelineWorkspace{}, PipelineProfileStore{}, err
	}
	workspace := PipelineWorkspaceFromProfileStore(profiles, workspaceSchemaVersion, now, sanitize)
	return workspace, LegacyPipelineProfileStoreFromWorkspace(workspace, profileSchemaVersion, now, sanitize), nil
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
