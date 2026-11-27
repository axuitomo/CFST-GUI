package appcore

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// HotStore is the stable hot-tier contract. The current build uses the JSON
// fallback because modernc.org/sqlite could not be fetched in the build
// environment; a SQLite implementation can replace it without changing callers.
type HotStore interface {
	UpsertTaskSnapshot(TaskSnapshot) error
	GetTaskSnapshot(string) (*TaskSnapshot, error)
	ListTaskSnapshots(string, bool) ([]TaskSnapshot, error)
	MarkArchived(string) error
	DeleteArchivedBefore(time.Time, int) (int, error)
	ListRecentCompleted(int) ([]TaskSnapshot, error)
	GetCachedSnapshot(string) (*TaskSnapshot, bool)
	InvalidateCache(string)
	Close() error
	Checkpoint() error
	Vacuum() error
}

type FileHotStore struct {
	mu    sync.RWMutex
	root  string
	cache map[string]TaskSnapshot
}

func NewFileHotStore(root string) *FileHotStore {
	return &FileHotStore{root: root, cache: map[string]TaskSnapshot{}}
}

func (store *FileHotStore) snapshotPath(id string) string {
	return filepath.Join(store.root, TaskStorageID(id)+".json")
}
func (store *FileHotStore) UpsertTaskSnapshot(snapshot TaskSnapshot) error {
	if strings.TrimSpace(snapshot.TaskID) == "" {
		return errors.New("task id is required")
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if err := os.MkdirAll(store.root, 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return err
	}
	if err := WriteFileAtomic(store.snapshotPath(snapshot.TaskID), raw, 0o600); err != nil {
		return err
	}
	store.cache[snapshot.TaskID] = snapshot
	return nil
}
func (store *FileHotStore) GetTaskSnapshot(id string) (*TaskSnapshot, error) {
	store.mu.RLock()
	snapshot, ok := store.cache[id]
	store.mu.RUnlock()
	if ok {
		return &snapshot, nil
	}
	raw, err := os.ReadFile(store.snapshotPath(id))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	snapshot = TaskSnapshot{}
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		return nil, err
	}
	store.mu.Lock()
	store.cache[id] = snapshot
	store.mu.Unlock()
	return &snapshot, nil
}
func (store *FileHotStore) ListTaskSnapshots(status string, includeArchived bool) ([]TaskSnapshot, error) {
	entries, err := os.ReadDir(store.root)
	if err != nil {
		if os.IsNotExist(err) {
			return []TaskSnapshot{}, nil
		}
		return nil, err
	}
	result := make([]TaskSnapshot, 0)
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		raw, readErr := os.ReadFile(filepath.Join(store.root, entry.Name()))
		if readErr != nil {
			return nil, readErr
		}
		var snapshot TaskSnapshot
		if json.Unmarshal(raw, &snapshot) != nil {
			continue
		}
		if status != "" && snapshot.Status != status {
			continue
		}
		if !includeArchived && snapshot.Archived {
			continue
		}
		result = append(result, snapshot)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].UpdatedAt > result[j].UpdatedAt })
	return result, nil
}
func (store *FileHotStore) MarkArchived(id string) error {
	snapshot, err := store.GetTaskSnapshot(id)
	if err != nil || snapshot == nil {
		return err
	}
	snapshot.Archived = true
	return store.UpsertTaskSnapshot(*snapshot)
}
func (store *FileHotStore) DeleteArchivedBefore(before time.Time, limit int) (int, error) {
	list, err := store.ListTaskSnapshots("", true)
	if err != nil {
		return 0, err
	}
	deleted := 0
	for _, snapshot := range list {
		if limit > 0 && deleted >= limit || !snapshot.Archived {
			continue
		}
		timestamp, parseErr := time.Parse(time.RFC3339, snapshot.UpdatedAt)
		if parseErr != nil || timestamp.After(before) {
			continue
		}
		if err := os.Remove(store.snapshotPath(snapshot.TaskID)); err != nil && !os.IsNotExist(err) {
			return deleted, err
		}
		store.InvalidateCache(snapshot.TaskID)
		deleted++
	}
	return deleted, nil
}
func (store *FileHotStore) ListRecentCompleted(limit int) ([]TaskSnapshot, error) {
	list, err := store.ListTaskSnapshots("completed", true)
	if err != nil {
		return nil, err
	}
	if limit > 0 && len(list) > limit {
		list = list[:limit]
	}
	return list, nil
}
func (store *FileHotStore) GetCachedSnapshot(id string) (*TaskSnapshot, bool) {
	store.mu.RLock()
	defer store.mu.RUnlock()
	snapshot, ok := store.cache[id]
	if !ok {
		return nil, false
	}
	return &snapshot, true
}
func (store *FileHotStore) InvalidateCache(id string) {
	store.mu.Lock()
	delete(store.cache, id)
	store.mu.Unlock()
}
func (store *FileHotStore) Close() error      { return nil }
func (store *FileHotStore) Checkpoint() error { return nil }
func (store *FileHotStore) Vacuum() error     { return nil }
