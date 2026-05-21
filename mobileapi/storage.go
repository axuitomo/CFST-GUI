package mobileapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/appcore"
	"github.com/axuitomo/CFST-GUI/internal/probecore"
)

const (
	storageSchemaVersion        = "cfst-gui-storage-v1"
	profilesSchemaVersion       = "cfst-gui-profiles-v1"
	sourceProfilesSchemaVersion = "cfst-gui-source-profiles-v1"
	defaultSourceProfileID      = "source-profile-default"
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

type mobileProfileItem = appcore.ProfileItem
type mobileProfileStore = appcore.ProfileStore
type mobileSourceProfileItem = appcore.SourceProfileItem
type mobileSourceProfileStore = appcore.SourceProfileStore

func (s *Service) storageBootstrapPath() string {
	return filepath.Join(s.basePath(), "storage.json")
}

func (s *Service) storageStatus() map[string]any {
	health := checkMobileStorageHealth(s.basePath())
	return map[string]any{
		"backend":         "private",
		"bootstrap_path":  s.storageBootstrapPath(),
		"current_dir":     s.basePath(),
		"default_dir":     s.basePath(),
		"display_name":    "",
		"health":          health,
		"last_sync_at":    "",
		"last_sync_error": "",
		"log_uri":         "",
		"permission_ok":   true,
		"portable_mode":   false,
		"runtime_dir":     s.basePath(),
		"setup_completed": true,
		"setup_required":  false,
		"storage_uri":     "",
		"writable":        health.Writable,
	}
}

func (s *Service) readStorageBootstrap() (mobileStorageBootstrap, error) {
	raw, err := os.ReadFile(s.storageBootstrapPath())
	if err != nil {
		return mobileStorageBootstrap{}, err
	}
	var bootstrap mobileStorageBootstrap
	if _, err := appcore.UnmarshalJSONCompat(raw, &bootstrap); err != nil {
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
	return appcore.WriteFileAtomic(s.storageBootstrapPath(), raw, 0o600)
}

func checkMobileStorageHealth(path string) mobileStorageHealth {
	health := mobileStorageHealth{
		CheckedAt: time.Now().Format(time.RFC3339),
		FreeBytes: -1,
		Path:      path,
	}
	if strings.TrimSpace(path) == "" {
		health.Message = "应用私有目录为空。"
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
	health.Message = "应用私有目录可用。"
	return health
}

func (s *Service) SetStorageDirectory(payloadJSON string) string {
	_, err := decodeObject(payloadJSON)
	if err != nil {
		return encodeCommand(commandResultFor("STORAGE_SET_FAILED", nil, err.Error(), false, nil, nil))
	}
	bootstrap := mobileStorageBootstrap{
		PortableMode:   false,
		SchemaVersion:  storageSchemaVersion,
		SetupCompleted: true,
		StorageDir:     s.basePath(),
	}
	if err := s.writeStorageBootstrap(bootstrap); err != nil {
		return encodeCommand(commandResultFor("STORAGE_SET_FAILED", nil, err.Error(), false, nil, nil))
	}
	return encodeCommand(commandResultFor("STORAGE_SET_DEPRECATED", map[string]any{
		"migration": map[string]any{"copied": []string{}, "failed": []string{}, "skipped": []string{}},
		"storage":   s.storageStatus(),
	}, "当前版本不再支持自定义储存目录，Android 固定使用应用私有目录。", true, nil, nil))
}

func (s *Service) CheckStorageHealth(payloadJSON string) string {
	_, _ = decodeObject(payloadJSON)
	return encodeCommand(commandResultFor("STORAGE_HEALTH_READY", map[string]any{
		"health":  checkMobileStorageHealth(s.basePath()),
		"storage": s.storageStatus(),
	}, "应用私有目录健康检查已完成。", true, nil, nil))
}

func (s *Service) profilesPath() string {
	return filepath.Join(s.basePath(), "profiles.json")
}

func (s *Service) loadProfileStore() (mobileProfileStore, error) {
	return appcore.LoadProfileStore(s.profilesPath(), profilesSchemaVersion, sanitizeMobileConfigSnapshot)
}

func (s *Service) saveProfileStore(store mobileProfileStore) error {
	return appcore.SaveProfileStore(s.profilesPath(), store, profilesSchemaVersion, sanitizeMobileConfigSnapshot)
}

func (s *Service) sourceProfilesPath() string {
	return filepath.Join(s.basePath(), "source-profiles.json")
}

func (s *Service) loadSourceProfileStore() (mobileSourceProfileStore, error) {
	return appcore.LoadSourceProfileStore(s.sourceProfilesPath(), sourceProfilesSchemaVersion)
}

func (s *Service) saveSourceProfileStore(store mobileSourceProfileStore) error {
	return appcore.SaveSourceProfileStore(s.sourceProfilesPath(), store, sourceProfilesSchemaVersion)
}

func (s *Service) loadSourceProfileStoreForSnapshot(_ map[string]any) (mobileSourceProfileStore, error) {
	store, err := s.loadSourceProfileStore()
	if err != nil {
		return store, err
	}
	if len(store.Items) == 0 {
		return blankMobileSourceProfileStore(), nil
	}
	if strings.TrimSpace(store.ActiveProfileID) == "" {
		store.ActiveProfileID = store.Items[0].ID
	}
	return store, nil
}

func blankMobileSourceProfileStore() mobileSourceProfileStore {
	return appcore.BlankSourceProfileStore(nowRFC3339(), sourceProfilesSchemaVersion)
}

func defaultMobileSourceProfileStoreFromSnapshot(snapshot map[string]any) mobileSourceProfileStore {
	return appcore.DefaultSourceProfileStoreFromSnapshot(snapshot, defaultConfigSnapshot(), sourceProfilesSchemaVersion)
}

func normalizeMobileSourceProfileStoreForSave(store mobileSourceProfileStore) mobileSourceProfileStore {
	return appcore.NormalizeSourceProfileStoreForSave(store, sourceProfilesSchemaVersion, nowRFC3339(), func(index int) string {
		return fmt.Sprintf("source-profile-%d", time.Now().UnixNano()+int64(index))
	})
}

func activeMobileSourceProfileSources(store mobileSourceProfileStore) []desktopSource {
	return appcore.ActiveSourceProfileSources(store)
}

func isBlankMobileSourceProfilePlaceholder(store mobileSourceProfileStore) bool {
	return appcore.IsBlankSourceProfilePlaceholder(store, defaultSourceProfileID)
}

func mobileSourceProfileStoreFromAny(value any) mobileSourceProfileStore {
	return appcore.SourceProfileStoreFromAny(value)
}

func mobileSourcesFromAny(value any) []desktopSource {
	return appcore.SourcesFromAny(value)
}

func cloneMobileSources(sources []desktopSource) []desktopSource {
	return appcore.CloneSources(sources)
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
	snapshot = sanitizeMobileConfigSnapshot(snapshot)
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

func (s *Service) UpdateCurrentProfile(payloadJSON string) string {
	payload, err := decodeObject(payloadJSON)
	if err != nil {
		return encodeCommand(commandResultFor("PROFILE_INVALID", nil, err.Error(), false, nil, nil))
	}
	snapshot := mapValue(firstNonNil(payload["config_snapshot"], payload["configSnapshot"]))
	if len(snapshot) == 0 {
		return encodeCommand(commandResultFor("PROFILE_INVALID", nil, "缺少 config_snapshot。", false, nil, nil))
	}
	snapshot = sanitizeMobileConfigSnapshot(snapshot)
	store, err := s.loadProfileStore()
	if err != nil {
		return encodeCommand(commandResultFor("PROFILE_LOAD_FAILED", nil, err.Error(), false, nil, nil))
	}
	now := nowRFC3339()
	profileID := strings.TrimSpace(stringValue(firstNonNil(payload["profile_id"], payload["profileId"], payload["id"], store.ActiveProfileID), ""))
	name := strings.TrimSpace(stringValue(payload["name"], ""))
	store, _ = probecore.UpdateCurrentProfileStore(probecore.CurrentProfileUpdateOptions[mobileProfileStore, mobileProfileItem, map[string]any]{
		Store:       store,
		Value:       snapshot,
		ProfileID:   profileID,
		Name:        name,
		Now:         now,
		DefaultName: "当前配置",
		Items: func(store mobileProfileStore) []mobileProfileItem {
			return store.Items
		},
		SetItems: func(store *mobileProfileStore, items []mobileProfileItem) {
			store.Items = items
		},
		ActiveID: func(store mobileProfileStore) string {
			return store.ActiveProfileID
		},
		SetActiveID: func(store *mobileProfileStore, profileID string) {
			store.ActiveProfileID = profileID
		},
		ItemID: func(item mobileProfileItem) string {
			return item.ID
		},
		UpdateItem: func(item *mobileProfileItem, patch probecore.ProfileItemPatch[map[string]any]) {
			item.ConfigSnapshot = patch.Value
			if patch.Name != "" {
				item.Name = patch.Name
			}
			if strings.TrimSpace(item.Name) == "" {
				item.Name = "当前配置"
			}
			if item.CreatedAt == "" {
				item.CreatedAt = patch.Now
			}
			item.UpdatedAt = patch.Now
		},
		NewItem: func(patch probecore.ProfileItemPatch[map[string]any]) mobileProfileItem {
			return mobileProfileItem{
				ConfigSnapshot: patch.Value,
				CreatedAt:      patch.Now,
				ID:             patch.ID,
				Name:           patch.Name,
				UpdatedAt:      patch.Now,
			}
		},
		NewProfileID: func() string {
			return fmt.Sprintf("profile-%d", time.Now().UnixNano())
		},
	})
	if err := s.saveProfileStore(store); err != nil {
		return encodeCommand(commandResultFor("PROFILE_SAVE_FAILED", nil, err.Error(), false, nil, nil))
	}
	return encodeCommand(commandResultFor("PROFILE_UPDATE_OK", store, "当前配置档案已更新并保存。", true, nil, nil))
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
		snapshot := sanitizeMobileConfigSnapshot(item.ConfigSnapshot)
		return encodeCommand(commandResultFor("PROFILE_SWITCH_OK", map[string]any{
			"configPath":      s.configPath(),
			"config_snapshot": snapshot,
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

func (s *Service) LoadSourceProfiles() string {
	snapshot, err := s.loadConfigSnapshotFromDisk()
	if err != nil {
		return encodeCommand(commandResultFor("SOURCE_PROFILE_LOAD_FAILED", nil, err.Error(), false, nil, nil))
	}
	store, err := s.loadSourceProfileStoreForSnapshot(snapshot)
	if err != nil {
		return encodeCommand(commandResultFor("SOURCE_PROFILE_LOAD_FAILED", nil, err.Error(), false, nil, nil))
	}
	return encodeCommand(commandResultFor("SOURCE_PROFILE_LOAD_OK", store, "输入源配置档案已加载。", true, nil, nil))
}

func (s *Service) SaveSourceProfile(payloadJSON string) string {
	payload, err := decodeObject(payloadJSON)
	if err != nil {
		return encodeCommand(commandResultFor("SOURCE_PROFILE_INVALID", nil, err.Error(), false, nil, nil))
	}
	snapshot, err := s.loadConfigSnapshotFromDisk()
	if err != nil {
		return encodeCommand(commandResultFor("SOURCE_PROFILE_LOAD_FAILED", nil, err.Error(), false, nil, nil))
	}
	store, err := s.loadSourceProfileStoreForSnapshot(snapshot)
	if err != nil {
		return encodeCommand(commandResultFor("SOURCE_PROFILE_LOAD_FAILED", nil, err.Error(), false, nil, nil))
	}
	sources := mobileSourcesFromAny(firstNonNil(payload["sources"], payload["Sources"]))
	name := strings.TrimSpace(stringValue(payload["name"], ""))
	profileID := strings.TrimSpace(stringValue(firstNonNil(payload["profile_id"], payload["profileId"], payload["id"]), ""))
	if name == "" {
		name = "输入源档案"
	}
	if profileID == "" {
		profileID = fmt.Sprintf("source-profile-%d", time.Now().UnixNano())
	}
	if profileID != defaultSourceProfileID && isBlankMobileSourceProfilePlaceholder(store) {
		store.Items = []mobileSourceProfileItem{}
	}
	now := nowRFC3339()
	updated := false
	for index := range store.Items {
		if store.Items[index].ID != profileID {
			continue
		}
		store.Items[index].Name = name
		store.Items[index].Sources = cloneMobileSources(sources)
		if store.Items[index].CreatedAt == "" {
			store.Items[index].CreatedAt = now
		}
		store.Items[index].UpdatedAt = now
		updated = true
		break
	}
	if !updated {
		store.Items = append(store.Items, mobileSourceProfileItem{
			CreatedAt: now,
			ID:        profileID,
			Name:      name,
			Sources:   cloneMobileSources(sources),
			UpdatedAt: now,
		})
	}
	setActive := boolValue(firstNonNil(payload["set_active"], payload["setActive"]), true)
	if setActive {
		store.ActiveProfileID = profileID
	}
	if err := s.saveSourceProfileStore(store); err != nil {
		return encodeCommand(commandResultFor("SOURCE_PROFILE_SAVE_FAILED", nil, err.Error(), false, nil, nil))
	}
	if setActive {
		snapshot["sources"] = cloneMobileSources(sources)
		if err := s.writeConfigSnapshot(snapshot); err != nil {
			return encodeCommand(commandResultFor("SOURCE_PROFILE_SAVE_FAILED", nil, err.Error(), false, nil, nil))
		}
	}
	return encodeCommand(commandResultFor("SOURCE_PROFILE_SAVE_OK", store, "输入源配置档案已保存。", true, nil, nil))
}

func (s *Service) UpdateCurrentSourceProfile(payloadJSON string) string {
	payload, err := decodeObject(payloadJSON)
	if err != nil {
		return encodeCommand(commandResultFor("SOURCE_PROFILE_INVALID", nil, err.Error(), false, nil, nil))
	}
	snapshot, err := s.loadConfigSnapshotFromDisk()
	if err != nil {
		return encodeCommand(commandResultFor("SOURCE_PROFILE_LOAD_FAILED", nil, err.Error(), false, nil, nil))
	}
	store, err := s.loadSourceProfileStoreForSnapshot(snapshot)
	if err != nil {
		return encodeCommand(commandResultFor("SOURCE_PROFILE_LOAD_FAILED", nil, err.Error(), false, nil, nil))
	}
	sources := mobileSourcesFromAny(firstNonNil(payload["sources"], payload["Sources"], snapshot["sources"]))
	now := nowRFC3339()
	profileID := strings.TrimSpace(stringValue(firstNonNil(payload["profile_id"], payload["profileId"], payload["id"], store.ActiveProfileID), ""))
	name := strings.TrimSpace(stringValue(payload["name"], ""))
	store, _ = probecore.UpdateCurrentProfileStore(probecore.CurrentProfileUpdateOptions[mobileSourceProfileStore, mobileSourceProfileItem, []desktopSource]{
		Store:       store,
		Value:       cloneMobileSources(sources),
		ProfileID:   profileID,
		Name:        name,
		Now:         now,
		DefaultName: "当前输入源",
		Items: func(store mobileSourceProfileStore) []mobileSourceProfileItem {
			return store.Items
		},
		SetItems: func(store *mobileSourceProfileStore, items []mobileSourceProfileItem) {
			store.Items = items
		},
		ActiveID: func(store mobileSourceProfileStore) string {
			return store.ActiveProfileID
		},
		SetActiveID: func(store *mobileSourceProfileStore, profileID string) {
			store.ActiveProfileID = profileID
		},
		ItemID: func(item mobileSourceProfileItem) string {
			return item.ID
		},
		UpdateItem: func(item *mobileSourceProfileItem, patch probecore.ProfileItemPatch[[]desktopSource]) {
			if patch.Name != "" {
				item.Name = patch.Name
			}
			if strings.TrimSpace(item.Name) == "" {
				item.Name = "当前输入源"
			}
			item.Sources = cloneMobileSources(patch.Value)
			if item.CreatedAt == "" {
				item.CreatedAt = patch.Now
			}
			item.UpdatedAt = patch.Now
		},
		NewItem: func(patch probecore.ProfileItemPatch[[]desktopSource]) mobileSourceProfileItem {
			return mobileSourceProfileItem{
				CreatedAt: patch.Now,
				ID:        patch.ID,
				Name:      patch.Name,
				Sources:   cloneMobileSources(patch.Value),
				UpdatedAt: patch.Now,
			}
		},
		NewProfileID: func() string {
			return fmt.Sprintf("source-profile-%d", time.Now().UnixNano())
		},
		ForceNewID: func(profileID string) bool {
			return profileID == defaultSourceProfileID
		},
		DropPlaceholder: func(store mobileSourceProfileStore, profileID string) bool {
			return profileID != defaultSourceProfileID && isBlankMobileSourceProfilePlaceholder(store)
		},
	})
	if err := s.saveSourceProfileStore(store); err != nil {
		return encodeCommand(commandResultFor("SOURCE_PROFILE_SAVE_FAILED", nil, err.Error(), false, nil, nil))
	}
	snapshot["sources"] = cloneMobileSources(sources)
	if err := s.writeConfigSnapshot(snapshot); err != nil {
		return encodeCommand(commandResultFor("SOURCE_PROFILE_SAVE_FAILED", nil, err.Error(), false, nil, nil))
	}
	return encodeCommand(commandResultFor("SOURCE_PROFILE_UPDATE_OK", map[string]any{
		"config_snapshot": snapshot,
		"source_profiles": store,
		"sources":         cloneMobileSources(sources),
	}, "当前输入源档案已更新并保存。", true, nil, nil))
}

func (s *Service) SaveSourceProfileStore(payloadJSON string) string {
	payload, err := decodeObject(payloadJSON)
	if err != nil {
		return encodeCommand(commandResultFor("SOURCE_PROFILE_INVALID", nil, err.Error(), false, nil, nil))
	}
	rawStore := firstNonNil(payload["source_profiles"], payload["sourceProfiles"], payload["store"])
	store := mobileSourceProfileStoreFromAny(rawStore)
	if len(store.Items) == 0 {
		store = blankMobileSourceProfileStore()
	}
	store = normalizeMobileSourceProfileStoreForSave(store)
	if err := s.saveSourceProfileStore(store); err != nil {
		return encodeCommand(commandResultFor("SOURCE_PROFILE_STORE_SAVE_FAILED", nil, err.Error(), false, nil, nil))
	}
	return encodeCommand(commandResultFor("SOURCE_PROFILE_STORE_SAVE_OK", store, "输入源配置档案列表已恢复。", true, nil, nil))
}

func (s *Service) SwitchSourceProfile(payloadJSON string) string {
	payload, err := decodeObject(payloadJSON)
	if err != nil {
		return encodeCommand(commandResultFor("SOURCE_PROFILE_INVALID", nil, err.Error(), false, nil, nil))
	}
	profileID := strings.TrimSpace(stringValue(firstNonNil(payload["profile_id"], payload["profileId"], payload["id"]), ""))
	if profileID == "" {
		return encodeCommand(commandResultFor("SOURCE_PROFILE_INVALID", nil, "缺少 profile_id。", false, nil, nil))
	}
	snapshot, err := s.loadConfigSnapshotFromDisk()
	if err != nil {
		return encodeCommand(commandResultFor("SOURCE_PROFILE_LOAD_FAILED", nil, err.Error(), false, nil, nil))
	}
	store, err := s.loadSourceProfileStoreForSnapshot(snapshot)
	if err != nil {
		return encodeCommand(commandResultFor("SOURCE_PROFILE_LOAD_FAILED", nil, err.Error(), false, nil, nil))
	}
	for _, item := range store.Items {
		if item.ID != profileID {
			continue
		}
		store.ActiveProfileID = profileID
		if err := s.saveSourceProfileStore(store); err != nil {
			return encodeCommand(commandResultFor("SOURCE_PROFILE_SAVE_FAILED", nil, err.Error(), false, nil, nil))
		}
		snapshot["sources"] = cloneMobileSources(item.Sources)
		if err := s.writeConfigSnapshot(snapshot); err != nil {
			return encodeCommand(commandResultFor("SOURCE_PROFILE_SWITCH_FAILED", nil, err.Error(), false, nil, nil))
		}
		return encodeCommand(commandResultFor("SOURCE_PROFILE_SWITCH_OK", map[string]any{
			"config_snapshot": snapshot,
			"source_profiles": store,
			"sources":         cloneMobileSources(item.Sources),
		}, "输入源配置档案已切换。", true, nil, nil))
	}
	return encodeCommand(commandResultFor("SOURCE_PROFILE_NOT_FOUND", nil, "未找到输入源配置档案。", false, nil, nil))
}

func (s *Service) DeleteSourceProfile(payloadJSON string) string {
	payload, err := decodeObject(payloadJSON)
	if err != nil {
		return encodeCommand(commandResultFor("SOURCE_PROFILE_INVALID", nil, err.Error(), false, nil, nil))
	}
	profileID := strings.TrimSpace(stringValue(firstNonNil(payload["profile_id"], payload["profileId"], payload["id"]), ""))
	if profileID == "" {
		return encodeCommand(commandResultFor("SOURCE_PROFILE_INVALID", nil, "缺少 profile_id。", false, nil, nil))
	}
	snapshot, err := s.loadConfigSnapshotFromDisk()
	if err != nil {
		return encodeCommand(commandResultFor("SOURCE_PROFILE_LOAD_FAILED", nil, err.Error(), false, nil, nil))
	}
	store, err := s.loadSourceProfileStoreForSnapshot(snapshot)
	if err != nil {
		return encodeCommand(commandResultFor("SOURCE_PROFILE_LOAD_FAILED", nil, err.Error(), false, nil, nil))
	}
	deletedActiveProfile := store.ActiveProfileID == profileID
	nextItems := make([]mobileSourceProfileItem, 0, len(store.Items))
	deleted := false
	for _, item := range store.Items {
		if item.ID == profileID {
			deleted = true
			continue
		}
		nextItems = append(nextItems, item)
	}
	if !deleted {
		return encodeCommand(commandResultFor("SOURCE_PROFILE_NOT_FOUND", nil, "未找到输入源配置档案。", false, nil, nil))
	}
	store.Items = nextItems
	if len(store.Items) == 0 {
		store = blankMobileSourceProfileStore()
	} else if store.ActiveProfileID == profileID {
		store.ActiveProfileID = store.Items[0].ID
	}
	if err := s.saveSourceProfileStore(store); err != nil {
		return encodeCommand(commandResultFor("SOURCE_PROFILE_DELETE_FAILED", nil, err.Error(), false, nil, nil))
	}
	if deletedActiveProfile {
		snapshot["sources"] = cloneMobileSources(activeMobileSourceProfileSources(store))
		if err := s.writeConfigSnapshot(snapshot); err != nil {
			return encodeCommand(commandResultFor("SOURCE_PROFILE_DELETE_FAILED", nil, err.Error(), false, nil, nil))
		}
	}
	return encodeCommand(commandResultFor("SOURCE_PROFILE_DELETE_OK", store, "输入源配置档案已删除。", true, nil, nil))
}

func (s *Service) writeConfigSnapshot(snapshot map[string]any) error {
	return appcore.WriteConfigSnapshot(s.configPath(), snapshot, schemaVersion, sanitizeMobileConfigSnapshot)
}

func (s *Service) loadConfigSnapshotFromDisk() (map[string]any, error) {
	return appcore.LoadConfigSnapshotFromDisk(s.configPath(), defaultConfigSnapshot, sanitizeMobileConfigSnapshot)
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
	} else {
		snapshot = sanitizeMobileConfigSnapshot(snapshot)
	}
	profiles, err := s.loadProfileStore()
	if err != nil {
		return encodeCommand(commandResultFor("CONFIG_EXPORT_PROFILE_FAILED", nil, err.Error(), false, nil, nil))
	}
	sourceProfiles, err := s.loadSourceProfileStoreForSnapshot(snapshot)
	if err != nil {
		return encodeCommand(commandResultFor("CONFIG_EXPORT_SOURCE_PROFILE_FAILED", nil, err.Error(), false, nil, nil))
	}
	body := map[string]any{
		"app_version":     "mobile",
		"config_snapshot": snapshot,
		"exported_at":     nowRFC3339(),
		"profiles":        profiles,
		"source_profiles": sourceProfiles,
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
	} else {
		snapshot = sanitizeMobileConfigSnapshot(snapshot)
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
	return probecore.SanitizeTemplateFileName(value)
}

func renderExportFileTemplate(template, taskID, profileName string, now time.Time) string {
	return probecore.RenderExportFileTemplate(template, taskID, profileName, now)
}
