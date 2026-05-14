package mobileapi

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/archivecore"
	"github.com/axuitomo/CFST-GUI/internal/probecore"
)

const (
	configArchiveEntryName      = archivecore.ConfigArchiveEntryName
	defaultConfigArchiveName    = archivecore.DefaultConfigArchiveName
	defaultWebDAVTimeoutSeconds = archivecore.DefaultWebDAVTimeoutSeconds
)

type mobileWebDAVConfig = archivecore.WebDAVConfig

func (s *Service) ExportConfigArchive(payloadJSON string) string {
	payload, err := decodeObject(payloadJSON)
	if err != nil {
		return encodeCommand(commandResultFor("CONFIG_ARCHIVE_EXPORT_INVALID", nil, err.Error(), false, nil, nil))
	}
	snapshot, err := s.mobileSnapshotForArchive(payload)
	if err != nil {
		return encodeCommand(commandResultFor("CONFIG_ARCHIVE_READ_FAILED", nil, err.Error(), false, nil, nil))
	}
	raw, _, err := s.buildMobileConfigArchive(snapshot)
	if err != nil {
		return encodeCommand(commandResultFor("CONFIG_ARCHIVE_BUILD_FAILED", nil, err.Error(), false, nil, nil))
	}
	targetPath := strings.TrimSpace(stringValue(firstNonNil(payload["target_path"], payload["targetPath"], payload["path"]), ""))
	if targetPath != "" {
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return encodeCommand(commandResultFor("CONFIG_ARCHIVE_WRITE_FAILED", nil, err.Error(), false, nil, nil))
		}
		if err := os.WriteFile(targetPath, raw, 0o600); err != nil {
			return encodeCommand(commandResultFor("CONFIG_ARCHIVE_WRITE_FAILED", nil, err.Error(), false, nil, nil))
		}
	}
	return encodeCommand(commandResultFor("CONFIG_ARCHIVE_EXPORT_OK", map[string]any{
		"content_base64": base64.StdEncoding.EncodeToString(raw),
		"file_name":      defaultConfigArchiveName,
		"path":           targetPath,
	}, "配置压缩包已导出。", true, nil, sensitiveMobileArchiveWarnings()))
}

func (s *Service) ImportConfigArchive(payloadJSON string) string {
	payload, err := decodeObject(payloadJSON)
	if err != nil {
		return encodeCommand(commandResultFor("CONFIG_ARCHIVE_IMPORT_INVALID", nil, err.Error(), false, nil, nil))
	}
	return s.importMobileConfigArchivePayload(payload, "配置压缩包已导入。")
}

func (s *Service) TestWebDAV(payloadJSON string) string {
	payload, err := decodeObject(payloadJSON)
	if err != nil {
		return encodeCommand(commandResultFor("WEBDAV_INVALID", nil, err.Error(), false, nil, nil))
	}
	cfg, err := s.mobileWebDAVConfigFromPayload(payload)
	if err != nil {
		return encodeCommand(commandResultFor("WEBDAV_INVALID", nil, err.Error(), false, nil, nil))
	}
	targetURL, err := mobileWebDAVTargetURL(cfg)
	if err != nil {
		return encodeCommand(commandResultFor("WEBDAV_INVALID", nil, err.Error(), false, nil, nil))
	}
	status, body, err := mobileWebDAVRequest(cfg, http.MethodHead, targetURL, nil)
	if err != nil {
		return encodeCommand(commandResultFor("WEBDAV_TEST_FAILED", nil, err.Error(), false, nil, nil))
	}
	ok := (status >= 200 && status < 400) || status == http.StatusNotFound
	if !ok {
		return encodeCommand(commandResultFor("WEBDAV_TEST_FAILED", map[string]any{
			"status":     status,
			"target_url": targetURL,
		}, webDAVHTTPErrorMessage("WebDAV 测试失败", status, body), false, nil, nil))
	}
	message := "WebDAV 连接可用。"
	if status == http.StatusNotFound {
		message = "WebDAV 连接可用，远端配置包尚不存在。"
	}
	return encodeCommand(commandResultFor("WEBDAV_TEST_OK", map[string]any{
		"remote_path": cfg.RemotePath,
		"status":      status,
		"target_url":  targetURL,
	}, message, true, nil, nil))
}

func (s *Service) BackupConfigToWebDAV(payloadJSON string) string {
	payload, err := decodeObject(payloadJSON)
	if err != nil {
		return encodeCommand(commandResultFor("WEBDAV_INVALID", nil, err.Error(), false, nil, nil))
	}
	cfg, err := s.mobileWebDAVConfigFromPayload(payload)
	if err != nil {
		return encodeCommand(commandResultFor("WEBDAV_INVALID", nil, err.Error(), false, nil, nil))
	}
	snapshot, err := s.mobileSnapshotForArchive(payload)
	if err != nil {
		return encodeCommand(commandResultFor("WEBDAV_BACKUP_READ_FAILED", nil, err.Error(), false, nil, nil))
	}
	snapshot = setMobileWebDAVTimestamp(snapshot, "last_backup_at", nowRFC3339())
	raw, _, err := s.buildMobileConfigArchive(snapshot)
	if err != nil {
		return encodeCommand(commandResultFor("WEBDAV_BACKUP_BUILD_FAILED", nil, err.Error(), false, nil, nil))
	}
	targetURL, err := mobileWebDAVTargetURL(cfg)
	if err != nil {
		return encodeCommand(commandResultFor("WEBDAV_INVALID", nil, err.Error(), false, nil, nil))
	}
	status, body, err := mobileWebDAVRequest(cfg, http.MethodPut, targetURL, raw)
	if err != nil {
		return encodeCommand(commandResultFor("WEBDAV_BACKUP_FAILED", nil, err.Error(), false, nil, nil))
	}
	if status < 200 || status >= 300 {
		return encodeCommand(commandResultFor("WEBDAV_BACKUP_FAILED", map[string]any{
			"status":     status,
			"target_url": targetURL,
		}, webDAVHTTPErrorMessage("WebDAV 备份失败", status, body), false, nil, nil))
	}
	_ = s.writeConfigSnapshot(snapshot)
	return encodeCommand(commandResultFor("WEBDAV_BACKUP_OK", map[string]any{
		"remote_path": cfg.RemotePath,
		"status":      status,
		"target_url":  targetURL,
	}, "配置压缩包已备份到 WebDAV。", true, nil, sensitiveMobileArchiveWarnings()))
}

func (s *Service) RestoreConfigFromWebDAV(payloadJSON string) string {
	payload, err := decodeObject(payloadJSON)
	if err != nil {
		return encodeCommand(commandResultFor("WEBDAV_INVALID", nil, err.Error(), false, nil, nil))
	}
	cfg, err := s.mobileWebDAVConfigFromPayload(payload)
	if err != nil {
		return encodeCommand(commandResultFor("WEBDAV_INVALID", nil, err.Error(), false, nil, nil))
	}
	targetURL, err := mobileWebDAVTargetURL(cfg)
	if err != nil {
		return encodeCommand(commandResultFor("WEBDAV_INVALID", nil, err.Error(), false, nil, nil))
	}
	status, body, err := mobileWebDAVRequest(cfg, http.MethodGet, targetURL, nil)
	if err != nil {
		return encodeCommand(commandResultFor("WEBDAV_RESTORE_FAILED", nil, err.Error(), false, nil, nil))
	}
	if status < 200 || status >= 300 {
		return encodeCommand(commandResultFor("WEBDAV_RESTORE_FAILED", map[string]any{
			"status":     status,
			"target_url": targetURL,
		}, webDAVHTTPErrorMessage("WebDAV 还原失败", status, body), false, nil, nil))
	}
	payload["content_base64"] = base64.StdEncoding.EncodeToString(body)
	payload["restored_at"] = nowRFC3339()
	encoded := s.importMobileConfigArchivePayload(payload, "已从 WebDAV 还原配置压缩包。")
	result, err := decodeObject(encoded)
	if err != nil {
		return encoded
	}
	if boolValue(result["ok"], false) {
		data := mapValue(result["data"])
		data["remote_path"] = cfg.RemotePath
		data["target_url"] = targetURL
		result["data"] = data
		return encodeCommand(commandResultFor(
			stringValue(result["code"], "CONFIG_ARCHIVE_IMPORT_OK"),
			data,
			stringValue(result["message"], "已从 WebDAV 还原配置压缩包。"),
			true,
			nil,
			stringSliceValue(result["warnings"]),
		))
	}
	return encoded
}

func (s *Service) importMobileConfigArchivePayload(payload map[string]any, successMessage string) string {
	raw, sourceName, err := mobileArchivePayloadBytes(payload)
	if err != nil {
		return encodeCommand(commandResultFor("CONFIG_ARCHIVE_IMPORT_READ_FAILED", nil, err.Error(), false, nil, nil))
	}
	body, err := parseMobileConfigArchive(raw)
	if err != nil {
		return encodeCommand(commandResultFor("CONFIG_ARCHIVE_IMPORT_PARSE_FAILED", nil, err.Error(), false, nil, nil))
	}
	current := mapValue(firstNonNil(payload["current_config_snapshot"], payload["currentConfigSnapshot"], payload["backup_config_snapshot"], payload["backupConfigSnapshot"]))
	if len(current) == 0 {
		current, _ = s.loadConfigSnapshotFromDisk()
	} else {
		current = sanitizeMobileConfigSnapshot(current)
	}
	backupPath := ""
	if len(current) > 0 {
		if path, err := s.writeMobileLocalArchiveBackup(current, "pre-import"); err == nil {
			backupPath = path
		} else {
			return encodeCommand(commandResultFor("CONFIG_ARCHIVE_IMPORT_BACKUP_FAILED", nil, err.Error(), false, nil, nil))
		}
	}
	snapshot := mapValue(firstNonNil(body["config_snapshot"], body["configSnapshot"]))
	if len(snapshot) == 0 {
		snapshot = body
	}
	snapshot = sanitizeMobileConfigSnapshot(snapshot)
	profiles, profilesPresent := s.mobileProfilesForImport(body)
	sourceProfiles := s.mobileSourceProfilesForImport(body, snapshot)
	if restoredAt := strings.TrimSpace(stringValue(payload["restored_at"], "")); restoredAt != "" {
		snapshot = setMobileWebDAVTimestamp(snapshot, "last_restore_at", restoredAt)
	}
	if err := s.writeConfigSnapshot(snapshot); err != nil {
		return encodeCommand(commandResultFor("CONFIG_ARCHIVE_IMPORT_WRITE_FAILED", nil, err.Error(), false, nil, nil))
	}
	if profilesPresent {
		if err := s.saveProfileStore(profiles); err != nil {
			return encodeCommand(commandResultFor("CONFIG_ARCHIVE_IMPORT_PROFILE_SAVE_FAILED", nil, err.Error(), false, nil, nil))
		}
	}
	if err := s.saveSourceProfileStore(sourceProfiles); err != nil {
		return encodeCommand(commandResultFor("CONFIG_ARCHIVE_IMPORT_SOURCE_PROFILE_SAVE_FAILED", nil, err.Error(), false, nil, nil))
	}
	if !profilesPresent {
		loadedProfiles, err := s.loadProfileStore()
		if err == nil {
			profiles = loadedProfiles
		}
	}
	return encodeCommand(commandResultFor("CONFIG_ARCHIVE_IMPORT_OK", map[string]any{
		"backup_path":     backupPath,
		"configPath":      s.configPath(),
		"config_snapshot": snapshot,
		"file_name":       sourceName,
		"profiles":        profiles,
		"source_profiles": sourceProfiles,
		"storage":         s.storageStatus(),
	}, successMessage, true, nil, sensitiveMobileArchiveWarnings()))
}

func (s *Service) mobileSnapshotForArchive(payload map[string]any) (map[string]any, error) {
	snapshot := mapValue(firstNonNil(payload["config_snapshot"], payload["configSnapshot"]))
	if len(snapshot) > 0 {
		return sanitizeMobileConfigSnapshot(snapshot), nil
	}
	return s.loadConfigSnapshotFromDisk()
}

func (s *Service) buildMobileConfigArchive(snapshot map[string]any) ([]byte, map[string]any, error) {
	snapshot = sanitizeMobileConfigSnapshot(snapshot)
	profiles, err := s.loadProfileStore()
	if err != nil {
		return nil, nil, err
	}
	sourceProfiles, err := s.loadSourceProfileStoreForSnapshot(snapshot)
	if err != nil {
		return nil, nil, err
	}
	body := map[string]any{
		"app_version":     "mobile",
		"config_snapshot": snapshot,
		"exported_at":     nowRFC3339(),
		"profiles":        profiles,
		"schema_version":  schemaVersion,
		"source_profiles": sourceProfiles,
		"storage":         s.storageStatus(),
	}
	raw, err := json.MarshalIndent(body, "", "  ")
	if err != nil {
		return nil, nil, err
	}
	archive, err := zipMobileSingleFile(configArchiveEntryName, raw)
	if err != nil {
		return nil, nil, err
	}
	return archive, body, nil
}

func zipMobileSingleFile(name string, raw []byte) ([]byte, error) {
	return archivecore.ZipSingleFile(name, raw)
}

func parseMobileConfigArchive(raw []byte) (map[string]any, error) {
	return archivecore.ParseConfigArchive(raw)
}

func mobileArchivePayloadBytes(payload map[string]any) ([]byte, string, error) {
	return archivecore.ArchivePayloadBytes(payload)
}

func (s *Service) writeMobileLocalArchiveBackup(snapshot map[string]any, reason string) (string, error) {
	raw, _, err := s.buildMobileConfigArchive(snapshot)
	if err != nil {
		return "", err
	}
	name := fmt.Sprintf("cfst-gui-%s-%s.zip", sanitizeTemplateFileName(reason), time.Now().Format("20060102-150405"))
	targetPath := filepath.Join(s.basePath(), "backups", name)
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(targetPath, raw, 0o600); err != nil {
		return "", err
	}
	return targetPath, nil
}

func (s *Service) mobileProfilesForImport(body map[string]any) (mobileProfileStore, bool) {
	raw, ok := firstMobilePresent(body, "profiles", "Profiles")
	if !ok {
		return mobileProfileStore{}, false
	}
	store := mobileProfileStoreFromArchive(raw)
	return normalizeMobileProfileStoreForArchive(store), true
}

func (s *Service) mobileSourceProfilesForImport(body map[string]any, snapshot map[string]any) mobileSourceProfileStore {
	raw, ok := firstMobilePresent(body, "source_profiles", "sourceProfiles")
	if !ok {
		return normalizeMobileSourceProfileStoreForSave(defaultMobileSourceProfileStoreFromSnapshot(snapshot))
	}
	store := mobileSourceProfileStoreFromAny(raw)
	if len(store.Items) == 0 {
		store = defaultMobileSourceProfileStoreFromSnapshot(snapshot)
	}
	return normalizeMobileSourceProfileStoreForSave(store)
}

func mobileProfileStoreFromArchive(value any) mobileProfileStore {
	raw, err := json.Marshal(value)
	if err != nil {
		return mobileProfileStore{}
	}
	var store mobileProfileStore
	if err := json.Unmarshal(raw, &store); err != nil {
		return mobileProfileStore{}
	}
	return store
}

func normalizeMobileProfileStoreForArchive(store mobileProfileStore) mobileProfileStore {
	now := nowRFC3339()
	return probecore.NormalizeProfileStoreForArchive(probecore.ArchiveProfileNormalizeOptions[mobileProfileStore, mobileProfileItem]{
		Store:         store,
		SchemaVersion: profilesSchemaVersion,
		Now:           now,
		Items: func(store mobileProfileStore) []mobileProfileItem {
			return store.Items
		},
		SetItems: func(store *mobileProfileStore, items []mobileProfileItem) {
			store.Items = items
		},
		ActiveID: func(store mobileProfileStore) string {
			return store.ActiveProfileID
		},
		SetActiveID: func(store *mobileProfileStore, id string) {
			store.ActiveProfileID = id
		},
		Schema: func(store mobileProfileStore) string {
			return store.SchemaVersion
		},
		SetSchema: func(store *mobileProfileStore, schema string) {
			store.SchemaVersion = schema
		},
		UpdatedAt: func(store mobileProfileStore) string {
			return store.UpdatedAt
		},
		SetUpdatedAt: func(store *mobileProfileStore, updatedAt string) {
			store.UpdatedAt = updatedAt
		},
		ItemID: func(item mobileProfileItem) string {
			return item.ID
		},
		NewItemID: func(index int) string {
			return fmt.Sprintf("profile-%d", time.Now().UnixNano()+int64(index))
		},
		NormalizeItem: func(item *mobileProfileItem, patch probecore.ArchiveProfileItemPatch) {
			if strings.TrimSpace(item.ID) == "" {
				item.ID = patch.DefaultID
			}
			if strings.TrimSpace(item.Name) == "" {
				item.Name = patch.DefaultName
			}
			if item.ConfigSnapshot == nil {
				item.ConfigSnapshot = map[string]any{}
			}
			item.ConfigSnapshot = sanitizeMobileConfigSnapshot(item.ConfigSnapshot)
			if item.CreatedAt == "" {
				item.CreatedAt = patch.Now
			}
			if item.UpdatedAt == "" {
				item.UpdatedAt = patch.Now
			}
		},
	})
}

func firstMobilePresent(source map[string]any, keys ...string) (any, bool) {
	for _, key := range keys {
		if value, ok := source[key]; ok && value != nil {
			return value, true
		}
	}
	return nil, false
}

func (s *Service) mobileWebDAVConfigFromPayload(payload map[string]any) (mobileWebDAVConfig, error) {
	raw := mapValue(firstNonNil(payload["webdav"], payload["webDAV"]))
	if len(raw) == 0 {
		snapshot := mapValue(firstNonNil(payload["config_snapshot"], payload["configSnapshot"]))
		if len(snapshot) == 0 {
			var err error
			snapshot, err = s.loadConfigSnapshotFromDisk()
			if err != nil {
				return mobileWebDAVConfig{}, err
			}
		} else {
			snapshot = sanitizeMobileConfigSnapshot(snapshot)
		}
		raw = mapValue(mapValue(snapshot["backup"])["webdav"])
	}
	return archivecore.ParseWebDAVConfig(raw)
}

func mobileWebDAVTargetURL(cfg mobileWebDAVConfig) (string, error) {
	return archivecore.WebDAVTargetURL(cfg)
}

func mobileWebDAVRequest(cfg mobileWebDAVConfig, method, targetURL string, body []byte) (int, []byte, error) {
	return archivecore.WebDAVRequest(nil, cfg, method, targetURL, body, "CFST-GUI/mobile")
}

func webDAVHTTPErrorMessage(prefix string, status int, body []byte) string {
	return archivecore.WebDAVHTTPErrorMessage(prefix, status, body)
}

func setMobileWebDAVTimestamp(snapshot map[string]any, key string, value string) map[string]any {
	return archivecore.SetWebDAVTimestamp(snapshot, key, value)
}

func stringSliceValue(value any) []string {
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		text := strings.TrimSpace(stringValue(item, ""))
		if text != "" {
			result = append(result, text)
		}
	}
	return result
}

func sensitiveMobileArchiveWarnings() []string {
	return archivecore.SensitiveArchiveWarnings()
}
