package mobileapi

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

type mobileWebDAVConfig struct {
	Enabled        bool
	Password       string
	RemotePath     string
	ServerURL      string
	TimeoutSeconds int
	Username       string
}

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
	buffer := bytes.NewBuffer(nil)
	writer := zip.NewWriter(buffer)
	header := &zip.FileHeader{Name: name, Method: zip.Deflate}
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

func parseMobileConfigArchive(raw []byte) (map[string]any, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("配置文件内容为空")
	}
	if bytes.HasPrefix(trimmed, []byte("{")) {
		return parseMobileConfigArchiveJSON(trimmed)
	}
	reader, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return nil, err
	}
	var fallback *zip.File
	for _, file := range reader.File {
		if file.Name == configArchiveEntryName {
			return readMobileArchiveJSONFile(file)
		}
		if fallback == nil && strings.HasSuffix(strings.ToLower(file.Name), ".json") {
			fallback = file
		}
	}
	if fallback != nil {
		return readMobileArchiveJSONFile(fallback)
	}
	return nil, fmt.Errorf("配置压缩包缺少 %s", configArchiveEntryName)
}

func readMobileArchiveJSONFile(file *zip.File) (map[string]any, error) {
	reader, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	raw, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	return parseMobileConfigArchiveJSON(raw)
}

func parseMobileConfigArchiveJSON(raw []byte) (map[string]any, error) {
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		return nil, err
	}
	return body, nil
}

func mobileArchivePayloadBytes(payload map[string]any) ([]byte, string, error) {
	if encoded := strings.TrimSpace(stringValue(firstNonNil(payload["content_base64"], payload["contentBase64"]), "")); encoded != "" {
		raw, err := base64.StdEncoding.DecodeString(encoded)
		return raw, defaultConfigArchiveName, err
	}
	if content := stringValue(payload["content"], ""); strings.TrimSpace(content) != "" {
		return []byte(content), "cfst-gui-config.json", nil
	}
	if targetPath := strings.TrimSpace(stringValue(firstNonNil(payload["path"], payload["target_path"], payload["targetPath"], payload["source_path"], payload["sourcePath"]), "")); targetPath != "" && !strings.HasPrefix(targetPath, "content://") {
		raw, err := os.ReadFile(targetPath)
		return raw, filepath.Base(targetPath), err
	}
	return nil, "", fmt.Errorf("缺少配置压缩包内容或路径")
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
	if store.SchemaVersion == "" {
		store.SchemaVersion = profilesSchemaVersion
	}
	now := nowRFC3339()
	if store.UpdatedAt == "" {
		store.UpdatedAt = now
	}
	if store.Items == nil {
		store.Items = []mobileProfileItem{}
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
		store.Items[index].ConfigSnapshot = sanitizeMobileConfigSnapshot(store.Items[index].ConfigSnapshot)
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
	cfg := mobileWebDAVConfig{
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
		return mobileWebDAVConfig{}, fmt.Errorf("缺少 WebDAV 地址")
	}
	return cfg, nil
}

func mobileWebDAVTargetURL(cfg mobileWebDAVConfig) (string, error) {
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

func mobileWebDAVRequest(cfg mobileWebDAVConfig, method, targetURL string, body []byte) (int, []byte, error) {
	timeout := time.Duration(cfg.TimeoutSeconds) * time.Second
	client := &http.Client{Timeout: timeout}
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(context.Background(), method, targetURL, reader)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("User-Agent", "CFST-GUI/mobile")
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

func setMobileWebDAVTimestamp(snapshot map[string]any, key string, value string) map[string]any {
	backup := mapValue(snapshot["backup"])
	webdav := mapValue(backup["webdav"])
	webdav[key] = value
	backup["webdav"] = webdav
	snapshot["backup"] = backup
	return snapshot
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
	return []string{"配置压缩包包含完整 Cloudflare Token 和 WebDAV 凭据，请只保存到可信位置。"}
}
