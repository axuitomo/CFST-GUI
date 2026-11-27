package appcore

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTaskResultsKeepsPersistedResultsAuthoritative(t *testing.T) {
	for _, tc := range []struct {
		name      string
		empty     bool
		request   TaskResultsRequest
		wantCount int
		wantTotal int
	}{
		{name: "empty", empty: true},
		{name: "populated", wantCount: 1, wantTotal: 1},
		{name: "filtered_empty", request: TaskResultsRequest{IPFilter: "ipv6"}},
		{name: "past_last_page", request: TaskResultsRequest{Limit: 1, Offset: 1}, wantTotal: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service := newTaskServiceForTest(t)
			csvPath := filepath.Join(t.TempDir(), "prior-result.csv")
			if err := os.WriteFile(csvPath, []byte("address,tcp_latency_ms\n192.0.2.99,5\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := service.WriteTaskSnapshot(TaskSnapshot{TaskID: "bound-task", Status: "completed", ExportRecord: &ExportRecordSnapshot{SourcePath: csvPath}}); err != nil {
				t.Fatal(err)
			}
			rows := []ProbeResultRow{}
			wantKind := "empty_persisted"
			if !tc.empty {
				rows = append(rows, ProbeResultRow{Address: "1.1.1.1"})
				wantKind = "persisted"
			}
			if err := service.WriteTaskResults("bound-task", rows); err != nil {
				t.Fatal(err)
			}
			request := tc.request
			request.TaskID, request.Path = " bound-task ", csvPath
			page, err := service.QueryTaskResults(request)
			if err != nil {
				t.Fatal(err)
			}
			if !page.Found || page.SourceKind != wantKind || page.Count != tc.wantCount || page.TotalCount != tc.wantTotal || page.Results == nil {
				t.Fatalf("QueryTaskResults = %#v", page)
			}
			if page.Count > 0 && page.Results[0].Address != "1.1.1.1" {
				t.Fatalf("wrong task rows: %#v", page.Results)
			}
		})
	}
}

func TestTaskResultsOnlyUsesBoundCSVForTask(t *testing.T) {
	service := newTaskServiceForTest(t)
	root := t.TempDir()
	writeCSV := func(name, address string) string {
		t.Helper()
		path := filepath.Join(root, name)
		if err := os.WriteFile(path, []byte("address,tcp_latency_ms\n"+address+",5\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	bound := writeCSV("bound.csv", "1.1.1.1")
	unrelated := writeCSV("unrelated.csv", "192.0.2.99")
	for _, taskID := range []string{"unknown-task", "without-export"} {
		if taskID == "without-export" {
			if err := service.WriteTaskSnapshot(TaskSnapshot{TaskID: taskID, Status: "completed"}); err != nil {
				t.Fatal(err)
			}
		}
		page, err := service.QueryTaskResults(TaskResultsRequest{TaskID: taskID, Path: unrelated})
		if err != nil || page.Found || page.Count != 0 {
			t.Fatalf("unrelated path used: %#v, %v", page, err)
		}
	}
	if err := service.WriteTaskSnapshot(TaskSnapshot{TaskID: "android-task", Status: "completed", ExportRecord: &ExportRecordSnapshot{SourcePath: bound, TargetDir: "content://exports/tree/results", FileName: "result.csv"}}); err != nil {
		t.Fatal(err)
	}
	page, err := service.QueryTaskResults(TaskResultsRequest{TaskID: "android-task", Path: unrelated})
	if err != nil || !page.Found || page.SourcePath != bound || page.Count != 1 || page.Results[0].Address != "1.1.1.1" {
		t.Fatalf("bound Android CSV query = %#v, %v", page, err)
	}
	for _, record := range []ExportRecordSnapshot{
		{SourcePath: bound, TargetDir: root, FileName: "unrelated.csv"},
		{SourcePath: filepath.Join(root, "missing.csv"), TargetDir: root, FileName: "bound.csv"},
		{SourcePath: "content://exports/document/result", TargetDir: root, FileName: "bound.csv"},
	} {
		if err := service.WriteTaskSnapshot(TaskSnapshot{TaskID: "legacy-task", Status: "completed", ExportRecord: &record}); err != nil {
			t.Fatal(err)
		}
		page, err := service.QueryTaskResults(TaskResultsRequest{TaskID: "legacy-task", Path: unrelated})
		if err != nil || page.SourcePath != bound || page.Count != 1 || page.Results[0].Address != "1.1.1.1" {
			t.Fatalf("bound CSV priority for %#v = %#v, %v", record, page, err)
		}
	}
	if err := service.WriteTaskResults("android-task", []ProbeResultRow{}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(service.taskStore.ResultsPath("android-task"), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := service.QueryTaskResults(TaskResultsRequest{TaskID: "android-task", Path: unrelated}); err == nil {
		t.Fatal("corrupt persisted results silently fell back to CSV")
	}
}

func TestTaskResultsPreservesExplicitFileMode(t *testing.T) {
	service := newTaskServiceForTest(t)
	root := service.StorageLayout().ExportsRoot()
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "explicit.csv")
	if err := os.WriteFile(path, []byte("address,tcp_latency_ms\n1.1.1.1,5\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, taskID := range []string{"", " ", "result-file", " result-file "} {
		for _, requestPath := range []string{path, filepath.Base(path)} {
			page, err := service.QueryTaskResults(TaskResultsRequest{TaskID: taskID, Path: requestPath})
			if err != nil || !page.Found || page.SourceKind != "csv" || page.Count != 1 || page.Results[0].Address != "1.1.1.1" {
				t.Fatalf("explicit file query = %#v, %v", page, err)
			}
		}
	}
}
