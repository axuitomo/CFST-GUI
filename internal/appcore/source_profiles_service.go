package appcore

import (
	"fmt"
	"strings"
	"time"
)

type SourceProfileSaveRequest struct {
	Name      string   `json:"name"`
	ProfileID string   `json:"profile_id"`
	SetActive *bool    `json:"set_active"`
	Sources   []Source `json:"sources"`
}

type SourceProfileUpdateRequest struct {
	Name      string   `json:"name"`
	ProfileID string   `json:"profile_id"`
	Sources   []Source `json:"sources"`
}

type SourceProfileStoreSaveRequest struct {
	SourceProfiles SourceProfileStore `json:"source_profiles"`
	Store          SourceProfileStore `json:"store"`
}

type SourceProfileSelectRequest struct {
	ProfileID string `json:"profile_id"`
}

func (s *Service) LoadSourceProfiles() CommandResult {
	_, store, result := s.loadSourceProfileContext()
	if !result.OK {
		return result
	}
	return NewCommandResult("SOURCE_PROFILE_LOAD_OK", store, "输入源配置档案已加载。", true, nil, nil)
}

func (s *Service) SaveSourceProfile(request SourceProfileSaveRequest) CommandResult {
	snapshot, store, result := s.loadSourceProfileContext()
	if !result.OK {
		return result
	}
	now := s.now().Format(time.RFC3339)
	profileID := strings.TrimSpace(request.ProfileID)
	if profileID == "" {
		profileID = s.newSourceProfileID(0)
	}
	name := strings.TrimSpace(request.Name)
	if name == "" {
		name = "输入源配置档案"
	}
	if profileID != DefaultSourceProfileID && IsBlankSourceProfilePlaceholder(store, DefaultSourceProfileID) {
		store.Items = []SourceProfileItem{}
	}
	sources := CloneSources(request.Sources)
	updated := false
	for index := range store.Items {
		if store.Items[index].ID != profileID {
			continue
		}
		store.Items[index].Name = name
		store.Items[index].Sources = sources
		if strings.TrimSpace(store.Items[index].CreatedAt) == "" {
			store.Items[index].CreatedAt = now
		}
		store.Items[index].UpdatedAt = now
		updated = true
		break
	}
	if !updated {
		store.Items = append(store.Items, SourceProfileItem{
			CreatedAt: now,
			ID:        profileID,
			Name:      name,
			Sources:   sources,
			UpdatedAt: now,
		})
	}
	setActive := request.SetActive == nil || *request.SetActive
	if setActive {
		store.ActiveProfileID = profileID
	}
	if err := s.saveSourceProfiles(store); err != nil {
		return NewCommandResult("SOURCE_PROFILE_SAVE_FAILED", nil, err.Error(), false, nil, nil)
	}
	if setActive {
		snapshot["sources"] = sources
		if _, err := s.SaveConfig(snapshot); err != nil {
			return NewCommandResult("SOURCE_PROFILE_SAVE_FAILED", nil, err.Error(), false, nil, nil)
		}
	}
	return NewCommandResult("SOURCE_PROFILE_SAVE_OK", store, "输入源配置档案已保存。", true, nil, nil)
}

func (s *Service) UpdateCurrentSourceProfile(request SourceProfileUpdateRequest) CommandResult {
	snapshot, store, result := s.loadSourceProfileContext()
	if !result.OK {
		return result
	}
	sources := CloneSources(request.Sources)
	if request.Sources == nil {
		sources = SourcesFromAny(snapshot["sources"])
	}
	profileID := strings.TrimSpace(request.ProfileID)
	if profileID == "" {
		profileID = strings.TrimSpace(store.ActiveProfileID)
	}
	if profileID == "" || profileID == DefaultSourceProfileID {
		profileID = s.newSourceProfileID(0)
	}
	if IsBlankSourceProfilePlaceholder(store, DefaultSourceProfileID) {
		store.Items = []SourceProfileItem{}
	}
	now := s.now().Format(time.RFC3339)
	name := strings.TrimSpace(request.Name)
	updated := false
	for index := range store.Items {
		if store.Items[index].ID != profileID {
			continue
		}
		if name != "" {
			store.Items[index].Name = name
		}
		if strings.TrimSpace(store.Items[index].Name) == "" {
			store.Items[index].Name = "当前输入源"
		}
		store.Items[index].Sources = sources
		if strings.TrimSpace(store.Items[index].CreatedAt) == "" {
			store.Items[index].CreatedAt = now
		}
		store.Items[index].UpdatedAt = now
		updated = true
		break
	}
	if !updated {
		if name == "" {
			name = "当前输入源"
		}
		store.Items = append(store.Items, SourceProfileItem{
			CreatedAt: now,
			ID:        profileID,
			Name:      name,
			Sources:   sources,
			UpdatedAt: now,
		})
	}
	store.ActiveProfileID = profileID
	if err := s.saveSourceProfiles(store); err != nil {
		return NewCommandResult("SOURCE_PROFILE_SAVE_FAILED", nil, err.Error(), false, nil, nil)
	}
	snapshot["sources"] = sources
	if _, err := s.SaveConfig(snapshot); err != nil {
		return NewCommandResult("SOURCE_PROFILE_SAVE_FAILED", nil, err.Error(), false, nil, nil)
	}
	return NewCommandResult("SOURCE_PROFILE_UPDATE_OK", map[string]any{
		"config_snapshot": snapshot,
		"source_profiles": store,
		"sources":         sources,
	}, "当前输入源配置档案已更新并保存。", true, nil, nil)
}

func (s *Service) SaveSourceProfileStore(request SourceProfileStoreSaveRequest) CommandResult {
	store := request.SourceProfiles
	if len(store.Items) == 0 && len(request.Store.Items) > 0 {
		store = request.Store
	}
	if len(store.Items) == 0 {
		store = BlankSourceProfileStore(s.now().Format(time.RFC3339), DefaultSourceProfilesSchemaVersion)
	}
	store = NormalizeSourceProfileStoreForSave(store, DefaultSourceProfilesSchemaVersion, s.now().Format(time.RFC3339), s.newSourceProfileID)
	if err := s.saveSourceProfiles(store); err != nil {
		return NewCommandResult("SOURCE_PROFILE_STORE_SAVE_FAILED", nil, err.Error(), false, nil, nil)
	}
	return NewCommandResult("SOURCE_PROFILE_STORE_SAVE_OK", store, "输入源配置档案列表已恢复。", true, nil, nil)
}

func (s *Service) SwitchSourceProfile(request SourceProfileSelectRequest) CommandResult {
	profileID := strings.TrimSpace(request.ProfileID)
	if profileID == "" {
		return NewCommandResult("SOURCE_PROFILE_INVALID", nil, "缺少 profile_id。", false, nil, nil)
	}
	snapshot, store, result := s.loadSourceProfileContext()
	if !result.OK {
		return result
	}
	for _, item := range store.Items {
		if item.ID != profileID {
			continue
		}
		store.ActiveProfileID = profileID
		if err := s.saveSourceProfiles(store); err != nil {
			return NewCommandResult("SOURCE_PROFILE_SAVE_FAILED", nil, err.Error(), false, nil, nil)
		}
		sources := CloneSources(item.Sources)
		snapshot["sources"] = sources
		if _, err := s.SaveConfig(snapshot); err != nil {
			return NewCommandResult("SOURCE_PROFILE_SWITCH_FAILED", nil, err.Error(), false, nil, nil)
		}
		return NewCommandResult("SOURCE_PROFILE_SWITCH_OK", map[string]any{
			"config_snapshot": snapshot,
			"source_profiles": store,
			"sources":         sources,
		}, "输入源配置档案已切换。", true, nil, nil)
	}
	return NewCommandResult("SOURCE_PROFILE_NOT_FOUND", nil, "未找到输入源配置档案。", false, nil, nil)
}

func (s *Service) DeleteSourceProfile(request SourceProfileSelectRequest) CommandResult {
	profileID := strings.TrimSpace(request.ProfileID)
	if profileID == "" {
		return NewCommandResult("SOURCE_PROFILE_INVALID", nil, "缺少 profile_id。", false, nil, nil)
	}
	snapshot, store, result := s.loadSourceProfileContext()
	if !result.OK {
		return result
	}
	deletedActive := store.ActiveProfileID == profileID
	items := make([]SourceProfileItem, 0, len(store.Items))
	deleted := false
	for _, item := range store.Items {
		if item.ID == profileID {
			deleted = true
			continue
		}
		items = append(items, item)
	}
	if !deleted {
		return NewCommandResult("SOURCE_PROFILE_NOT_FOUND", nil, "未找到输入源配置档案。", false, nil, nil)
	}
	store.Items = items
	if len(store.Items) == 0 {
		store = BlankSourceProfileStore(s.now().Format(time.RFC3339), DefaultSourceProfilesSchemaVersion)
	} else if deletedActive {
		store.ActiveProfileID = store.Items[0].ID
	}
	if err := s.saveSourceProfiles(store); err != nil {
		return NewCommandResult("SOURCE_PROFILE_DELETE_FAILED", nil, err.Error(), false, nil, nil)
	}
	if deletedActive {
		snapshot["sources"] = ActiveSourceProfileSources(store)
		if _, err := s.SaveConfig(snapshot); err != nil {
			return NewCommandResult("SOURCE_PROFILE_DELETE_FAILED", nil, err.Error(), false, nil, nil)
		}
	}
	return NewCommandResult("SOURCE_PROFILE_DELETE_OK", store, "输入源配置档案已删除。", true, nil, nil)
}

func (s *Service) loadSourceProfileContext() (map[string]any, SourceProfileStore, CommandResult) {
	config, err := s.LoadConfig()
	if err != nil {
		return nil, SourceProfileStore{}, NewCommandResult("SOURCE_PROFILE_LOAD_FAILED", nil, err.Error(), false, nil, nil)
	}
	store, err := LoadSourceProfileStore(s.StorageLayout().SourceProfilesPath(), DefaultSourceProfilesSchemaVersion)
	if err != nil {
		return nil, SourceProfileStore{}, NewCommandResult("SOURCE_PROFILE_LOAD_FAILED", nil, err.Error(), false, nil, nil)
	}
	if len(store.Items) == 0 {
		store = BlankSourceProfileStore(s.now().Format(time.RFC3339), DefaultSourceProfilesSchemaVersion)
	} else if strings.TrimSpace(store.ActiveProfileID) == "" {
		store.ActiveProfileID = store.Items[0].ID
	}
	return config.Snapshot, store, CommandResult{OK: true}
}

func (s *Service) saveSourceProfiles(store SourceProfileStore) error {
	return saveSourceProfileStoreAt(
		s.StorageLayout().SourceProfilesPath(),
		store,
		DefaultSourceProfilesSchemaVersion,
		s.now(),
	)
}

func (s *Service) newSourceProfileID(index int) string {
	return fmt.Sprintf("source-profile-%d", s.now().UnixNano()+int64(index))
}
