package appcore

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadTaskSnapshotsSortsAndSkipsNonSnapshots(t *testing.T) {
	root := t.TempDir()
	write := func(name string, value any) {
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, name), raw, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("old.json", TaskSnapshot{TaskID: "old", UpdatedAt: "2026-08-01T00:00:00Z", Status: "completed"})
	write("new.json", TaskSnapshot{TaskID: "new", UpdatedAt: "2026-08-02T00:00:00Z", Status: "completed"})
	write("new-results.json", []any{})
	write("broken.json", "not a snapshot")

	snapshots, err := LoadTaskSnapshots(root)
	if err != nil {
		t.Fatalf("LoadTaskSnapshots: %v", err)
	}
	if len(snapshots) != 2 || snapshots[0].TaskID != "new" || snapshots[1].TaskID != "old" {
		t.Fatalf("snapshots = %#v, want new then old", snapshots)
	}
}

func TestLoadTaskSnapshotsLimitReadsLatestValidCandidates(t *testing.T) {
	root := t.TempDir()
	write := func(name, taskID string, modTime time.Time) {
		raw, err := json.Marshal(TaskSnapshot{TaskID: taskID, UpdatedAt: modTime.Format(time.RFC3339)})
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(root, name)
		if err := os.WriteFile(path, raw, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, modTime, modTime); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now()
	write("old.json", "old", now.Add(-2*time.Hour))
	if err := os.WriteFile(filepath.Join(root, "broken.json"), []byte("broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(filepath.Join(root, "broken.json"), now, now); err != nil {
		t.Fatal(err)
	}
	write("new.json", "new", now.Add(-time.Hour))

	snapshots, err := LoadTaskSnapshotsLimit(root, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshots) != 1 || snapshots[0].TaskID != "new" {
		t.Fatalf("snapshots = %#v, want newest valid snapshot", snapshots)
	}
}

func TestSortTaskSnapshotsUsesTaskIDForEqualTimestamps(t *testing.T) {
	snapshots := []TaskSnapshot{
		{TaskID: "a", UpdatedAt: "2026-08-01T00:00:00Z"},
		{TaskID: "b", UpdatedAt: "2026-08-01T00:00:00Z"},
	}
	SortTaskSnapshotsLatestFirst(snapshots)
	if snapshots[0].TaskID != "b" {
		t.Fatalf("first task = %q, want b", snapshots[0].TaskID)
	}
}

func TestLoadTaskSnapshotsSkipsSymlinks(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.json")
	raw, err := json.Marshal(TaskSnapshot{TaskID: "outside", UpdatedAt: "2026-08-01T00:00:00Z"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outside, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "linked.json")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	snapshots, err := LoadTaskSnapshots(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshots) != 0 {
		t.Fatalf("symlink snapshot should be skipped: %#v", snapshots)
	}
}
