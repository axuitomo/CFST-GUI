package sources

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/kernel/appcore/contracts"
	"github.com/axuitomo/CFST-GUI/internal/kernel/domain/probe/task"
	"github.com/axuitomo/CFST-GUI/internal/kernel/infra/fileio"
	"github.com/axuitomo/CFST-GUI/internal/kernel/infra/jsoncompat"
)

const (
	DefaultSourceProfileID             = "source-profile-default"
	DefaultSourceProfilesSchemaVersion = "cfst-gui-source-profiles-v1"
	RecentRunSourceProfileID           = "source-profile-recent-run"
	RecentRunSourceProfileName         = "最近运行输入源"
	SchedulerSourceProfileActionUpdate = "update_recent_run_source_profile"
)

func ApplySchedulerSourceProfileAction(store contracts.SourceProfileStore, sources []contracts.Source, action string, now time.Time) (contracts.SourceProfileStore, string, bool) {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "none", "disabled", "skip", "skipped":
		return store, "skipped", false
	case SchedulerSourceProfileActionUpdate:
	default:
		return store, "unsupported", false
	}
	timestamp := now.Format(time.RFC3339)
	coloFilter, coloFilterMode := SourceGroupColoFilterSettings(sources)
	for index := range store.Items {
		if store.Items[index].ID != RecentRunSourceProfileID {
			continue
		}
		store.Items[index].Name = RecentRunSourceProfileName
		store.Items[index].Sources = CloneSources(sources)
		store.Items[index].ColoFilter = coloFilter
		store.Items[index].ColoFilterMode = coloFilterMode
		store.Items[index].IPMode = SourceGroupIPMode("", sources)
		if strings.TrimSpace(store.Items[index].CreatedAt) == "" {
			store.Items[index].CreatedAt = timestamp
		}
		store.Items[index].UpdatedAt = timestamp
		return store, "updated", true
	}
	store.Items = append(store.Items, contracts.SourceProfileItem{
		CreatedAt:      timestamp,
		ID:             RecentRunSourceProfileID,
		Name:           RecentRunSourceProfileName,
		IPMode:         SourceGroupIPMode("", sources),
		ColoFilter:     coloFilter,
		ColoFilterMode: coloFilterMode,
		Sources:        CloneSources(sources),
		UpdatedAt:      timestamp,
	})
	return store, "created", true
}

func LoadSourceProfileStore(path string, schemaVersion string) (contracts.SourceProfileStore, error) {
	store := contracts.SourceProfileStore{
		Items:         []contracts.SourceProfileItem{},
		SchemaVersion: schemaVersion,
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return store, nil
		}
		return store, err
	}
	if _, err := jsoncompat.UnmarshalJSONCompat(raw, &store); err != nil {
		return store, err
	}
	if store.Items == nil {
		store.Items = []contracts.SourceProfileItem{}
	}
	if store.SchemaVersion == "" {
		store.SchemaVersion = schemaVersion
	}
	dedupeSourceProfileIDs(&store)
	NormalizeSourceProfileIPModes(&store)
	NormalizeSourceProfileColoFilters(&store)
	return store, nil
}

func SaveSourceProfileStore(path string, store contracts.SourceProfileStore, schemaVersion string) error {
	return SaveSourceProfileStoreAt(path, store, schemaVersion, time.Now())
}

func SaveSourceProfileStoreAt(path string, store contracts.SourceProfileStore, schemaVersion string, now time.Time) error {
	store.SchemaVersion = schemaVersion
	store.UpdatedAt = now.Format(time.RFC3339)
	if store.Items == nil {
		store.Items = []contracts.SourceProfileItem{}
	}
	dedupeSourceProfileIDs(&store)
	NormalizeSourceProfileIPModes(&store)
	NormalizeSourceProfileColoFilters(&store)
	raw, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	return fileio.WriteFileAtomic(path, raw, 0o600)
}

func BlankSourceProfileStore(now string, schemaVersion string) contracts.SourceProfileStore {
	now = strings.TrimSpace(now)
	if now == "" {
		now = time.Now().Format(time.RFC3339)
	}
	return contracts.SourceProfileStore{
		ActiveProfileID: DefaultSourceProfileID,
		Items: []contracts.SourceProfileItem{
			{
				CreatedAt:      now,
				ID:             DefaultSourceProfileID,
				Name:           "默认输入源",
				IPMode:         "traverse",
				ColoFilterMode: task.ColoFilterModeAllow,
				Sources:        []contracts.Source{},
				UpdatedAt:      now,
			},
		},
		SchemaVersion: schemaVersion,
		UpdatedAt:     now,
	}
}

func DefaultSourceProfileStoreFromSnapshot(snapshot map[string]any, defaultSnapshot map[string]any, schemaVersion string) contracts.SourceProfileStore {
	sources := SourcesFromAny(snapshot["sources"])
	if len(sources) == 0 {
		sources = SourcesFromAny(defaultSnapshot["sources"])
	}
	coloFilter, coloFilterMode := SourceGroupColoFilterSettings(sources)
	return contracts.SourceProfileStore{
		ActiveProfileID: DefaultSourceProfileID,
		Items: []contracts.SourceProfileItem{
			{
				ID:             DefaultSourceProfileID,
				Name:           "默认输入源",
				IPMode:         SourceGroupIPMode(SourceGroupModeFromSnapshot(snapshot), sources),
				ColoFilter:     coloFilter,
				ColoFilterMode: coloFilterMode,
				Sources:        CloneSources(sources),
			},
		},
		SchemaVersion: schemaVersion,
	}
}

func dedupeSourceProfileIDs(store *contracts.SourceProfileStore) {
	seen := make(map[string]struct{}, len(store.Items))
	items := store.Items[:0]
	for _, item := range store.Items {
		id := strings.TrimSpace(item.ID)
		if id != "" {
			if _, exists := seen[id]; exists {
				continue
			}
			seen[id] = struct{}{}
		}
		items = append(items, item)
	}
	store.Items = items
}

func NormalizeSourceProfileStoreForSave(store contracts.SourceProfileStore, schemaVersion string, now string, newProfileID func(index int) string) contracts.SourceProfileStore {
	if store.SchemaVersion == "" {
		store.SchemaVersion = schemaVersion
	}
	if strings.TrimSpace(store.UpdatedAt) == "" {
		store.UpdatedAt = now
	}
	if store.Items == nil {
		store.Items = []contracts.SourceProfileItem{}
	}
	for index := range store.Items {
		if strings.TrimSpace(store.Items[index].ID) == "" {
			if newProfileID != nil {
				store.Items[index].ID = newProfileID(index)
			}
			if strings.TrimSpace(store.Items[index].ID) == "" {
				store.Items[index].ID = fmt.Sprintf("source-profile-%d", time.Now().UnixNano()+int64(index))
			}
		}
		if strings.TrimSpace(store.Items[index].Name) == "" {
			store.Items[index].Name = fmt.Sprintf("输入组 %d", index+1)
		}
		if store.Items[index].Sources == nil {
			store.Items[index].Sources = []contracts.Source{}
		}
		if store.Items[index].CreatedAt == "" {
			store.Items[index].CreatedAt = now
		}
		if store.Items[index].UpdatedAt == "" {
			store.Items[index].UpdatedAt = now
		}
		store.Items[index].Sources = CloneSources(store.Items[index].Sources)
	}
	dedupeSourceProfileIDs(&store)
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
	NormalizeSourceProfileIPModes(&store)
	NormalizeSourceProfileColoFilters(&store)
	return store
}

func ActiveSourceProfileSources(store contracts.SourceProfileStore) []contracts.Source {
	for _, item := range store.Items {
		if item.ID == store.ActiveProfileID {
			return SourcesWithGroupIPMode(item.Sources, item.IPMode)
		}
	}
	if len(store.Items) == 0 {
		return []contracts.Source{}
	}
	return SourcesWithGroupIPMode(store.Items[0].Sources, store.Items[0].IPMode)
}

func IsBlankSourceProfilePlaceholder(store contracts.SourceProfileStore, defaultProfileID string) bool {
	if store.ActiveProfileID != defaultProfileID || len(store.Items) != 1 {
		return false
	}
	item := store.Items[0]
	return item.ID == defaultProfileID && item.Name == "默认输入源" && len(item.Sources) == 0
}

func SourceProfileStoreFromAny(value any) contracts.SourceProfileStore {
	if value == nil {
		return contracts.SourceProfileStore{}
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return contracts.SourceProfileStore{}
	}
	var store contracts.SourceProfileStore
	if err := json.Unmarshal(raw, &store); err != nil {
		return contracts.SourceProfileStore{}
	}
	return store
}

func SourcesFromAny(value any) []contracts.Source {
	if value == nil {
		return []contracts.Source{}
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return []contracts.Source{}
	}
	var sources []contracts.Source
	if err := json.Unmarshal(raw, &sources); err != nil {
		return []contracts.Source{}
	}
	if sources == nil {
		return []contracts.Source{}
	}
	applyMissingSourceEnabled(raw, sources)
	// 输入源的 ip_limit 已从界面移除：任何来源（含旧配置）都按“不限量”处理，
	// 实际候选量由 probe.source_pool_limit（默认 65536）与聚合绝对上限收敛。
	for index := range sources {
		sources[index].IPLimit = -1
	}
	return sources
}

// applyMissingSourceEnabled 兼容没有写 enabled 字段的配置：缺省视为启用，
// 否则带 name/id/ip_limit 的来源会被静默当成“已停用”。
func applyMissingSourceEnabled(raw []byte, sources []contracts.Source) {
	var items []map[string]any
	if err := json.Unmarshal(raw, &items); err != nil {
		return
	}
	for index := range sources {
		if index >= len(items) || items[index] == nil {
			continue
		}
		if enabled, present := items[index]["enabled"]; !present || enabled == nil {
			sources[index].Enabled = true
		}
	}
}

func CloneSources(sources []contracts.Source) []contracts.Source {
	if sources == nil {
		return []contracts.Source{}
	}
	cloned := make([]contracts.Source, len(sources))
	copy(cloned, sources)
	return cloned
}

func CloneSourceProfileStore(store contracts.SourceProfileStore) contracts.SourceProfileStore {
	cloned := store
	cloned.Items = make([]contracts.SourceProfileItem, len(store.Items))
	for index, item := range store.Items {
		cloned.Items[index] = item
		cloned.Items[index].Sources = CloneSources(item.Sources)
	}
	return cloned
}
