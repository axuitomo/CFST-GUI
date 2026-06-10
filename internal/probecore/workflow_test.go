package probecore

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/utils"
)

func TestRunProbeWorkflowGroupsPortsAndExportsCombinedResults(t *testing.T) {
	startedAt := time.Date(2026, 5, 14, 10, 0, 0, 0, time.UTC)
	var groupRequests []WorkflowGroupRequest
	var exported []utils.CloudflareIPData
	result, err := RunProbeWorkflow(WorkflowRunRequest{
		Config: WorkflowConfig{
			DownloadSpeedMetric: utils.DownloadSpeedMetricAverage,
			PrintNum:            0,
			TCPPort:             443,
		},
		Groups: []PortGroup{
			{Port: 443, IPs: []string{"8.8.8.8"}},
			{Port: 2053, IPs: []string{"1.1.1.1"}},
		},
		Source: WorkflowSource{
			Summary: SourceSummary{
				Valid:      []string{"1.1.1.1", "8.8.8.8"},
				ValidCount: 2,
			},
			Text:     "1.1.1.1\n8.8.8.8",
			Warnings: []string{"source warning"},
		},
		TaskContext: TaskContext{
			CurrentTestPort: 0,
			GlobalTCPPort:   443,
			PortPolicy:      PortPolicySourceOverrideGlobal,
		},
		TaskID: "task-1",
	}, WorkflowAdapter{
		BeginMultiGroup: func(WorkflowRunRequest) (WorkflowLifecycle, error) {
			return WorkflowLifecycle{
				DebugLogPath: "/tmp/probe.log",
				StartedAt:    startedAt,
				Warnings:     []string{"debug warning"},
			}, nil
		},
		Export: func(req WorkflowExportRequest) (WorkflowExportResult, error) {
			exported = append([]utils.CloudflareIPData(nil), req.RawResults...)
			return WorkflowExportResult{OutputFile: "/tmp/result.csv"}, nil
		},
		Now: func() time.Time {
			return startedAt.Add(2 * time.Second)
		},
		RunGroup: func(req WorkflowGroupRequest) (WorkflowGroupResult, error) {
			groupRequests = append(groupRequests, req)
			ip := strings.TrimSpace(req.SourceText)
			return WorkflowGroupResult{
				RawResults: []utils.CloudflareIPData{
					probeCoreTestData(ip, time.Duration(req.Group.Port)*time.Microsecond, 10),
				},
				Summary: ProbeSummary{Total: 1, Passed: 1},
			}, nil
		},
	})
	if err != nil {
		t.Fatalf("RunProbeWorkflow() error = %v", err)
	}
	if len(groupRequests) != 2 {
		t.Fatalf("group calls = %d, want 2", len(groupRequests))
	}
	for _, req := range groupRequests {
		if !req.DisableDebugLog || !req.DisableExport {
			t.Fatalf("group request = %#v, want export/debug disabled for grouped run", req)
		}
		if req.TaskContext.CurrentTestPort != req.Group.Port {
			t.Fatalf("group current port = %d, want %d", req.TaskContext.CurrentTestPort, req.Group.Port)
		}
	}
	if result.OutputFile != "/tmp/result.csv" {
		t.Fatalf("OutputFile = %q, want export path", result.OutputFile)
	}
	if result.TaskContext.CurrentTestPort != 0 {
		t.Fatalf("CurrentTestPort = %d, want 0 for multi-port run", result.TaskContext.CurrentTestPort)
	}
	if result.Summary.Total != 2 || result.Summary.Passed != 2 || result.Summary.Failed != 0 {
		t.Fatalf("Summary = %#v, want total/pass 2 and failed 0", result.Summary)
	}
	gotPorts := []int{result.Results[0].TestPort, result.Results[1].TestPort}
	if !slices.Equal(gotPorts, []int{443, 2053}) {
		t.Fatalf("result ports = %#v, want grouped ports", gotPorts)
	}
	if len(exported) != len(result.RawResults) || len(exported) != 2 {
		t.Fatalf("exported/raw counts = %d/%d, want 2/2", len(exported), len(result.RawResults))
	}
	if !slices.Equal(result.Warnings, []string{"debug warning", "source warning"}) {
		t.Fatalf("Warnings = %#v, want lifecycle + source warnings", result.Warnings)
	}
}

func TestRunProbeWorkflowSingleGroupDelegatesWithoutForcedExportDisable(t *testing.T) {
	var gotReq WorkflowGroupRequest
	result, err := RunProbeWorkflow(WorkflowRunRequest{
		Config: WorkflowConfig{TCPPort: 443},
		Groups: []PortGroup{
			{Port: 2053, IPs: []string{"1.1.1.1"}},
		},
		Source: WorkflowSource{
			Summary: SourceSummary{Valid: []string{"1.1.1.1"}, ValidCount: 1},
			Text:    "1.1.1.1",
		},
		TaskContext: TaskContext{
			GlobalTCPPort: 443,
			PortPolicy:    PortPolicySourceOverrideGlobal,
		},
		TaskID: "task-2",
	}, WorkflowAdapter{
		RunGroup: func(req WorkflowGroupRequest) (WorkflowGroupResult, error) {
			gotReq = req
			return WorkflowGroupResult{
				RawResults: []utils.CloudflareIPData{probeCoreTestData("1.1.1.1", time.Millisecond, 10)},
				Summary:    ProbeSummary{Total: 1, Passed: 1},
			}, nil
		},
	})
	if err != nil {
		t.Fatalf("RunProbeWorkflow() error = %v", err)
	}
	if gotReq.DisableDebugLog || gotReq.DisableExport {
		t.Fatalf("single group request disables side effects: %#v", gotReq)
	}
	if gotReq.TaskContext.CurrentTestPort != 2053 {
		t.Fatalf("single group CurrentTestPort = %d, want source port", gotReq.TaskContext.CurrentTestPort)
	}
	if len(result.Results) != 1 || result.Results[0].TestPort != 2053 {
		t.Fatalf("result rows = %#v, want single row with port 2053", result.Results)
	}
}
