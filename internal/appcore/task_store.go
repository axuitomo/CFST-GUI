package appcore

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const hashedTaskStoragePrefix = "task-hash-"

type TaskAttachment struct {
	CurrentTaskID  string
	PauseRequested bool
	PausedTaskID   string
}

type TaskStore struct {
	mu    sync.Mutex
	root  string
	clock Clock
	cache map[string]TaskSnapshot
}

func NewTaskStore(root string, clock Clock) *TaskStore {
	if clock == nil {
		clock = time.Now
	}
	return &TaskStore{root: root, clock: clock, cache: map[string]TaskSnapshot{}}
}

func (store *TaskStore) SetRoot(root string) {
	store.mu.Lock()
	store.root = root
	store.cache = map[string]TaskSnapshot{}
	store.mu.Unlock()
}

func (store *TaskStore) Root() string {
	store.mu.Lock()
	defer store.mu.Unlock()
	return store.root
}

func TaskStorageID(taskID string) string {
	taskID = strings.TrimSpace(taskID)
	if IsSafeTaskStorageID(taskID) {
		return taskID
	}
	sum := sha256.Sum256([]byte(taskID))
	return hashedTaskStoragePrefix + hex.EncodeToString(sum[:])
}

func IsSafeTaskStorageID(taskID string) bool {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" || taskID == "." || taskID == ".." || strings.HasPrefix(taskID, ".") || strings.HasPrefix(taskID, hashedTaskStoragePrefix) {
		return false
	}
	for index := 0; index < len(taskID); index++ {
		char := taskID[index]
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || char == '-' || char == '_' || char == '.' {
			continue
		}
		return false
	}
	return true
}

func (store *TaskStore) SnapshotPath(taskID string) string {
	return filepath.Join(store.Root(), TaskStorageID(taskID)+".json")
}

func (store *TaskStore) ResultsPath(taskID string) string {
	return filepath.Join(store.Root(), TaskStorageID(taskID)+"-results.json")
}

func (store *TaskStore) WriteSnapshot(snapshot TaskSnapshot, attachment TaskAttachment) error {
	taskID := strings.TrimSpace(snapshot.TaskID)
	if taskID == "" {
		return nil
	}
	snapshot.TaskID = taskID
	snapshot.UpdatedAt = store.clock().Format(time.RFC3339)
	normalizeTaskRuntimeAttachment(&snapshot, attachment)

	if err := os.MkdirAll(store.Root(), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return err
	}
	if err := WriteFileAtomic(store.SnapshotPath(taskID), raw, 0o600); err != nil {
		return err
	}
	store.cacheSnapshot(snapshot)
	return nil
}

func (store *TaskStore) WriteResults(taskID string, rows []ProbeResultRow) error {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil
	}
	raw, err := json.MarshalIndent(rows, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(store.Root(), 0o755); err != nil {
		return err
	}
	return WriteFileAtomic(store.ResultsPath(taskID), raw, 0o600)
}

func (store *TaskStore) LoadSnapshot(taskID string, attachment TaskAttachment) (TaskSnapshot, bool, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return TaskSnapshot{}, false, nil
	}
	store.mu.Lock()
	snapshot, ok := store.cache[taskID]
	store.mu.Unlock()
	if !ok {
		raw, err := os.ReadFile(store.SnapshotPath(taskID))
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return TaskSnapshot{}, false, nil
			}
			return TaskSnapshot{}, false, err
		}
		if err := json.Unmarshal(raw, &snapshot); err != nil {
			return TaskSnapshot{}, false, err
		}
	}

	changed := normalizeLoadedTaskSnapshot(&snapshot, attachment)
	store.cacheSnapshot(snapshot)
	if changed {
		if err := store.WriteSnapshot(snapshot, attachment); err != nil {
			return TaskSnapshot{}, false, err
		}
	}
	return snapshot, true, nil
}

func (store *TaskStore) LoadResults(taskID string) ([]ProbeResultRow, error) {
	page, err := store.QueryResults(taskID, TaskResultsRequest{})
	if err != nil {
		return nil, err
	}
	if !page.Found {
		return nil, nil
	}
	if page.Results == nil {
		return []ProbeResultRow{}, nil
	}
	return page.Results, nil
}

func (store *TaskStore) QueryResults(taskID string, request TaskResultsRequest) (TaskResultsPage, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return TaskResultsPage{}, nil
	}
	return queryTaskResultsFromJSONLimited(store.ResultsPath(taskID), request, MaxTaskResultsBytes)
}

func queryTaskResultsFromJSONLimited(path string, request TaskResultsRequest, limit int64) (TaskResultsPage, error) {
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return TaskResultsPage{}, nil
		}
		return TaskResultsPage{}, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return TaskResultsPage{}, err
	}
	if limit > 0 && info.Size() > limit {
		return TaskResultsPage{}, fmt.Errorf("任务结果超过 %d 字节上限", limit)
	}
	reader := io.Reader(file)
	if limit > 0 {
		reader = io.LimitReader(file, limit+1)
	}
	decoder := json.NewDecoder(reader)
	token, err := decoder.Token()
	if err != nil {
		return TaskResultsPage{}, err
	}
	delim, ok := token.(json.Delim)
	if !ok || delim != '[' {
		return TaskResultsPage{}, errors.New("任务结果不是 JSON 数组")
	}

	rows := make([]ProbeResultRow, 0)
	for decoder.More() {
		var row ProbeResultRow
		if err := decoder.Decode(&row); err != nil {
			return TaskResultsPage{}, err
		}
		rows = append(rows, row)
	}
	token, err = decoder.Token()
	if err != nil {
		return TaskResultsPage{}, err
	}
	if delim, ok = token.(json.Delim); !ok || delim != ']' {
		return TaskResultsPage{}, errors.New("任务结果 JSON 数组未正确结束")
	}
	if _, err := decoder.Token(); err != io.EOF {
		if err != nil {
			return TaskResultsPage{}, err
		}
		return TaskResultsPage{}, errors.New("任务结果 JSON 存在多余内容")
	}

	page := PageTaskResults(rows, request)
	page.Found = true
	return page, nil
}

func (store *TaskStore) ListSnapshots(limit int) ([]TaskSnapshot, error) {
	return LoadTaskSnapshotsLimit(store.Root(), limit)
}

func (store *TaskStore) cacheSnapshot(snapshot TaskSnapshot) {
	store.mu.Lock()
	defer store.mu.Unlock()
	if ShouldCacheTaskSnapshot(snapshot.Status) {
		store.cache[snapshot.TaskID] = snapshot
	} else {
		delete(store.cache, snapshot.TaskID)
	}
}

func (store *TaskStore) CacheCount() int {
	store.mu.Lock()
	defer store.mu.Unlock()
	return len(store.cache)
}

func (store *TaskStore) TrimTerminalCache() {
	store.mu.Lock()
	defer store.mu.Unlock()
	for taskID, snapshot := range store.cache {
		if !ShouldCacheTaskSnapshot(snapshot.Status) {
			delete(store.cache, taskID)
		}
	}
}

func normalizeTaskRuntimeAttachment(snapshot *TaskSnapshot, attachment TaskAttachment) {
	if isTerminalTaskStatus(snapshot.Status) {
		snapshot.RuntimeAttached = false
		snapshot.ResumeCapable = false
		snapshot.SessionState = "persisted_only"
		return
	}
	snapshot.RuntimeAttached = attachment.CurrentTaskID == snapshot.TaskID
	snapshot.ResumeCapable = snapshot.RuntimeAttached && attachment.PauseRequested && attachment.PausedTaskID == snapshot.TaskID
	if snapshot.ResumeCapable {
		snapshot.SessionState = "paused_runtime"
	} else if snapshot.RuntimeAttached {
		snapshot.SessionState = "active_runtime"
	} else if strings.TrimSpace(snapshot.SessionState) == "" {
		snapshot.SessionState = "persisted_only"
	}
}

func normalizeLoadedTaskSnapshot(snapshot *TaskSnapshot, attachment TaskAttachment) bool {
	if isActiveTaskStatus(snapshot.Status) {
		if attachment.CurrentTaskID == snapshot.TaskID {
			normalizeTaskRuntimeAttachment(snapshot, attachment)
		} else {
			snapshot.RuntimeAttached = false
			snapshot.ResumeCapable = false
			snapshot.SessionState = "persisted_only"
			snapshot.Status = "failed"
			if strings.TrimSpace(snapshot.CurrentStage) == "" {
				snapshot.CurrentStage = "recovery_required"
			}
			if snapshot.FailureSummary == nil {
				snapshot.FailureSummary = map[string]any{}
			}
			if _, exists := snapshot.FailureSummary["recovery_status"]; !exists {
				snapshot.FailureSummary["recovery_status"] = "runtime_detached"
			}
		}
		return true
	}
	if isTerminalTaskStatus(snapshot.Status) && (snapshot.RuntimeAttached || snapshot.ResumeCapable || strings.TrimSpace(snapshot.SessionState) != "persisted_only") {
		normalizeTaskRuntimeAttachment(snapshot, attachment)
		return true
	}
	return false
}

func isActiveTaskStatus(status string) bool {
	switch status {
	case "running", "preparing", "cooling", "partial":
		return true
	default:
		return false
	}
}

func isTerminalTaskStatus(status string) bool {
	switch status {
	case "cancelled", "completed", "failed", "no_results":
		return true
	default:
		return false
	}
}
