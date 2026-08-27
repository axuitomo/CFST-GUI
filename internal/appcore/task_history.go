package appcore

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// LoadTaskSnapshots reads persisted task metadata without interpreting runtime
// ownership. Platform adapters must still call their loadTaskSnapshot method
// so a detached runtime is marked failed before it is exposed to a client.
func LoadTaskSnapshots(root string) ([]TaskSnapshot, error) {
	return LoadTaskSnapshotsLimit(root, 0)
}

func LoadTaskSnapshotsLimit(root string, limit int) ([]TaskSnapshot, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, nil
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	type snapshotEntry struct {
		entry   os.DirEntry
		modTime time.Time
	}
	candidates := make([]snapshotEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || !strings.HasSuffix(entry.Name(), ".json") || strings.HasSuffix(entry.Name(), "-results.json") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, err
		}
		candidates = append(candidates, snapshotEntry{entry: entry, modTime: info.ModTime()})
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if !candidates[i].modTime.Equal(candidates[j].modTime) {
			return candidates[i].modTime.After(candidates[j].modTime)
		}
		return candidates[i].entry.Name() > candidates[j].entry.Name()
	})
	snapshots := make([]TaskSnapshot, 0, len(candidates))
	for _, candidate := range candidates {
		entry := candidate.entry
		raw, err := os.ReadFile(filepath.Join(root, entry.Name()))
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, err
		}
		var snapshot TaskSnapshot
		if err := json.Unmarshal(raw, &snapshot); err != nil || strings.TrimSpace(snapshot.TaskID) == "" {
			// A single damaged historical file must not prevent startup recovery.
			continue
		}
		snapshots = append(snapshots, snapshot)
		if limit > 0 && len(snapshots) >= limit {
			break
		}
	}
	SortTaskSnapshotsLatestFirst(snapshots)
	return snapshots, nil
}

func SortTaskSnapshotsLatestFirst(snapshots []TaskSnapshot) {
	sort.SliceStable(snapshots, func(i, j int) bool {
		left := taskSnapshotTime(snapshots[i])
		right := taskSnapshotTime(snapshots[j])
		if !left.Equal(right) {
			return left.After(right)
		}
		return snapshots[i].TaskID > snapshots[j].TaskID
	})
}

func taskSnapshotTime(snapshot TaskSnapshot) time.Time {
	for _, value := range []string{snapshot.UpdatedAt, snapshot.CompletedAt, snapshot.StartedAt} {
		if parsed, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(value)); err == nil {
			return parsed
		}
	}
	return time.Time{}
}
