package app

import (
	"context"
	"encoding/base64"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/appcore"
	"github.com/axuitomo/CFST-GUI/internal/archivecore"
)

const (
	configArchiveEntryName      = archivecore.ConfigArchiveEntryName
	defaultConfigArchiveName    = archivecore.DefaultConfigArchiveName
	defaultWebDAVTimeoutSeconds = archivecore.DefaultWebDAVTimeoutSeconds
)

type desktopWebDAVConfig = archivecore.WebDAVConfig

var (
	writeDesktopConfigSnapshotForImport    = writeDesktopConfigSnapshot
	saveDesktopSourceProfileStoreForImport = saveSourceProfileStore
)

func zipSingleFile(name string, raw []byte) ([]byte, error) {
	return archivecore.ZipSingleFile(name, raw)
}

func (a *App) ExportConfigArchive(payload map[string]any) DesktopCommandResult {
	snapshot, err := desktopSnapshotForArchive(payload)
	if err != nil {
		return desktopCommandResult("CONFIG_ARCHIVE_READ_FAILED", nil, err.Error(), false, nil, nil)
	}
	raw, _, err := buildDesktopConfigArchive(snapshot)
	if err != nil {
		return desktopCommandResult("CONFIG_ARCHIVE_BUILD_FAILED", nil, err.Error(), false, nil, nil)
	}
	targetURI := strings.TrimSpace(stringValue(firstNonNil(payload["target_uri"], payload["targetUri"], payload["uri"]), ""))
	if strings.HasPrefix(targetURI, "browser-download:") {
		fileName := strings.TrimSpace(strings.TrimPrefix(targetURI, "browser-download:"))
		if fileName == "" {
			fileName = defaultConfigArchiveName
		}
		return desktopCommandResult("CONFIG_ARCHIVE_EXPORT_OK", map[string]any{
			"content_base64": base64.StdEncoding.EncodeToString(raw),
			"file_name":      filepath.Base(fileName),
			"target_uri":     targetURI,
		}, "配置压缩包已准备下载。", true, nil, sensitiveArchiveWarnings())
	}
	targetPath := strings.TrimSpace(stringValue(firstNonNil(payload["target_path"], payload["targetPath"], payload["path"]), ""))
	if targetPath == "" {
		return desktopCommandResult("CONFIG_ARCHIVE_EXPORT_INVALID", nil, "缺少导出目标路径。", false, nil, nil)
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return desktopCommandResult("CONFIG_ARCHIVE_WRITE_FAILED", nil, err.Error(), false, nil, nil)
	}
	if err := os.WriteFile(targetPath, raw, 0o600); err != nil {
		return desktopCommandResult("CONFIG_ARCHIVE_WRITE_FAILED", nil, err.Error(), false, nil, nil)
	}
	return desktopCommandResult("CONFIG_ARCHIVE_EXPORT_OK", map[string]any{
		"file_name": filepath.Base(targetPath),
		"path":      targetPath,
	}, "配置压缩包已导出。", true, nil, sensitiveArchiveWarnings())
}

func (a *App) ImportConfigArchive(payload map[string]any) DesktopCommandResult {
	return a.importConfigArchivePayload(payload, "配置压缩包已导入。")
}

func (a *App) TestWebDAV(payload map[string]any) DesktopCommandResult {
	cfg, err := desktopWebDAVConfigFromPayload(payload)
	if err != nil {
		return desktopCommandResult("WEBDAV_INVALID", nil, err.Error(), false, nil, nil)
	}
	targetURL, err := desktopWebDAVTargetURL(cfg)
	if err != nil {
		return desktopCommandResult("WEBDAV_INVALID", nil, err.Error(), false, nil, nil)
	}
	status, body, err := desktopWebDAVRequest(a.ctx, cfg, http.MethodHead, targetURL, nil)
	if err != nil {
		return desktopCommandResult("WEBDAV_TEST_FAILED", nil, err.Error(), false, nil, nil)
	}
	ok := (status >= 200 && status < 400) || status == http.StatusNotFound
	if !ok {
		return desktopCommandResult("WEBDAV_TEST_FAILED", map[string]any{
			"status":     status,
			"target_url": targetURL,
		}, webDAVHTTPErrorMessage("WebDAV 测试失败", status, body), false, nil, nil)
	}
	message := "WebDAV 连接可用。"
	if status == http.StatusNotFound {
		message = "WebDAV 连接可用，远端配置包尚不存在。"
	}
	return desktopCommandResult("WEBDAV_TEST_OK", map[string]any{
		"status":      status,
		"remote_path": cfg.RemotePath,
		"target_url":  targetURL,
	}, message, true, nil, nil)
}

func (a *App) BackupConfigToWebDAV(payload map[string]any) DesktopCommandResult {
	cfg, err := desktopWebDAVConfigFromPayload(payload)
	if err != nil {
		return desktopCommandResult("WEBDAV_INVALID", nil, err.Error(), false, nil, nil)
	}
	snapshot, err := desktopSnapshotForArchive(payload)
	if err != nil {
		return desktopCommandResult("WEBDAV_BACKUP_READ_FAILED", nil, err.Error(), false, nil, nil)
	}
	now := time.Now().Format(time.RFC3339)
	snapshot = setDesktopWebDAVTimestamp(snapshot, "last_backup_at", now)
	raw, _, err := buildDesktopConfigArchive(snapshot)
	if err != nil {
		return desktopCommandResult("WEBDAV_BACKUP_BUILD_FAILED", nil, err.Error(), false, nil, nil)
	}
	targetURL, err := desktopWebDAVTargetURL(cfg)
	if err != nil {
		return desktopCommandResult("WEBDAV_INVALID", nil, err.Error(), false, nil, nil)
	}
	status, body, err := desktopWebDAVRequest(a.ctx, cfg, http.MethodPut, targetURL, raw)
	if err != nil {
		return desktopCommandResult("WEBDAV_BACKUP_FAILED", nil, err.Error(), false, nil, nil)
	}
	if status < 200 || status >= 300 {
		return desktopCommandResult("WEBDAV_BACKUP_FAILED", map[string]any{
			"status":     status,
			"target_url": targetURL,
		}, webDAVHTTPErrorMessage("WebDAV 备份失败", status, body), false, nil, nil)
	}
	_ = writeDesktopConfigSnapshot(desktopConfigFilePath(), snapshot)
	return desktopCommandResult("WEBDAV_BACKUP_OK", map[string]any{
		"remote_path": cfg.RemotePath,
		"status":      status,
		"target_url":  targetURL,
	}, "配置压缩包已备份到 WebDAV。", true, nil, sensitiveArchiveWarnings())
}

func (a *App) RestoreConfigFromWebDAV(payload map[string]any) DesktopCommandResult {
	cfg, err := desktopWebDAVConfigFromPayload(payload)
	if err != nil {
		return desktopCommandResult("WEBDAV_INVALID", nil, err.Error(), false, nil, nil)
	}
	targetURL, err := desktopWebDAVTargetURL(cfg)
	if err != nil {
		return desktopCommandResult("WEBDAV_INVALID", nil, err.Error(), false, nil, nil)
	}
	status, body, err := desktopWebDAVRequest(a.ctx, cfg, http.MethodGet, targetURL, nil)
	if err != nil {
		return desktopCommandResult("WEBDAV_RESTORE_FAILED", nil, err.Error(), false, nil, nil)
	}
	if status < 200 || status >= 300 {
		return desktopCommandResult("WEBDAV_RESTORE_FAILED", map[string]any{
			"status":     status,
			"target_url": targetURL,
		}, webDAVHTTPErrorMessage("WebDAV 还原失败", status, body), false, nil, nil)
	}
	payload["content_base64"] = base64.StdEncoding.EncodeToString(body)
	payload["restored_at"] = time.Now().Format(time.RFC3339)
	result := a.importConfigArchivePayload(payload, "已从 WebDAV 还原配置压缩包。")
	if result.OK {
		data := mapValue(result.Data)
		data["remote_path"] = cfg.RemotePath
		data["target_url"] = targetURL
		result.Data = data
	}
	return result
}

func (a *App) importConfigArchivePayload(payload map[string]any, successMessage string) DesktopCommandResult {
	raw, sourceName, err := desktopArchivePayloadBytes(payload)
	if err != nil {
		return desktopCommandResult("CONFIG_ARCHIVE_IMPORT_READ_FAILED", nil, err.Error(), false, nil, nil)
	}
	body, err := parseDesktopConfigArchive(raw)
	if err != nil {
		return desktopCommandResult("CONFIG_ARCHIVE_IMPORT_PARSE_FAILED", nil, err.Error(), false, nil, nil)
	}
	current := mapValue(firstNonNil(payload["current_config_snapshot"], payload["currentConfigSnapshot"], payload["backup_config_snapshot"], payload["backupConfigSnapshot"]))
	if len(current) == 0 {
		current, _ = loadDesktopConfigSnapshotFromDisk()
	} else {
		current = sanitizeDesktopConfigSnapshot(current)
	}
	backupPath := ""
	if len(current) > 0 {
		if path, err := writeDesktopLocalArchiveBackup(current, "pre-import"); err == nil {
			backupPath = path
		} else {
			return desktopCommandResult("CONFIG_ARCHIVE_IMPORT_BACKUP_FAILED", nil, err.Error(), false, nil, nil)
		}
	}
	snapshot := mapValue(firstNonNil(body["config_snapshot"], body["configSnapshot"]))
	if len(snapshot) == 0 {
		snapshot = body
	}
	snapshot = sanitizeDesktopConfigSnapshot(snapshot)
	snapshot = appcore.PreserveLocalExportTarget(snapshot, current)
	sourceProfiles, err := desktopSourceProfilesForImport(body, snapshot)
	if err != nil {
		return desktopCommandResult("CONFIG_ARCHIVE_IMPORT_SOURCE_PROFILE_FAILED", nil, err.Error(), false, nil, nil)
	}
	if restoredAt := strings.TrimSpace(stringValue(payload["restored_at"], "")); restoredAt != "" {
		snapshot = setDesktopWebDAVTimestamp(snapshot, "last_restore_at", restoredAt)
	}
	rollbackStates, err := appcore.CaptureFileStates(
		desktopConfigFilePath(),
		sourceProfilesPath(),
	)
	if err != nil {
		return desktopCommandResult("CONFIG_ARCHIVE_IMPORT_WRITE_FAILED", nil, "准备导入回滚状态失败："+err.Error(), false, nil, nil)
	}
	if err := writeDesktopConfigSnapshotForImport(desktopConfigFilePath(), snapshot); err != nil {
		return desktopImportRollbackFailure("CONFIG_ARCHIVE_IMPORT_WRITE_FAILED", err, rollbackStates)
	}
	if err := saveDesktopSourceProfileStoreForImport(sourceProfiles); err != nil {
		return desktopImportRollbackFailure("CONFIG_ARCHIVE_IMPORT_SOURCE_PROFILE_SAVE_FAILED", err, rollbackStates)
	}
	return desktopCommandResult("CONFIG_ARCHIVE_IMPORT_OK", map[string]any{
		"backup_path":     backupPath,
		"configPath":      desktopConfigFilePath(),
		"config_snapshot": snapshot,
		"file_name":       sourceName,
		"source_profiles": sourceProfiles,
		"storage":         resolveStorageState(),
	}, successMessage, true, nil, sensitiveArchiveWarnings())
}

func desktopSnapshotForArchive(payload map[string]any) (map[string]any, error) {
	snapshot := mapValue(firstNonNil(payload["config_snapshot"], payload["configSnapshot"]))
	if len(snapshot) > 0 {
		return sanitizeDesktopConfigSnapshot(snapshot), nil
	}
	return loadDesktopConfigSnapshotFromDisk()
}

func buildDesktopConfigArchive(snapshot map[string]any) ([]byte, map[string]any, error) {
	snapshot = sanitizeDesktopConfigSnapshot(snapshot)
	sourceProfiles, err := loadSourceProfileStoreForSnapshot(snapshot)
	if err != nil {
		return nil, nil, err
	}
	return appcore.BuildConfigArchive(snapshot, sourceProfiles, resolveStorageState(), version, guiSchemaVersion, time.Now().Format(time.RFC3339))
}

func parseDesktopConfigArchive(raw []byte) (map[string]any, error) {
	return appcore.ParseConfigArchive(raw)
}

func desktopArchivePayloadBytes(payload map[string]any) ([]byte, string, error) {
	return appcore.ArchivePayloadBytes(payload)
}

func writeDesktopLocalArchiveBackup(snapshot map[string]any, reason string) (string, error) {
	return appcore.WriteLocalArchiveBackup(storageRoot(), snapshot, reason, buildDesktopConfigArchive)
}

func desktopSourceProfilesForImport(body map[string]any, snapshot map[string]any) (sourceProfileStore, error) {
	return appcore.SourceProfilesForArchiveImport(body, snapshot, sourceProfilesSchemaVersion, defaultDesktopConfigSnapshot, time.Now().Format(time.RFC3339)), nil
}

func desktopWebDAVConfigFromPayload(payload map[string]any) (desktopWebDAVConfig, error) {
	raw := mapValue(firstNonNil(payload["webdav"], payload["webDAV"]))
	if len(raw) == 0 {
		snapshot := mapValue(firstNonNil(payload["config_snapshot"], payload["configSnapshot"]))
		if len(snapshot) == 0 {
			var err error
			snapshot, err = loadDesktopConfigSnapshotFromDisk()
			if err != nil {
				return desktopWebDAVConfig{}, err
			}
		} else {
			snapshot = sanitizeDesktopConfigSnapshot(snapshot)
		}
		raw = mapValue(mapValue(snapshot["backup"])["webdav"])
	}
	return archivecore.ParseWebDAVConfig(raw)
}

func desktopWebDAVTargetURL(cfg desktopWebDAVConfig) (string, error) {
	return archivecore.WebDAVTargetURL(cfg)
}

func desktopWebDAVRequest(ctx context.Context, cfg desktopWebDAVConfig, method, targetURL string, body []byte) (int, []byte, error) {
	return archivecore.WebDAVRequest(ctx, cfg, method, targetURL, body, "CFST-GUI/"+version)
}

func webDAVHTTPErrorMessage(prefix string, status int, body []byte) string {
	return archivecore.WebDAVHTTPErrorMessage(prefix, status, body)
}

func setDesktopWebDAVTimestamp(snapshot map[string]any, key string, value string) map[string]any {
	return archivecore.SetWebDAVTimestamp(snapshot, key, value)
}

func sensitiveArchiveWarnings() []string {
	return archivecore.SensitiveArchiveWarnings()
}

func desktopImportRollbackFailure(code string, err error, rollbackStates []appcore.FileState) DesktopCommandResult {
	if rollbackErr := appcore.RestoreFileStates(rollbackStates); rollbackErr != nil {
		return desktopCommandResult(code, nil, err.Error()+"；回滚失败："+rollbackErr.Error(), false, nil, nil)
	}
	return desktopCommandResult(code, nil, err.Error(), false, nil, nil)
}
