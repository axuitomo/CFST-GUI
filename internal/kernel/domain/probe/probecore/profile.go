package probecore

import (
	"strconv"
	"strings"
	"time"
)

type ProfileItemPatch[TValue any] struct {
	ID    string
	Name  string
	Now   string
	Value TValue
}

type CurrentProfileUpdateOptions[TStore any, TItem any, TValue any] struct {
	Store       TStore
	Value       TValue
	ProfileID   string
	Name        string
	Now         string
	DefaultName string

	Items       func(TStore) []TItem
	SetItems    func(*TStore, []TItem)
	ActiveID    func(TStore) string
	SetActiveID func(*TStore, string)
	ItemID      func(TItem) string

	UpdateItem func(*TItem, ProfileItemPatch[TValue])
	NewItem    func(ProfileItemPatch[TValue]) TItem

	NewProfileID    func() string
	ForceNewID      func(string) bool
	DropPlaceholder func(TStore, string) bool
}

type ArchiveProfileItemPatch struct {
	DefaultID   string
	DefaultName string
	Index       int
	Now         string
}

type ArchiveProfileNormalizeOptions[TStore any, TItem any] struct {
	Store         TStore
	SchemaVersion string
	Now           string

	Items        func(TStore) []TItem
	SetItems     func(*TStore, []TItem)
	ActiveID     func(TStore) string
	SetActiveID  func(*TStore, string)
	Schema       func(TStore) string
	SetSchema    func(*TStore, string)
	UpdatedAt    func(TStore) string
	SetUpdatedAt func(*TStore, string)
	ItemID       func(TItem) string

	NewItemID     func(index int) string
	NormalizeItem func(*TItem, ArchiveProfileItemPatch)
}

func NormalizeProfileStoreForArchive[TStore any, TItem any](opts ArchiveProfileNormalizeOptions[TStore, TItem]) TStore {
	store := opts.Store
	now := strings.TrimSpace(opts.Now)
	if now == "" {
		now = time.Now().Format(time.RFC3339)
	}
	if opts.Schema != nil && opts.SetSchema != nil && strings.TrimSpace(opts.Schema(store)) == "" {
		opts.SetSchema(&store, opts.SchemaVersion)
	}
	if opts.UpdatedAt != nil && opts.SetUpdatedAt != nil && strings.TrimSpace(opts.UpdatedAt(store)) == "" {
		opts.SetUpdatedAt(&store, now)
	}

	items := opts.Items(store)
	if items == nil {
		items = []TItem{}
	}
	for index := range items {
		defaultID := ""
		if opts.NewItemID != nil {
			defaultID = strings.TrimSpace(opts.NewItemID(index))
		}
		if defaultID == "" {
			defaultID = "profile-current"
		}
		if opts.NormalizeItem != nil {
			opts.NormalizeItem(&items[index], ArchiveProfileItemPatch{
				DefaultID:   defaultID,
				DefaultName: "配置档案 " + strconv.Itoa(index+1),
				Index:       index,
				Now:         now,
			})
		}
	}
	opts.SetItems(&store, items)

	activeID := ""
	if opts.ActiveID != nil {
		activeID = strings.TrimSpace(opts.ActiveID(store))
	}
	if activeID == "" && len(items) > 0 {
		activeID = strings.TrimSpace(opts.ItemID(items[0]))
		opts.SetActiveID(&store, activeID)
	}
	if len(items) > 0 {
		found := false
		for _, item := range items {
			if strings.TrimSpace(opts.ItemID(item)) == activeID {
				found = true
				break
			}
		}
		if !found {
			opts.SetActiveID(&store, strings.TrimSpace(opts.ItemID(items[0])))
		}
	}
	return store
}

func UpdateCurrentProfileStore[TStore any, TItem any, TValue any](opts CurrentProfileUpdateOptions[TStore, TItem, TValue]) (TStore, string) {
	store := opts.Store
	profileID := strings.TrimSpace(opts.ProfileID)
	if profileID == "" && opts.ActiveID != nil {
		profileID = strings.TrimSpace(opts.ActiveID(store))
	}
	name := strings.TrimSpace(opts.Name)
	defaultName := strings.TrimSpace(opts.DefaultName)
	if defaultName == "" {
		defaultName = "当前配置"
	}

	if opts.DropPlaceholder != nil && opts.DropPlaceholder(store, profileID) {
		opts.SetItems(&store, []TItem{})
	}

	items := opts.Items(store)
	if profileID != "" {
		for index := range items {
			if strings.TrimSpace(opts.ItemID(items[index])) != profileID {
				continue
			}
			opts.UpdateItem(&items[index], ProfileItemPatch[TValue]{
				ID:    profileID,
				Name:  name,
				Now:   opts.Now,
				Value: opts.Value,
			})
			opts.SetItems(&store, items)
			opts.SetActiveID(&store, profileID)
			return store, "updated"
		}
	}

	if profileID == "" || (opts.ForceNewID != nil && opts.ForceNewID(profileID)) {
		profileID = strings.TrimSpace(opts.NewProfileID())
	}
	if profileID == "" {
		profileID = "profile-current"
	}
	if name == "" {
		name = defaultName
	}

	items = append(items, opts.NewItem(ProfileItemPatch[TValue]{
		ID:    profileID,
		Name:  name,
		Now:   opts.Now,
		Value: opts.Value,
	}))
	opts.SetItems(&store, items)
	opts.SetActiveID(&store, profileID)
	return store, "created"
}
