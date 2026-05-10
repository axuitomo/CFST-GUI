package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

const (
	configArchiveEntryName      = "cfst-gui-config.json"
	defaultConfigArchiveName    = "cfst-gui-config.zip"
	defaultWebDAVTimeoutSeconds = 30
)

type desktopWebDAVConfig struct {
	Enabled        bool
	Password       string
	RemotePath     string
	ServerURL      string
	TimeoutSeconds int
	Username       string
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
	profiles, profilesPresent, err := desktopProfilesForImport(body)
	if err != nil {
		return desktopCommandResult("CONFIG_ARCHIVE_IMPORT_PROFILE_FAILED", nil, err.Error(), false, nil, nil)
	}
	sourceProfiles, err := desktopSourceProfilesForImport(body, snapshot)
	if err != nil {
		return desktopCommandResult("CONFIG_ARCHIVE_IMPORT_SOURCE_PROFILE_FAILED", nil, err.Error(), false, nil, nil)
	}
	if restoredAt := strings.TrimSpace(stringValue(payload["restored_at"], "")); restoredAt != "" {
		snapshot = setDesktopWebDAVTimestamp(snapshot, "last_restore_at", restoredAt)
	}
	if err := writeDesktopConfigSnapshot(desktopConfigFilePath(), snapshot); err != nil {
		return desktopCommandResult("CONFIG_ARCHIVE_IMPORT_WRITE_FAILED", nil, err.Error(), false, nil, nil)
	}
	if profilesPresent {
		if err := saveProfileStore(profiles); err != nil {
			return desktopCommandResult("CONFIG_ARCHIVE_IMPORT_PROFILE_SAVE_FAILED", nil, err.Error(), false, nil, nil)
		}
	}
	if err := saveSourceProfileStore(sourceProfiles); err != nil {
		return desktopCommandResult("CONFIG_ARCHIVE_IMPORT_SOURCE_PROFILE_SAVE_FAILED", nil, err.Error(), false, nil, nil)
	}
	if !profilesPresent {
		loadedProfiles, err := loadProfileStore()
		if err == nil {
			profiles = loadedProfiles
		}
	}
	return desktopCommandResult("CONFIG_ARCHIVE_IMPORT_OK", map[string]any{
		"backup_path":     backupPath,
		"configPath":      desktopConfigFilePath(),
		"config_snapshot": snapshot,
		"file_name":       sourceName,
		"profiles":        profiles,
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
	profiles, err := loadProfileStore()
	if err != nil {
		return nil, nil, err
	}
	sourceProfiles, err := loadSourceProfileStoreForSnapshot(snapshot)
	if err != nil {
		return nil, nil, err
	}
	body := map[string]any{
		"app_version":     version,
		"config_snapshot": snapshot,
		"exported_at":     time.Now().Format(time.RFC3339),
		"profiles":        profiles,
		"schema_version":  guiSchemaVersion,
		"source_profiles": sourceProfiles,
		"storage":         resolveStorageState(),
	}
	raw, err := json.MarshalIndent(body, "", "  ")
	if err != nil {
		return nil, nil, err
	}
	archive, err := zipSingleFile(configArchiveEntryName, raw)
	if err != nil {
		return nil, nil, err
	}
	return archive, body, nil
}

func zipSingleFile(name string, raw []byte) ([]byte, error) {
	buffer := bytes.NewBuffer(nil)
	writer := zip.NewWriter(buffer)
	header := &zip.FileHeader{
		Name:   name,
		Method: zip.Deflate,
	}
	header.SetModTime(time.Now())
	entry, err := writer.CreateHeader(header)
	if err != nil {
		_ = writer.Close()
		return nil, err
	}
	if _, err := entry.Write(raw); err != nil {
		_ = writer.Close()
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func parseDesktopConfigArchive(raw []byte) (map[string]any, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("配置文件内容为空")
	}
	if bytes.HasPrefix(trimmed, []byte("{")) {
		return parseConfigArchiveJSON(trimmed)
	}
	reader, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return nil, err
	}
	var fallback *zip.File
	for _, file := range reader.File {
		if file.Name == configArchiveEntryName {
			return readDesktopArchiveJSONFile(file)
		}
		if fallback == nil && strings.HasSuffix(strings.ToLower(file.Name), ".json") {
			fallback = file
		}
	}
	if fallback != nil {
		return readDesktopArchiveJSONFile(fallback)
	}
	return nil, fmt.Errorf("配置压缩包缺少 %s", configArchiveEntryName)
}

func readDesktopArchiveJSONFile(file *zip.File) (map[string]any, error) {
	reader, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	raw, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	return parseConfigArchiveJSON(raw)
}

func parseConfigArchiveJSON(raw []byte) (map[string]any, error) {
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		return nil, err
	}
	return body, nil
}

func desktopArchivePayloadBytes(payload map[string]any) ([]byte, string, error) {
	if encoded := strings.TrimSpace(stringValue(firstNonNil(payload["content_base64"], payload["contentBase64"]), "")); encoded != "" {
		raw, err := base64.StdEncoding.DecodeString(encoded)
		return raw, defaultConfigArchiveName, err
	}
	if content := stringValue(payload["content"], ""); strings.TrimSpace(content) != "" {
		return []byte(content), "cfst-gui-config.json", nil
	}
	if targetPath := strings.TrimSpace(stringValue(firstNonNil(payload["path"], payload["target_path"], payload["targetPath"], payload["source_path"], payload["sourcePath"]), "")); targetPath != "" {
		raw, err := os.ReadFile(targetPath)
		return raw, filepath.Base(targetPath), err
	}
	return nil, "", fmt.Errorf("缺少配置压缩包内容或路径")
}

func writeDesktopLocalArchiveBackup(snapshot map[string]any, reason string) (string, error) {
	raw, _, err := buildDesktopConfigArchive(snapshot)
	if err != nil {
		return "", err
	}
	name := fmt.Sprintf("cfst-gui-%s-%s.zip", sanitizeTemplateFileName(reason), time.Now().Format("20060102-150405"))
	targetPath := filepath.Join(storageRoot(), "backups", name)
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(targetPath, raw, 0o600); err != nil {
		return "", err
	}
	return targetPath, nil
}

func desktopProfilesForImport(body map[string]any) (profileStore, bool, error) {
	raw, ok := firstPresent(body, "profiles", "Profiles")
	if !ok {
		return profileStore{}, false, nil
	}
	store := profileStoreFromArchive(raw)
	store = normalizeProfileStoreForArchive(store)
	return store, true, nil
}

func desktopSourceProfilesForImport(body map[string]any, snapshot map[string]any) (sourceProfileStore, error) {
	raw, ok := firstPresent(body, "source_profiles", "sourceProfiles")
	if !ok {
		return normalizeSourceProfileStoreForSave(defaultSourceProfileStoreFromSnapshot(snapshot)), nil
	}
	store := sourceProfileStoreFromAny(raw)
	if len(store.Items) == 0 {
		store = defaultSourceProfileStoreFromSnapshot(snapshot)
	}
	return normalizeSourceProfileStoreForSave(store), nil
}

func profileStoreFromArchive(value any) profileStore {
	raw, err := json.Marshal(value)
	if err != nil {
		return profileStore{}
	}
	var store profileStore
	if err := json.Unmarshal(raw, &store); err != nil {
		return profileStore{}
	}
	return store
}

func normalizeProfileStoreForArchive(store profileStore) profileStore {
	if store.SchemaVersion == "" {
		store.SchemaVersion = profilesSchemaVersion
	}
	now := time.Now().Format(time.RFC3339)
	if store.UpdatedAt == "" {
		store.UpdatedAt = now
	}
	if store.Items == nil {
		store.Items = []profileItem{}
	}
	for index := range store.Items {
		if strings.TrimSpace(store.Items[index].ID) == "" {
			store.Items[index].ID = fmt.Sprintf("profile-%d", time.Now().UnixNano()+int64(index))
		}
		if strings.TrimSpace(store.Items[index].Name) == "" {
			store.Items[index].Name = fmt.Sprintf("配置档案 %d", index+1)
		}
		if store.Items[index].ConfigSnapshot == nil {
			store.Items[index].ConfigSnapshot = map[string]any{}
		}
		store.Items[index].ConfigSnapshot = sanitizeDesktopConfigSnapshot(store.Items[index].ConfigSnapshot)
		if store.Items[index].CreatedAt == "" {
			store.Items[index].CreatedAt = now
		}
		if store.Items[index].UpdatedAt == "" {
			store.Items[index].UpdatedAt = now
		}
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

func firstPresent(source map[string]any, keys ...string) (any, bool) {
	for _, key := range keys {
		if value, ok := source[key]; ok && value != nil {
			return value, true
		}
	}
	return nil, false
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
	cfg := desktopWebDAVConfig{
		Enabled:        boolValue(raw["enabled"], false),
		Password:       stringValue(raw["password"], ""),
		RemotePath:     strings.TrimSpace(stringValue(firstNonNil(raw["remote_path"], raw["remotePath"]), defaultConfigArchiveName)),
		ServerURL:      strings.TrimSpace(stringValue(firstNonNil(raw["server_url"], raw["serverUrl"], raw["url"]), "")),
		TimeoutSeconds: intValue(firstNonNil(raw["timeout_seconds"], raw["timeoutSeconds"]), defaultWebDAVTimeoutSeconds),
		Username:       stringValue(raw["username"], ""),
	}
	if cfg.RemotePath == "" {
		cfg.RemotePath = defaultConfigArchiveName
	}
	if cfg.TimeoutSeconds <= 0 {
		cfg.TimeoutSeconds = defaultWebDAVTimeoutSeconds
	}
	if cfg.ServerURL == "" {
		return desktopWebDAVConfig{}, fmt.Errorf("缺少 WebDAV 地址")
	}
	return cfg, nil
}

func desktopWebDAVTargetURL(cfg desktopWebDAVConfig) (string, error) {
	if parsed, err := url.Parse(cfg.RemotePath); err == nil && parsed.IsAbs() {
		return parsed.String(), nil
	}
	base, err := url.Parse(cfg.ServerURL)
	if err != nil {
		return "", err
	}
	if base.Scheme != "http" && base.Scheme != "https" {
		return "", fmt.Errorf("WebDAV 地址必须以 http:// 或 https:// 开头")
	}
	if !strings.HasSuffix(base.Path, "/") {
		base.Path += "/"
	}
	remotePath := strings.TrimLeft(cfg.RemotePath, "/")
	base.Path = path.Join(base.Path, remotePath)
	if strings.HasSuffix(remotePath, "/") {
		base.Path += "/"
	}
	return base.String(), nil
}

func desktopWebDAVRequest(ctx context.Context, cfg desktopWebDAVConfig, method, targetURL string, body []byte) (int, []byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	timeout := time.Duration(cfg.TimeoutSeconds) * time.Second
	client := &http.Client{Timeout: timeout}
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, targetURL, reader)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("User-Agent", "CFST-GUI/"+version)
	if body != nil {
		req.Header.Set("Content-Type", "application/zip")
	}
	if cfg.Username != "" || cfg.Password != "" {
		req.SetBasicAuth(cfg.Username, cfg.Password)
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 32*1024*1024))
	return resp.StatusCode, raw, nil
}

func webDAVHTTPErrorMessage(prefix string, status int, body []byte) string {
	detail := strings.TrimSpace(string(body))
	if detail == "" {
		return fmt.Sprintf("%s：HTTP %d", prefix, status)
	}
	if len(detail) > 240 {
		detail = detail[:240] + "..."
	}
	return fmt.Sprintf("%s：HTTP %d，%s", prefix, status, detail)
}

func setDesktopWebDAVTimestamp(snapshot map[string]any, key string, value string) map[string]any {
	backup := mapValue(snapshot["backup"])
	webdav := mapValue(backup["webdav"])
	webdav[key] = value
	backup["webdav"] = webdav
	snapshot["backup"] = backup
	return snapshot
}

func sensitiveArchiveWarnings() []string {
	return []string{"配置压缩包包含完整 Cloudflare Token 和 WebDAV 凭据，请只保存到可信位置。"}
}
