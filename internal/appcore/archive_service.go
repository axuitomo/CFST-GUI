package appcore

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/axuitomo/CFST-GUI/internal/archivecore"
	"github.com/axuitomo/CFST-GUI/internal/probecore"
)

func (s *Service) invokeArchiveExport(payloadJSON string) CommandResult {
	payload, err := decodeCommandObject(payloadJSON)
	if err != nil {
		return NewCommandResult("CONFIG_ARCHIVE_EXPORT_INVALID", nil, err.Error(), false, nil, nil)
	}
	snapshot, err := s.archiveSnapshot(payload)
	if err != nil {
		return NewCommandResult("CONFIG_ARCHIVE_READ_FAILED", nil, err.Error(), false, nil, nil)
	}
	raw, _, err := s.buildConfigArchive(snapshot)
	if err != nil {
		return NewCommandResult("CONFIG_ARCHIVE_BUILD_FAILED", nil, err.Error(), false, nil, nil)
	}
	targetURI := strings.TrimSpace(stringValue(firstNonNil(payload["target_uri"], payload["targetUri"], payload["uri"]), ""))
	targetPath := strings.TrimSpace(stringValue(firstNonNil(payload["target_path"], payload["targetPath"], payload["path"]), ""))
	fileName := archiveExportFileName(payload, targetURI)
	data := map[string]any{"file_name": fileName}
	if targetPath != "" {
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return NewCommandResult("CONFIG_ARCHIVE_WRITE_FAILED", nil, err.Error(), false, nil, nil)
		}
		if err := os.WriteFile(targetPath, raw, 0o600); err != nil {
			return NewCommandResult("CONFIG_ARCHIVE_WRITE_FAILED", nil, err.Error(), false, nil, nil)
		}
		data["file_name"] = filepath.Base(targetPath)
		data["path"] = targetPath
		return NewCommandResult("CONFIG_ARCHIVE_EXPORT_OK", data, "配置压缩包已导出。", true, nil, archivecore.SensitiveArchiveWarnings())
	}
	data["content_base64"] = base64.StdEncoding.EncodeToString(raw)
	if targetURI != "" {
		data["target_uri"] = targetURI
	}
	return NewCommandResult("CONFIG_ARCHIVE_EXPORT_OK", data, "配置压缩包已准备发布。", true, nil, archivecore.SensitiveArchiveWarnings())
}

func (s *Service) invokeArchiveImport(payloadJSON, successMessage string) CommandResult {
	payload, err := decodeCommandObject(payloadJSON)
	if err != nil {
		return NewCommandResult("CONFIG_ARCHIVE_IMPORT_INVALID", nil, err.Error(), false, nil, nil)
	}
	return s.importConfigArchive(payload, successMessage)
}

func (s *Service) importConfigArchive(payload map[string]any, successMessage string) CommandResult {
	raw, sourceName, err := archivecore.ArchivePayloadBytes(payload)
	if err != nil {
		return NewCommandResult("CONFIG_ARCHIVE_IMPORT_READ_FAILED", nil, err.Error(), false, nil, nil)
	}
	body, err := archivecore.ParseConfigArchive(raw)
	if err != nil {
		return NewCommandResult("CONFIG_ARCHIVE_IMPORT_PARSE_FAILED", nil, err.Error(), false, nil, nil)
	}
	current := mapValue(firstNonNil(payload["current_config_snapshot"], payload["currentConfigSnapshot"], payload["backup_config_snapshot"], payload["backupConfigSnapshot"]))
	if len(current) == 0 {
		loaded, loadErr := s.LoadConfig()
		if loadErr != nil {
			return NewCommandResult("CONFIG_ARCHIVE_IMPORT_READ_FAILED", nil, loadErr.Error(), false, nil, nil)
		}
		current = loaded.Snapshot
	} else {
		current = s.sanitizeConfig(current)
	}
	backupPath := ""
	if len(current) > 0 {
		backupPath, err = s.writeLocalArchiveBackup(current, "pre-import")
		if err != nil {
			return NewCommandResult("CONFIG_ARCHIVE_IMPORT_BACKUP_FAILED", nil, err.Error(), false, nil, nil)
		}
	}
	snapshot := mapValue(firstNonNil(body["config_snapshot"], body["configSnapshot"]))
	if len(snapshot) == 0 {
		snapshot = body
	}
	snapshot = PreserveLocalExportTarget(s.sanitizeConfig(snapshot), current)
	profiles := SourceProfilesForArchiveImport(
		body,
		snapshot,
		DefaultSourceProfilesSchemaVersion,
		func() map[string]any { return s.sanitizeConfig(nil) },
		s.now().Format(timeRFC3339),
	)
	if restoredAt := strings.TrimSpace(stringValue(payload["restored_at"], "")); restoredAt != "" {
		snapshot = archivecore.SetWebDAVTimestamp(snapshot, "last_restore_at", restoredAt)
	}
	layout := s.StorageLayout()
	rollbackStates, err := CaptureFileStates(layout.ConfigPath(), layout.SourceProfilesPath())
	if err != nil {
		return NewCommandResult("CONFIG_ARCHIVE_IMPORT_WRITE_FAILED", nil, "准备导入回滚状态失败："+err.Error(), false, nil, nil)
	}
	if err := s.writeArchiveConfig(snapshot); err != nil {
		return archiveImportRollbackFailure("CONFIG_ARCHIVE_IMPORT_WRITE_FAILED", err, rollbackStates)
	}
	if err := s.writeArchiveSourceProfiles(profiles); err != nil {
		return archiveImportRollbackFailure("CONFIG_ARCHIVE_IMPORT_SOURCE_PROFILE_SAVE_FAILED", err, rollbackStates)
	}
	return NewCommandResult("CONFIG_ARCHIVE_IMPORT_OK", map[string]any{
		"backup_path":     backupPath,
		"configPath":      layout.ConfigPath(),
		"config_snapshot": snapshot,
		"file_name":       sourceName,
		"source_profiles": profiles,
		"storage":         s.archiveStorageState(),
	}, successMessage, true, nil, archivecore.SensitiveArchiveWarnings())
}

func (s *Service) invokeWebDAVTest(payloadJSON string) CommandResult {
	payload, err := decodeCommandObject(payloadJSON)
	if err != nil {
		return NewCommandResult("WEBDAV_INVALID", nil, err.Error(), false, nil, nil)
	}
	cfg, targetURL, result := s.webDAVRequestConfig(payload)
	if !result.OK {
		return result
	}
	status, body, err := archivecore.WebDAVRequest(context.Background(), cfg, http.MethodHead, targetURL, nil, s.archiveUserAgent())
	if err != nil {
		return NewCommandResult("WEBDAV_TEST_FAILED", nil, err.Error(), false, nil, nil)
	}
	if !((status >= 200 && status < 400) || status == http.StatusNotFound) {
		return NewCommandResult("WEBDAV_TEST_FAILED", map[string]any{"status": status, "target_url": targetURL}, archivecore.WebDAVHTTPErrorMessage("WebDAV 测试失败", status, body), false, nil, nil)
	}
	message := "WebDAV 连接可用。"
	if status == http.StatusNotFound {
		message = "WebDAV 连接可用，远端配置包尚不存在。"
	}
	return NewCommandResult("WEBDAV_TEST_OK", map[string]any{"remote_path": cfg.RemotePath, "status": status, "target_url": targetURL}, message, true, nil, nil)
}

func (s *Service) invokeWebDAVBackup(payloadJSON string) CommandResult {
	payload, err := decodeCommandObject(payloadJSON)
	if err != nil {
		return NewCommandResult("WEBDAV_INVALID", nil, err.Error(), false, nil, nil)
	}
	cfg, targetURL, result := s.webDAVRequestConfig(payload)
	if !result.OK {
		return result
	}
	snapshot, err := s.archiveSnapshot(payload)
	if err != nil {
		return NewCommandResult("WEBDAV_BACKUP_READ_FAILED", nil, err.Error(), false, nil, nil)
	}
	snapshot = archivecore.SetWebDAVTimestamp(snapshot, "last_backup_at", s.now().Format(timeRFC3339))
	raw, _, err := s.buildConfigArchive(snapshot)
	if err != nil {
		return NewCommandResult("WEBDAV_BACKUP_BUILD_FAILED", nil, err.Error(), false, nil, nil)
	}
	status, body, err := archivecore.WebDAVRequest(context.Background(), cfg, http.MethodPut, targetURL, raw, s.archiveUserAgent())
	if err != nil {
		return NewCommandResult("WEBDAV_BACKUP_FAILED", nil, err.Error(), false, nil, nil)
	}
	if status < 200 || status >= 300 {
		return NewCommandResult("WEBDAV_BACKUP_FAILED", map[string]any{"status": status, "target_url": targetURL}, archivecore.WebDAVHTTPErrorMessage("WebDAV 备份失败", status, body), false, nil, nil)
	}
	if _, err := s.SaveConfig(snapshot); err != nil {
		return NewCommandResult("WEBDAV_BACKUP_STATE_SAVE_FAILED", nil, err.Error(), false, nil, archivecore.SensitiveArchiveWarnings())
	}
	return NewCommandResult("WEBDAV_BACKUP_OK", map[string]any{"remote_path": cfg.RemotePath, "status": status, "target_url": targetURL}, "配置压缩包已备份到 WebDAV。", true, nil, archivecore.SensitiveArchiveWarnings())
}

func (s *Service) invokeWebDAVRestore(payloadJSON string) CommandResult {
	payload, err := decodeCommandObject(payloadJSON)
	if err != nil {
		return NewCommandResult("WEBDAV_INVALID", nil, err.Error(), false, nil, nil)
	}
	cfg, targetURL, result := s.webDAVRequestConfig(payload)
	if !result.OK {
		return result
	}
	status, body, err := archivecore.WebDAVRequest(context.Background(), cfg, http.MethodGet, targetURL, nil, s.archiveUserAgent())
	if err != nil {
		return NewCommandResult("WEBDAV_RESTORE_FAILED", nil, err.Error(), false, nil, nil)
	}
	if status < 200 || status >= 300 {
		return NewCommandResult("WEBDAV_RESTORE_FAILED", map[string]any{"status": status, "target_url": targetURL}, archivecore.WebDAVHTTPErrorMessage("WebDAV 还原失败", status, body), false, nil, nil)
	}
	payload["content_base64"] = base64.StdEncoding.EncodeToString(body)
	payload["restored_at"] = s.now().Format(timeRFC3339)
	result = s.importConfigArchive(payload, "已从 WebDAV 还原配置压缩包。")
	if result.OK {
		data := mapValue(result.Data)
		data["remote_path"] = cfg.RemotePath
		data["target_url"] = targetURL
		result.Data = data
	}
	return result
}

func (s *Service) archiveSnapshot(payload map[string]any) (map[string]any, error) {
	snapshot := mapValue(firstNonNil(payload["config_snapshot"], payload["configSnapshot"]))
	if len(snapshot) > 0 {
		return s.sanitizeConfig(snapshot), nil
	}
	loaded, err := s.LoadConfig()
	return loaded.Snapshot, err
}

func (s *Service) buildConfigArchive(snapshot map[string]any) ([]byte, map[string]any, error) {
	snapshot = s.sanitizeConfig(snapshot)
	store, err := LoadSourceProfileStore(s.StorageLayout().SourceProfilesPath(), DefaultSourceProfilesSchemaVersion)
	if err != nil {
		return nil, nil, err
	}
	if len(store.Items) == 0 {
		store = BlankSourceProfileStore(s.now().Format(timeRFC3339), DefaultSourceProfilesSchemaVersion)
	} else if strings.TrimSpace(store.ActiveProfileID) == "" {
		store.ActiveProfileID = store.Items[0].ID
	}
	return BuildConfigArchive(snapshot, store, s.archiveStorageState(), s.archiveAppVersion(), ConfigSchemaVersion, s.now().Format(timeRFC3339))
}

func (s *Service) writeLocalArchiveBackup(snapshot map[string]any, reason string) (string, error) {
	raw, _, err := s.buildConfigArchive(snapshot)
	if err != nil {
		return "", err
	}
	name := fmt.Sprintf("cfst-gui-%s-%s.zip", probecore.SanitizeTemplateFileName(reason), s.now().Format("20060102-150405"))
	targetPath := filepath.Join(s.StorageLayout().BackupsRoot(), name)
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(targetPath, raw, 0o600); err != nil {
		return "", err
	}
	return targetPath, nil
}

func (s *Service) webDAVRequestConfig(payload map[string]any) (archivecore.WebDAVConfig, string, CommandResult) {
	raw := mapValue(firstNonNil(payload["webdav"], payload["webDAV"]))
	if len(raw) == 0 {
		snapshot, err := s.archiveSnapshot(payload)
		if err != nil {
			return archivecore.WebDAVConfig{}, "", NewCommandResult("WEBDAV_INVALID", nil, err.Error(), false, nil, nil)
		}
		raw = mapValue(mapValue(snapshot["backup"])["webdav"])
	}
	cfg, err := archivecore.ParseWebDAVConfig(raw)
	if err != nil {
		return cfg, "", NewCommandResult("WEBDAV_INVALID", nil, err.Error(), false, nil, nil)
	}
	targetURL, err := archivecore.WebDAVTargetURL(cfg)
	if err != nil {
		return cfg, "", NewCommandResult("WEBDAV_INVALID", nil, err.Error(), false, nil, nil)
	}
	return cfg, targetURL, CommandResult{OK: true}
}

func (s *Service) sanitizeConfig(snapshot map[string]any) map[string]any {
	s.mu.RLock()
	policy := s.options.ConfigPolicy
	s.mu.RUnlock()
	return sanitizeServiceConfig(snapshot, policy)
}

func (s *Service) writeArchiveConfig(snapshot map[string]any) error {
	s.mu.RLock()
	hook := s.options.ArchiveWriteConfig
	s.mu.RUnlock()
	if hook != nil {
		return hook(snapshot)
	}
	_, err := s.SaveConfig(snapshot)
	return err
}

func (s *Service) writeArchiveSourceProfiles(store SourceProfileStore) error {
	s.mu.RLock()
	hook := s.options.ArchiveSaveSourceProfiles
	s.mu.RUnlock()
	if hook != nil {
		return hook(store)
	}
	return s.saveSourceProfiles(store)
}

func (s *Service) archiveStorageState() any {
	s.mu.RLock()
	provider := s.options.ArchiveStorageState
	s.mu.RUnlock()
	if provider != nil {
		return provider()
	}
	layout := s.StorageLayout()
	return map[string]any{"current_dir": layout.Root, "runtime_dir": layout.Root}
}

func (s *Service) archiveAppVersion() string {
	s.mu.RLock()
	version := strings.TrimSpace(s.options.AppVersion)
	s.mu.RUnlock()
	if version == "" {
		return "unknown"
	}
	return version
}

func (s *Service) archiveUserAgent() string {
	return "CFST-GUI/" + s.archiveAppVersion()
}

func archiveExportFileName(payload map[string]any, targetURI string) string {
	if strings.HasPrefix(targetURI, "browser-download:") {
		if name := strings.TrimSpace(strings.TrimPrefix(targetURI, "browser-download:")); name != "" {
			return filepath.Base(name)
		}
	}
	name := strings.TrimSpace(stringValue(firstNonNil(payload["file_name"], payload["fileName"], payload["default_file_name"], payload["defaultFileName"]), DefaultConfigArchiveName))
	if name == "" {
		name = DefaultConfigArchiveName
	}
	return filepath.Base(name)
}

func archiveImportRollbackFailure(code string, err error, states []FileState) CommandResult {
	if rollbackErr := RestoreFileStates(states); rollbackErr != nil {
		return NewCommandResult(code, nil, err.Error()+"；回滚失败："+rollbackErr.Error(), false, nil, nil)
	}
	return NewCommandResult(code, nil, err.Error(), false, nil, nil)
}

const timeRFC3339 = "2006-01-02T15:04:05Z07:00"
