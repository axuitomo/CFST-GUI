package appcore

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/probecore"
)

func newTaskServiceForTest(t *testing.T) *Service {
	t.Helper()
	return NewService(ServiceOptions{Storage: StorageLayout{Root: filepath.Join(t.TempDir(), "state")}})
}

func TestServiceProbeControlsPersistAndPublishSharedEvents(t *testing.T) {
	service := newTaskServiceForTest(t)
	events := make(chan ProbeEvent, 4)
	service.SetEventSink(EventSinkFunc(func(_ context.Context, event ProbeEvent) { events <- event }))
	ctx, ok, _ := service.runtime.Start("control-task")
	if !ok || ctx == nil {
		t.Fatal("runtime did not start")
	}
	if result := service.PauseProbe(ProbeControlRequest{TaskID: "control-task"}); !result.OK || result.Code != "PROBE_PAUSE_REQUESTED" {
		t.Fatalf("PauseProbe = %#v", result)
	}
	if result := service.ResumeProbe(ProbeControlRequest{TaskID: "control-task"}); !result.OK || result.Code != "PROBE_RESUME_REQUESTED" {
		t.Fatalf("ResumeProbe = %#v", result)
	}
	if result := service.CancelProbe(ProbeControlRequest{TaskID: "control-task"}); !result.OK || result.Code != "PROBE_CANCEL_REQUESTED" {
		t.Fatalf("CancelProbe = %#v", result)
	}
	for range 3 {
		select {
		case event := <-events:
			if event.TaskID != "control-task" || event.Seq == 0 {
				t.Fatalf("event = %#v", event)
			}
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for shared control event")
		}
	}
	service.runtime.Clear("control-task")
	snapshot, ok, err := service.taskStoreSnapshot("control-task")
	if err != nil || !ok || snapshot.TaskID != "control-task" {
		t.Fatalf("snapshot = %#v, %v, %v", snapshot, ok, err)
	}
}

func TestServiceRunProbeOwnsRuntimeAndStartProbeIsAsync(t *testing.T) {
	service := newTaskServiceForTest(t)
	result, err := service.runProbeWithRunner("sync-task", func(ctx context.Context) (any, error) {
		if service.runtime.State().CurrentTaskID != "sync-task" {
			t.Fatalf("runtime task = %#v", service.runtime.State())
		}
		return "done", nil
	})
	if err != nil || result != "done" {
		t.Fatalf("RunProbe = %#v, %v", result, err)
	}
	accepted := service.startProbeWithRunner("async-task", func(context.Context) (any, error) { return nil, nil })
	if !accepted.OK || accepted.Code != "PROBE_ACCEPTED" {
		t.Fatalf("StartProbe = %#v", accepted)
	}
	if !service.runtime.WaitStopped("async-task", time.Second) {
		t.Fatal("async task did not stop")
	}
}

func TestServiceLifecycleResetAllowsTaskIDReuseAfterTerminalEvent(t *testing.T) {
	service := newTaskServiceForTest(t)
	events := make([]ProbeEvent, 0, 2)
	service.SetEventSink(EventSinkFunc(func(_ context.Context, event ProbeEvent) { events = append(events, event) }))
	if err := service.PublishProbeEvent(context.Background(), ProbeEvent{Event: "probe.cancelled", TaskID: "reused-task"}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.runProbeWithRunner("reused-task", func(context.Context) (any, error) {
		return nil, service.PublishProbeEvent(context.Background(), ProbeEvent{Event: "probe.progress", TaskID: "reused-task"})
	}); err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[1].Event != "probe.progress" {
		t.Fatalf("events after task id reuse = %#v", events)
	}
}

func TestServiceCancelProbeWaitsForPausedTaskAndReportsPending(t *testing.T) {
	service := NewService(ServiceOptions{
		CancelTimeout: 10 * time.Millisecond,
		Storage:       StorageLayout{Root: filepath.Join(t.TempDir(), "state")},
	})
	if _, ok, _ := service.runtime.Start("paused-task"); !ok {
		t.Fatal("runtime did not start")
	}
	service.runtime.Pause("paused-task")
	result := service.CancelProbe(ProbeControlRequest{TaskID: "paused-task"})
	if result.OK || result.Code != "PROBE_CANCEL_PENDING" {
		t.Fatalf("CancelProbe = %#v, want pending", result)
	}
	service.runtime.Clear("paused-task")
}

func TestServiceTaskQueriesUseSharedStore(t *testing.T) {
	service := newTaskServiceForTest(t)
	if err := service.writeTaskSnapshot(TaskSnapshot{TaskID: "history-task", Status: "completed"}); err != nil {
		t.Fatal(err)
	}
	got := service.GetTask(TaskQueryRequest{TaskID: "history-task"})
	if !got.OK || got.Code != "TASK_SNAPSHOT" {
		t.Fatalf("GetTask = %#v", got)
	}
	list := service.ListTasks(TaskQueryRequest{Limit: 5})
	if !list.OK || list.Code != "TASK_SNAPSHOT_LIST" {
		t.Fatalf("ListTasks = %#v", list)
	}
}

func TestServiceQueryTaskResultsOwnsFilteringSortingAndPagination(t *testing.T) {
	service := newTaskServiceForTest(t)
	fast := 20.0
	slow := 5.0
	rows := []ProbeResultRow{
		{Address: "2001:db8::1", DownloadMbps: &slow, ExportStatus: "exported", StageStatus: "completed"},
		{Address: "1.1.1.1", DownloadMbps: &fast, ExportStatus: "exported", StageStatus: "completed"},
		{Address: "2.2.2.2", ExportStatus: "pending", StageStatus: "pending"},
	}
	if err := service.taskStore.WriteResults("results-task", rows); err != nil {
		t.Fatal(err)
	}
	page, err := service.QueryTaskResults(TaskResultsRequest{
		TaskID: "results-task", SortBy: "download", Order: "desc", Filter: "exported", IPFilter: "ipv4", Limit: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !page.Found || page.SourceCount != 3 || page.TotalCount != 1 || page.Count != 1 || page.Results[0].Address != "1.1.1.1" {
		t.Fatalf("QueryTaskResults = %#v", page)
	}
}

func TestServiceQueryTaskResultsDistinguishesMissingAndEmptyFiles(t *testing.T) {
	service := newTaskServiceForTest(t)
	missing, err := service.QueryTaskResults(TaskResultsRequest{TaskID: "missing"})
	if err != nil || missing.Found {
		t.Fatalf("missing QueryTaskResults = %#v, %v", missing, err)
	}
	if err := service.taskStore.WriteResults("empty", []ProbeResultRow{}); err != nil {
		t.Fatal(err)
	}
	empty, err := service.QueryTaskResults(TaskResultsRequest{TaskID: "empty"})
	if err != nil || !empty.Found || empty.Count != 0 || empty.Results == nil {
		t.Fatalf("empty QueryTaskResults = %#v, %v", empty, err)
	}
}

func TestServiceQueryTaskResultsFallsBackFromMissingPathToSnapshotCSV(t *testing.T) {
	service := newTaskServiceForTest(t)
	csvPath := filepath.Join(t.TempDir(), "snapshot-result.csv")
	if err := os.WriteFile(csvPath, []byte("address,tcp_latency_ms\n1.1.1.1,12.5\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := service.WriteTaskSnapshot(TaskSnapshot{
		TaskID: "legacy-csv-task",
		Status: "completed",
		ExportRecord: &ExportRecordSnapshot{
			SourcePath: csvPath,
		},
	}); err != nil {
		t.Fatal(err)
	}
	page, err := service.QueryTaskResults(TaskResultsRequest{
		TaskID: "legacy-csv-task",
		Path:   filepath.Join(t.TempDir(), "missing.csv"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if page.SourceKind != "csv" || page.SourcePath != csvPath || page.Count != 1 || page.Results[0].Address != "1.1.1.1" {
		t.Fatalf("QueryTaskResults = %#v", page)
	}
}

func TestServicePersistCompletedProbeWritesSharedResultsAndSnapshot(t *testing.T) {
	service := newTaskServiceForTest(t)
	result := ProbeRunResult{
		OutputFile: "exports/result.csv",
		Results:    []probecore.ProbeRow{{IP: "1.1.1.1", Colo: "N/A", DelayMS: 12, DownloadSpeedMB: 8}},
		StartedAt:  "2026-08-08T10:00:00+08:00",
		Summary:    probecore.ProbeSummary{Passed: 1},
		TaskContext: probecore.TaskContext{
			GlobalTCPPort: 443,
		},
	}
	if err := service.PersistCompletedProbe("completed-task", result); err != nil {
		t.Fatal(err)
	}
	page, err := service.QueryTaskResults(TaskResultsRequest{TaskID: "completed-task"})
	if err != nil || page.Count != 1 || page.Results[0].Address != "1.1.1.1" || page.Results[0].Colo != nil {
		t.Fatalf("results = %#v, %v", page, err)
	}
	snapshot, ok, err := service.taskStoreSnapshot("completed-task")
	if err != nil || !ok || snapshot.Status != "completed" || snapshot.RuntimeAttached || snapshot.TaskContext["global_tcp_port"] != float64(443) {
		t.Fatalf("snapshot = %#v, %v, %v", snapshot, ok, err)
	}
}
