package appcore

import (
	"fmt"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/kernel/appcore/contracts"
	"github.com/axuitomo/CFST-GUI/internal/kernel/appcore/sources"
)

type SourceProfileSaveRequest struct {
	Name           string             `json:"name"`
	ProfileID      string             `json:"profile_id"`
	SetActive      *bool              `json:"set_active"`
	IPMode         *string            `json:"ip_mode"`
	ColoFilter     string             `json:"colo_filter"`
	ColoFilterMode string             `json:"colo_filter_mode"`
	Sources        []contracts.Source `json:"sources"`
}

type SourceProfileUpdateRequest struct {
	Name           string             `json:"name"`
	ProfileID      string             `json:"profile_id"`
	IPMode         *string            `json:"ip_mode"`
	ColoFilter     string             `json:"colo_filter"`
	ColoFilterMode string             `json:"colo_filter_mode"`
	Sources        []contracts.Source `json:"sources"`
}

type SourceProfileStoreSaveRequest struct {
	SourceProfiles contracts.SourceProfileStore `json:"source_profiles"`
	Store          contracts.SourceProfileStore `json:"store"`
}

type SourceProfileSelectRequest struct {
	ProfileID string `json:"profile_id"`
}

func (s *Service) LoadSourceProfiles() CommandResult {
	_, store, result := s.loadSourceProfileContext()
	if !result.OK {
		return result
	}
	return NewCommandResult("SOURCE_PROFILE_LOAD_OK", store, "输入组已加载。", true, nil, nil)
}

func (s *Service) SaveSourceProfile(request SourceProfileSaveRequest) CommandResult {
	snapshot, store, result := s.loadSourceProfileContext()
	if !result.OK {
		return result
	}
	previousStore := sources.CloneSourceProfileStore(store)
	now := s.now().Format(time.RFC3339)
	profileID := strings.TrimSpace(request.ProfileID)
	if profileID == "" {
		profileID = s.newSourceProfileID(0)
	}
	name := strings.TrimSpace(request.Name)
	if name == "" {
		name = "输入组"
	}
	if profileID != sources.DefaultSourceProfileID && sources.IsBlankSourceProfilePlaceholder(store, sources.DefaultSourceProfileID) {
		store.Items = []contracts.SourceProfileItem{}
	}
	mode := sources.RequestedSourceGroupIPMode(request.IPMode, store, profileID, request.Sources)
	profileSources := sources.SourcesWithGroupIPMode(request.Sources, mode)
	coloFilter, coloFilterMode := sources.RequestedSourceGroupColoFilter(request.ColoFilter, request.ColoFilterMode, store, profileID, request.Sources)
	profileSources = sources.SourcesWithGroupColoFilter(profileSources, coloFilter, coloFilterMode)
	updated := false
	for index := range store.Items {
		if store.Items[index].ID != profileID {
			continue
		}
		store.Items[index].Name = name
		store.Items[index].IPMode = mode
		store.Items[index].ColoFilter = coloFilter
		store.Items[index].ColoFilterMode = coloFilterMode
		store.Items[index].Sources = profileSources
		if strings.TrimSpace(store.Items[index].CreatedAt) == "" {
			store.Items[index].CreatedAt = now
		}
		store.Items[index].UpdatedAt = now
		updated = true
		break
	}
	if !updated {
		store.Items = append(store.Items, contracts.SourceProfileItem{
			CreatedAt:      now,
			ID:             profileID,
			Name:           name,
			IPMode:         mode,
			ColoFilter:     coloFilter,
			ColoFilterMode: coloFilterMode,
			Sources:        profileSources,
			UpdatedAt:      now,
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
		sources.SetSnapshotSourceGroup(snapshot, profileSources, mode)
		if _, err := s.SaveConfig(snapshot); err != nil {
			if rollbackErr := s.saveSourceProfiles(previousStore); rollbackErr != nil {
				err = fmt.Errorf("%w; source profile rollback failed: %v", err, rollbackErr)
			}
			return NewCommandResult("SOURCE_PROFILE_SAVE_FAILED", nil, err.Error(), false, nil, nil)
		}
	}
	return NewCommandResult("SOURCE_PROFILE_SAVE_OK", store, "输入组已保存。", true, nil, nil)
}

func (s *Service) UpdateCurrentSourceProfile(request SourceProfileUpdateRequest) CommandResult {
	snapshot, store, result := s.loadSourceProfileContext()
	if !result.OK {
		return result
	}
	previousStore := sources.CloneSourceProfileStore(store)
	profileSources := sources.CloneSources(request.Sources)
	if request.Sources == nil {
		profileSources = sources.SourcesFromAny(snapshot["sources"])
	}
	profileID := strings.TrimSpace(request.ProfileID)
	if profileID == "" {
		profileID = strings.TrimSpace(store.ActiveProfileID)
	}
	mode := sources.RequestedSourceGroupIPMode(request.IPMode, store, profileID, profileSources)
	profileSources = sources.SourcesWithGroupIPMode(profileSources, mode)
	coloFilter, coloFilterMode := sources.RequestedSourceGroupColoFilter(request.ColoFilter, request.ColoFilterMode, store, profileID, profileSources)
	profileSources = sources.SourcesWithGroupColoFilter(profileSources, coloFilter, coloFilterMode)
	if profileID == "" || profileID == sources.DefaultSourceProfileID {
		profileID = s.newSourceProfileID(0)
	}
	if sources.IsBlankSourceProfilePlaceholder(store, sources.DefaultSourceProfileID) {
		store.Items = []contracts.SourceProfileItem{}
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
		store.Items[index].IPMode = mode
		store.Items[index].ColoFilter = coloFilter
		store.Items[index].ColoFilterMode = coloFilterMode
		store.Items[index].Sources = profileSources
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
		store.Items = append(store.Items, contracts.SourceProfileItem{
			CreatedAt:      now,
			ID:             profileID,
			Name:           name,
			IPMode:         mode,
			ColoFilter:     coloFilter,
			ColoFilterMode: coloFilterMode,
			Sources:        profileSources,
			UpdatedAt:      now,
		})
	}
	store.ActiveProfileID = profileID
	if err := s.saveSourceProfiles(store); err != nil {
		return NewCommandResult("SOURCE_PROFILE_SAVE_FAILED", nil, err.Error(), false, nil, nil)
	}
	sources.SetSnapshotSourceGroup(snapshot, profileSources, mode)
	if _, err := s.SaveConfig(snapshot); err != nil {
		if rollbackErr := s.saveSourceProfiles(previousStore); rollbackErr != nil {
			err = fmt.Errorf("%w; source profile rollback failed: %v", err, rollbackErr)
		}
		return NewCommandResult("SOURCE_PROFILE_SAVE_FAILED", nil, err.Error(), false, nil, nil)
	}
	return NewCommandResult("SOURCE_PROFILE_UPDATE_OK", map[string]any{
		"config_snapshot": snapshot,
		"source_profiles": store,
		"sources":         profileSources,
	}, "当前输入组已更新并保存。", true, nil, nil)
}

func (s *Service) SaveSourceProfileStore(request SourceProfileStoreSaveRequest) CommandResult {
	store := request.SourceProfiles
	if len(store.Items) == 0 && len(request.Store.Items) > 0 {
		store = request.Store
	}
	if len(store.Items) == 0 {
		store = sources.BlankSourceProfileStore(s.now().Format(time.RFC3339), sources.DefaultSourceProfilesSchemaVersion)
	}
	store = sources.NormalizeSourceProfileStoreForSave(store, sources.DefaultSourceProfilesSchemaVersion, s.now().Format(time.RFC3339), s.newSourceProfileID)
	if err := s.saveSourceProfiles(store); err != nil {
		return NewCommandResult("SOURCE_PROFILE_STORE_SAVE_FAILED", nil, err.Error(), false, nil, nil)
	}
	return NewCommandResult("SOURCE_PROFILE_STORE_SAVE_OK", store, "输入组列表已恢复。", true, nil, nil)
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
		previousStore := sources.CloneSourceProfileStore(store)
		store.ActiveProfileID = profileID
		if err := s.saveSourceProfiles(store); err != nil {
			return NewCommandResult("SOURCE_PROFILE_SAVE_FAILED", nil, err.Error(), false, nil, nil)
		}
		profileSources := sources.SourcesWithGroupIPMode(item.Sources, item.IPMode)
		sources.SetSnapshotSourceGroup(snapshot, profileSources, item.IPMode)
		if _, err := s.SaveConfig(snapshot); err != nil {
			if rollbackErr := s.saveSourceProfiles(previousStore); rollbackErr != nil {
				err = fmt.Errorf("%w; source profile rollback failed: %v", err, rollbackErr)
			}
			return NewCommandResult("SOURCE_PROFILE_SWITCH_FAILED", nil, err.Error(), false, nil, nil)
		}
		return NewCommandResult("SOURCE_PROFILE_SWITCH_OK", map[string]any{
			"config_snapshot": snapshot,
			"source_profiles": store,
			"sources":         profileSources,
		}, "输入组已切换。", true, nil, nil)
	}
	return NewCommandResult("SOURCE_PROFILE_NOT_FOUND", nil, "未找到输入组。", false, nil, nil)
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
	previousStore := sources.CloneSourceProfileStore(store)
	deletedActive := store.ActiveProfileID == profileID
	items := make([]contracts.SourceProfileItem, 0, len(store.Items))
	deleted := false
	for _, item := range store.Items {
		if item.ID == profileID {
			deleted = true
			continue
		}
		items = append(items, item)
	}
	if !deleted {
		return NewCommandResult("SOURCE_PROFILE_NOT_FOUND", nil, "未找到输入组。", false, nil, nil)
	}
	store.Items = items
	if len(store.Items) == 0 {
		store = sources.BlankSourceProfileStore(s.now().Format(time.RFC3339), sources.DefaultSourceProfilesSchemaVersion)
	} else if deletedActive {
		store.ActiveProfileID = store.Items[0].ID
	}
	if err := s.saveSourceProfiles(store); err != nil {
		return NewCommandResult("SOURCE_PROFILE_DELETE_FAILED", nil, err.Error(), false, nil, nil)
	}
	if deletedActive {
		sources.SetSnapshotSourceGroup(snapshot, sources.ActiveSourceProfileSources(store), sources.ActiveSourceProfileIPMode(store))
		if _, err := s.SaveConfig(snapshot); err != nil {
			if rollbackErr := s.saveSourceProfiles(previousStore); rollbackErr != nil {
				err = fmt.Errorf("%w; source profile rollback failed: %v", err, rollbackErr)
			}
			return NewCommandResult("SOURCE_PROFILE_DELETE_FAILED", nil, err.Error(), false, nil, nil)
		}
	}
	return NewCommandResult("SOURCE_PROFILE_DELETE_OK", store, "输入组已删除。", true, nil, nil)
}

func (s *Service) loadSourceProfileContext() (map[string]any, contracts.SourceProfileStore, CommandResult) {
	config, err := s.LoadConfig()
	if err != nil {
		return nil, contracts.SourceProfileStore{}, NewCommandResult("SOURCE_PROFILE_LOAD_FAILED", nil, err.Error(), false, nil, nil)
	}
	store, err := sources.LoadSourceProfileStore(s.StorageLayout().SourceProfilesPath(), sources.DefaultSourceProfilesSchemaVersion)
	if err != nil {
		return nil, contracts.SourceProfileStore{}, NewCommandResult("SOURCE_PROFILE_LOAD_FAILED", nil, err.Error(), false, nil, nil)
	}
	if len(store.Items) == 0 {
		store = sources.DefaultSourceProfileStoreFromSnapshot(config.Snapshot, nil, sources.DefaultSourceProfilesSchemaVersion)
	} else if strings.TrimSpace(store.ActiveProfileID) == "" {
		store.ActiveProfileID = store.Items[0].ID
	}
	return config.Snapshot, store, CommandResult{OK: true}
}

func (s *Service) saveSourceProfiles(store contracts.SourceProfileStore) error {
	return sources.SaveSourceProfileStoreAt(
		s.StorageLayout().SourceProfilesPath(),
		store,
		sources.DefaultSourceProfilesSchemaVersion,
		s.now(),
	)
}

func (s *Service) newSourceProfileID(index int) string {
	return fmt.Sprintf("source-profile-%d", s.now().UnixNano()+int64(index))
}
