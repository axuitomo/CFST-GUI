package appcore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestTaskStoreKeepsUnsafeIDsInsideRoot(t *testing.T) {
	root := t.TempDir()
	store := NewTaskStore(root, time.Now)
	path := store.SnapshotPath(`..\outside/task`)
	if filepath.Dir(path) != root || !strings.HasPrefix(filepath.Base(path), hashedTaskStoragePrefix) {
		t.Fatalf("unsafe task path escaped root: %q", path)
	}
}

func TestTaskStoreNormalizesDetachedActiveSnapshot(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, time.August, 8, 9, 0, 0, 0, time.UTC)
	store := NewTaskStore(root, func() time.Time { return now })
	snapshot := BuildAcceptedTaskSnapshot("task-1", now)
	snapshot.Status = "running"
	if err := store.WriteSnapshot(snapshot, TaskAttachment{CurrentTaskID: "task-1"}); err != nil {
		t.Fatal(err)
	}
	store = NewTaskStore(root, func() time.Time { return now })
	loaded, ok, err := store.LoadSnapshot("task-1", TaskAttachment{})
	if err != nil || !ok {
		t.Fatalf("LoadSnapshot() = %#v, %v, %v", loaded, ok, err)
	}
	if loaded.Status != "failed" || loaded.SessionState != "persisted_only" || loaded.RuntimeAttached {
		t.Fatalf("detached snapshot = %#v", loaded)
	}
}

func TestTaskStoreRoundTripsResults(t *testing.T) {
	store := NewTaskStore(t.TempDir(), time.Now)
	rows := []ProbeResultRow{{Address: "1.1.1.1", StageStatus: "completed"}}
	if err := store.WriteResults("task-1", rows); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.LoadResults("task-1")
	if err != nil || len(loaded) != 1 || loaded[0].Address != rows[0].Address {
		t.Fatalf("LoadResults() = %#v, %v", loaded, err)
	}
	page, err := store.QueryResults("task-1", TaskResultsRequest{Limit: 1})
	if err != nil || !page.Found || page.SourceCount != 1 || page.Count != 1 || page.Results[0].Address != rows[0].Address {
		t.Fatalf("QueryResults() = %#v, %v", page, err)
	}
}

func TestTaskStoreQueryResultsPagesWithoutMaterializingAllRowsInResponse(t *testing.T) {
	store := NewTaskStore(t.TempDir(), time.Now)
	fast := 20.0
	slow := 5.0
	rows := []ProbeResultRow{
		{Address: "2.2.2.2", DownloadMbps: &slow, ExportStatus: "exported", StageStatus: "completed"},
		{Address: "1.1.1.1", DownloadMbps: &fast, ExportStatus: "exported", StageStatus: "completed"},
		{Address: "3.3.3.3", ExportStatus: "pending", StageStatus: "pending"},
	}
	if err := store.WriteResults("paged-task", rows); err != nil {
		t.Fatal(err)
	}
	page, err := store.QueryResults("paged-task", TaskResultsRequest{
		SortBy: "download", Order: "desc", Filter: "exported", Limit: 1, Offset: 0,
	})
	if err != nil || !page.Found || page.SourceCount != 3 || page.TotalCount != 2 || page.Count != 1 || page.Results[0].Address != "1.1.1.1" {
		t.Fatalf("QueryResults() = %#v, %v", page, err)
	}
}

func TestTaskStoreQueryResultsRejectsOversizedFile(t *testing.T) {
	root := t.TempDir()
	store := NewTaskStore(root, time.Now)
	path := store.ResultsPath("oversized")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("[]"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := queryTaskResultsFromJSONLimited(path, TaskResultsRequest{}, 1)
	if err == nil || !strings.Contains(err.Error(), "上限") {
		t.Fatalf("err = %v, want oversized results error", err)
	}
}
