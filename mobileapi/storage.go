package mobileapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	storageSchemaVersion  = "cfst-gui-storage-v1"
	profilesSchemaVersion = "cfst-gui-profiles-v1"
)

type mobileStorageBootstrap struct {
	DisplayName    string `json:"display_name,omitempty"`
	PortableMode   bool   `json:"portable_mode"`
	SchemaVersion  string `json:"schema_version"`
	SetupCompleted bool   `json:"setup_completed"`
	StorageDir     string `json:"storage_dir,omitempty"`
	StorageURI     string `json:"storage_uri,omitempty"`
	UpdatedAt      string `json:"updated_at"`
}

type mobileStorageHealth struct {
	CheckedAt    string `json:"checked_at"`
	Exists       bool   `json:"exists"`
	FreeBytes    int64  `json:"free_bytes"`
	IsDir        bool   `json:"is_dir"`
	Message      string `json:"message"`
	Path         string `json:"path"`
	PortableMode bool   `json:"portable_mode"`
	Writable     bool   `json:"writable"`
}

type mobileProfileItem struct {
	ConfigSnapshot map[string]any `json:"config_snapshot"`
	CreatedAt      string         `json:"created_at"`
	ID             string         `json:"id"`
	Name           string         `json:"name"`
	UpdatedAt      string         `json:"updated_at"`
}

type mobileProfileStore struct {
	ActiveProfileID string              `json:"active_profile_id"`
	Items           []mobileProfileItem `json:"items"`
	SchemaVersion   string              `json:"schema_version"`
	UpdatedAt       string              `json:"updated_at"`
}

func (s *Service) storageBootstrapPath() string {
	return filepath.Join(s.basePath(), "storage.json")
}

func (s *Service) storageStatus() map[string]any {
	bootstrap, _ := s.readStorageBootstrap()
	health := checkMobileStorageHealth(s.basePath())
	setupCompleted := bootstrap.SetupCompleted
	return map[string]any{
		"bootstrap_path":  s.storageBootstrapPath(),
		"current_dir":     s.basePath(),
		"default_dir":     s.basePath(),
		"display_name":    bootstrap.DisplayName,
		"health":          health,
		"portable_mode":   false,
		"setup_completed": setupCompleted,
		"setup_required":  !setupCompleted,
		"storage_uri":     bootstrap.StorageURI,
		"writable":        health.Writable,
	}
}

func (s *Service) readStorageBootstrap() (mobileStorageBootstrap, error) {
	raw, err := os.ReadFile(s.storageBootstrapPath())
	if err != nil {
		return mobileStorageBootstrap{}, err
	}
	var bootstrap mobileStorageBootstrap
	if err := json.Unmarshal(raw, &bootstrap); err != nil {
		return mobileStorageBootstrap{}, err
	}
	return bootstrap, nil
}

func (s *Service) writeStorageBootstrap(bootstrap mobileStorageBootstrap) error {
	bootstrap.SchemaVersion = storageSchemaVersion
	bootstrap.UpdatedAt = nowRFC3339()
	if err := os.MkdirAll(filepath.Dir(s.storageBootstrapPath()), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(bootstrap, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.storageBootstrapPath(), raw, 0o600)
}

func checkMobileStorageHealth(path string) mobileStorageHealth {
	health := mobileStorageHealth{
		CheckedAt: time.Now().Format(time.RFC3339),
		FreeBytes: -1,
		Path:      path,
	}
	if strings.TrimSpace(path) == "" {
		health.Message = "储存目录为空。"
		return health
	}
	info, err := os.Stat(path)
	if err == nil {
		health.Exists = true
		health.IsDir = info.IsDir()
	} else if errors.Is(err, os.ErrNotExist) {
		health.IsDir = true
	} else {
		health.Message = err.Error()
		return health
	}
	if !health.IsDir {
		health.Message = "目标路径不是目录。"
		return health
	}
	if err := os.MkdirAll(path, 0o755); err != nil {
		health.Message = err.Error()
		return health
	}
	testPath := filepath.Join(path, ".cfst-gui-write-test")
	if err := os.WriteFile(testPath, []byte("ok"), 0o600); err != nil {
		health.Message = err.Error()
		return health
	}
	_ = os.Remove(testPath)
	health.Exists = true
	health.Writable = true
	health.Message = "储存目录可用。"
	return health
}

func (s *Service) SetStorageDirectory(payloadJSON string) string {
	payload, err := decodeObject(payloadJSON)
	if err != nil {
		return encodeCommand(commandResultFor("STORAGE_SET_FAILED", nil, err.Error(), false, nil, nil))
	}
	bootstrap := mobileStorageBootstrap{
		DisplayName:    strings.TrimSpace(stringValue(firstNonNil(payload["display_name"], payload["displayName"]), "")),
		PortableMode:   false,
		SchemaVersion:  storageSchemaVersion,
		SetupCompleted: true,
		StorageDir:     s.basePath(),
		StorageURI:     strings.TrimSpace(stringValue(firstNonNil(payload["storage_uri"], payload["storageUri"], payload["uri"], payload["target_uri"], payload["targetUri"]), "")),
	}
	if boolValue(firstNonNil(payload["use_default"], payload["useDefault"], payload["reset_default"], payload["resetDefault"]), false) {
		bootstrap.StorageURI = ""
		bootstrap.DisplayName = ""
	}
	if err := s.writeStorageBootstrap(bootstrap); err != nil {
		return encodeCommand(commandResultFor("STORAGE_SET_FAILED", nil, err.Error(), false, nil, nil))
	}
	return encodeCommand(commandResultFor("STORAGE_SET_OK", map[string]any{
		"migration": map[string]any{"copied": []string{}, "failed": []string{}, "skipped": []string{}},
		"storage":   s.storageStatus(),
	}, "移动端储存目录已更新。", true, nil, nil))
}

func (s *Service) CheckStorageHealth(payloadJSON string) string {
	_, _ = decodeObject(payloadJSON)
	return encodeCommand(commandResultFor("STORAGE_HEALTH_READY", map[string]any{
		"health":  checkMobileStorageHealth(s.basePath()),
		"storage": s.storageStatus(),
	}, "储存目录健康检查已完成。", true, nil, nil))
}

func (s *Service) profilesPath() string {
	return filepath.Join(s.basePath(), "profiles.json")
}

func (s *Service) loadProfileStore() (mobileProfileStore, error) {
	store := mobileProfileStore{
		Items:         []mobileProfileItem{},
		SchemaVersion: profilesSchemaVersion,
	}
	raw, err := os.ReadFile(s.profilesPath())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return store, nil
		}
		return store, err
	}
	if err := json.Unmarshal(raw, &store); err != nil {
		return store, err
	}
	if store.Items == nil {
		store.Items = []mobileProfileItem{}
	}
	return store, nil
}

func (s *Service) saveProfileStore(store mobileProfileStore) error {
	store.SchemaVersion = profilesSchemaVersion
	store.UpdatedAt = nowRFC3339()
	if store.Items == nil {
		store.Items = []mobileProfileItem{}
	}
	raw, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.profilesPath()), 0o755); err != nil {
		return err
	}
	return os.WriteFile(s.profilesPath(), raw, 0o600)
}

func (s *Service) LoadProfiles() string {
	store, err := s.loadProfileStore()
	if err != nil {
		return encodeCommand(commandResultFor("PROFILE_LOAD_FAILED", nil, err.Error(), false, nil, nil))
	}
	return encodeCommand(commandResultFor("PROFILE_LOAD_OK", store, "配置档案已加载。", true, nil, nil))
}

func (s *Service) SaveCurrentProfile(payloadJSON string) string {
	payload, err := decodeObject(payloadJSON)
	if err != nil {
		return encodeCommand(commandResultFor("PROFILE_INVALID", nil, err.Error(), false, nil, nil))
	}
	snapshot := mapValue(firstNonNil(payload["config_snapshot"], payload["configSnapshot"]))
	if len(snapshot) == 0 {
		return encodeCommand(commandResultFor("PROFILE_INVALID", nil, "缺少 config_snapshot。", false, nil, nil))
	}
	name := strings.TrimSpace(stringValue(payload["name"], ""))
	if name == "" {
		name = "默认档案"
	}
	profileID := strings.TrimSpace(stringValue(firstNonNil(payload["profile_id"], payload["profileId"], payload["id"]), ""))
	if profileID == "" {
		profileID = fmt.Sprintf("profile-%d", time.Now().UnixNano())
	}
	store, err := s.loadProfileStore()
	if err != nil {
		return encodeCommand(commandResultFor("PROFILE_LOAD_FAILED", nil, err.Error(), false, nil, nil))
	}
	now := nowRFC3339()
	updated := false
	for index := range store.Items {
		if store.Items[index].ID == profileID {
			store.Items[index].ConfigSnapshot = snapshot
			store.Items[index].Name = name
			store.Items[index].UpdatedAt = now
			updated = true
		}
	}
	if !updated {
		store.Items = append(store.Items, mobileProfileItem{
			ConfigSnapshot: snapshot,
			CreatedAt:      now,
			ID:             profileID,
			Name:           name,
			UpdatedAt:      now,
		})
	}
	if boolValue(firstNonNil(payload["set_active"], payload["setActive"]), true) {
		store.ActiveProfileID = profileID
	}
	if err := s.saveProfileStore(store); err != nil {
		return encodeCommand(commandResultFor("PROFILE_SAVE_FAILED", nil, err.Error(), false, nil, nil))
	}
	return encodeCommand(commandResultFor("PROFILE_SAVE_OK", store, "配置档案已保存。", true, nil, nil))
}

func (s *Service) SwitchProfile(payloadJSON string) string {
	payload, err := decodeObject(payloadJSON)
	if err != nil {
		return encodeCommand(commandResultFor("PROFILE_INVALID", nil, err.Error(), false, nil, nil))
	}
	profileID := strings.TrimSpace(stringValue(firstNonNil(payload["profile_id"], payload["profileId"], payload["id"]), ""))
	store, err := s.loadProfileStore()
	if err != nil {
		return encodeCommand(commandResultFor("PROFILE_LOAD_FAILED", nil, err.Error(), false, nil, nil))
	}
	for _, item := range store.Items {
		if item.ID != profileID {
			continue
		}
		store.ActiveProfileID = profileID
		if err := s.saveProfileStore(store); err != nil {
			return encodeCommand(commandResultFor("PROFILE_SAVE_FAILED", nil, err.Error(), false, nil, nil))
		}
		if err := s.writeConfigSnapshot(item.ConfigSnapshot); err != nil {
			return encodeCommand(commandResultFor("PROFILE_SWITCH_FAILED", nil, err.Error(), false, nil, nil))
		}
		return encodeCommand(commandResultFor("PROFILE_SWITCH_OK", map[string]any{
			"configPath":      s.configPath(),
			"config_snapshot": item.ConfigSnapshot,
			"profiles":        store,
			"storage":         s.storageStatus(),
		}, "配置档案已切换。", true, nil, nil))
	}
	return encodeCommand(commandResultFor("PROFILE_NOT_FOUND", nil, "未找到配置档案。", false, nil, nil))
}

func (s *Service) DeleteProfile(payloadJSON string) string {
	payload, err := decodeObject(payloadJSON)
	if err != nil {
		return encodeCommand(commandResultFor("PROFILE_INVALID", nil, err.Error(), false, nil, nil))
	}
	profileID := strings.TrimSpace(stringValue(firstNonNil(payload["profile_id"], payload["profileId"], payload["id"]), ""))
	store, err := s.loadProfileStore()
	if err != nil {
		return encodeCommand(commandResultFor("PROFILE_LOAD_FAILED", nil, err.Error(), false, nil, nil))
	}
	nextItems := make([]mobileProfileItem, 0, len(store.Items))
	deleted := false
	for _, item := range store.Items {
		if item.ID == profileID {
			deleted = true
			continue
		}
		nextItems = append(nextItems, item)
	}
	if !deleted {
		return encodeCommand(commandResultFor("PROFILE_NOT_FOUND", nil, "未找到配置档案。", false, nil, nil))
	}
	store.Items = nextItems
	if store.ActiveProfileID == profileID {
		store.ActiveProfileID = ""
	}
	if err := s.saveProfileStore(store); err != nil {
		return encodeCommand(commandResultFor("PROFILE_DELETE_FAILED", nil, err.Error(), false, nil, nil))
	}
	return encodeCommand(commandResultFor("PROFILE_DELETE_OK", store, "配置档案已删除。", true, nil, nil))
}

func (s *Service) writeConfigSnapshot(snapshot map[string]any) error {
	body := map[string]any{
		"config_snapshot": snapshot,
		"saved_at":        nowRFC3339(),
		"schema_version":  schemaVersion,
	}
	raw, err := json.MarshalIndent(body, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.configPath()), 0o755); err != nil {
		return err
	}
	return os.WriteFile(s.configPath(), raw, 0o600)
}

func (s *Service) loadConfigSnapshotFromDisk() (map[string]any, error) {
	raw, err := os.ReadFile(s.configPath())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return defaultConfigSnapshot(), nil
		}
		return nil, err
	}
	var saved map[string]any
	if err := json.Unmarshal(raw, &saved); err != nil {
		return nil, err
	}
	if snapshot := mapValue(saved["config_snapshot"]); len(snapshot) > 0 {
		return snapshot, nil
	}
	return saved, nil
}

func (s *Service) ExportConfig(payloadJSON string) string {
	payload, err := decodeObject(payloadJSON)
	if err != nil {
		return encodeCommand(commandResultFor("CONFIG_EXPORT_INVALID", nil, err.Error(), false, nil, nil))
	}
	snapshot := mapValue(firstNonNil(payload["config_snapshot"], payload["configSnapshot"]))
	if len(snapshot) == 0 {
		snapshot, err = s.loadConfigSnapshotFromDisk()
		if err != nil {
			return encodeCommand(commandResultFor("CONFIG_EXPORT_READ_FAILED", nil, err.Error(), false, nil, nil))
		}
	}
	profiles, err := s.loadProfileStore()
	if err != nil {
		return encodeCommand(commandResultFor("CONFIG_EXPORT_PROFILE_FAILED", nil, err.Error(), false, nil, nil))
	}
	body := map[string]any{
		"app_version":     "mobile",
		"config_snapshot": snapshot,
		"exported_at":     nowRFC3339(),
		"profiles":        profiles,
		"schema_version":  schemaVersion,
		"storage":         s.storageStatus(),
	}
	raw, err := json.MarshalIndent(body, "", "  ")
	if err != nil {
		return encodeCommand(commandResultFor("CONFIG_EXPORT_SERIALIZE_FAILED", nil, err.Error(), false, nil, nil))
	}
	targetPath := strings.TrimSpace(stringValue(firstNonNil(payload["target_path"], payload["targetPath"], payload["path"]), ""))
	if targetPath != "" {
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return encodeCommand(commandResultFor("CONFIG_EXPORT_WRITE_FAILED", nil, err.Error(), false, nil, nil))
		}
		if err := os.WriteFile(targetPath, raw, 0o600); err != nil {
			return encodeCommand(commandResultFor("CONFIG_EXPORT_WRITE_FAILED", nil, err.Error(), false, nil, nil))
		}
	}
	return encodeCommand(commandResultFor("CONFIG_EXPORT_OK", map[string]any{
		"content": string(raw),
		"path":    targetPath,
	}, "完整配置已导出。", true, nil, []string{"导出的配置包含完整 Cloudflare API Token，请仅保存到可信位置。"}))
}

func (s *Service) BackupCurrentConfig(payloadJSON string) string {
	payload, _ := decodeObject(payloadJSON)
	snapshot := mapValue(firstNonNil(payload["config_snapshot"], payload["configSnapshot"]))
	var err error
	if len(snapshot) == 0 {
		snapshot, err = s.loadConfigSnapshotFromDisk()
		if err != nil {
			return encodeCommand(commandResultFor("CONFIG_BACKUP_READ_FAILED", nil, err.Error(), false, nil, nil))
		}
	}
	targetDir := filepath.Join(s.basePath(), "backups")
	targetPath := filepath.Join(targetDir, fmt.Sprintf("config-%s.json", time.Now().Format("20060102-150405")))
	body := map[string]any{
		"backed_up_at":    nowRFC3339(),
		"config_snapshot": snapshot,
		"schema_version":  schemaVersion,
	}
	raw, err := json.MarshalIndent(body, "", "  ")
	if err != nil {
		return encodeCommand(commandResultFor("CONFIG_BACKUP_SERIALIZE_FAILED", nil, err.Error(), false, nil, nil))
	}
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return encodeCommand(commandResultFor("CONFIG_BACKUP_WRITE_FAILED", nil, err.Error(), false, nil, nil))
	}
	if err := os.WriteFile(targetPath, raw, 0o600); err != nil {
		return encodeCommand(commandResultFor("CONFIG_BACKUP_WRITE_FAILED", nil, err.Error(), false, nil, nil))
	}
	return encodeCommand(commandResultFor("CONFIG_BACKUP_OK", map[string]any{
		"path": targetPath,
	}, "当前配置已备份。", true, nil, nil))
}

func (s *Service) activeProfileName() string {
	store, err := s.loadProfileStore()
	if err != nil || strings.TrimSpace(store.ActiveProfileID) == "" {
		return ""
	}
	for _, item := range store.Items {
		if item.ID == store.ActiveProfileID {
			return item.Name
		}
	}
	return ""
}

func sanitizeTemplateFileName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	replacer := strings.NewReplacer("\\", "_", "/", "_", ":", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_")
	value = replacer.Replace(value)
	for strings.Contains(value, "__") {
		value = strings.ReplaceAll(value, "__", "_")
	}
	return strings.TrimSpace(value)
}

func renderExportFileTemplate(template, taskID, profileName string, now time.Time) string {
	template = strings.TrimSpace(template)
	if template == "" {
		return ""
	}
	if profileName == "" {
		profileName = "default"
	}
	replacements := map[string]string{
		"{date}":    now.Format("20060102"),
		"{profile}": sanitizeTemplateFileName(profileName),
		"{task_id}": sanitizeTemplateFileName(taskID),
		"{time}":    now.Format("150405"),
	}
	for key, value := range replacements {
		template = strings.ReplaceAll(template, key, value)
	}
	return sanitizeTemplateFileName(template)
}
