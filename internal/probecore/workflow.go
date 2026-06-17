package probecore

import (
	"errors"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/utils"
)

type WorkflowConfig struct {
	DownloadSpeedMetric string
	PrintNum            int
	TCPPort             int
}

type WorkflowSource struct {
	Summary  SourceSummary
	Text     string
	Warnings []string
}

type WorkflowRunRequest struct {
	Config      WorkflowConfig
	Groups      []PortGroup
	SourcePorts map[string]int
	Source      WorkflowSource
	TaskContext TaskContext
	TaskID      string
}

type WorkflowGroupRequest struct {
	DisableDebugLog bool
	DisableExport   bool
	Group           PortGroup
	SourceText      string
	TaskContext     TaskContext
	TaskID          string
}

type WorkflowGroupResult struct {
	CompletedStages  []string
	DebugLogPath     string
	DurationMS       int64
	FailureStage     string
	OutputFile       string
	RawResults       []utils.CloudflareIPData
	Results          []ProbeRow
	Source           SourceSummary
	StartedAt        string
	Summary          ProbeSummary
	TaskContext      TaskContext
	TraceDiagnostics map[string]any
	Warnings         []string
}

type WorkflowExportRequest struct {
	DebugLogPath string
	RawResults   []utils.CloudflareIPData
	TaskID       string
}

type WorkflowExportResult struct {
	OutputFile string
	Warnings   []string
}

type WorkflowLifecycle struct {
	DebugLogPath string
	StartedAt    time.Time
	Warnings     []string
	Close        func()
}

type WorkflowAdapter struct {
	BeginMultiGroup func(WorkflowRunRequest) (WorkflowLifecycle, error)
	Export          func(WorkflowExportRequest) (WorkflowExportResult, error)
	Now             func() time.Time
	RunGroup        func(WorkflowGroupRequest) (WorkflowGroupResult, error)
}

type WorkflowRunResult struct {
	CompletedStages  []string
	DebugLogPath     string
	DurationMS       int64
	FailureStage     string
	OutputFile       string
	RawResults       []utils.CloudflareIPData
	Results          []ProbeRow
	Source           SourceSummary
	StartedAt        string
	Summary          ProbeSummary
	TaskContext      TaskContext
	TraceDiagnostics map[string]any
	Warnings         []string
}

func RunProbeWorkflow(req WorkflowRunRequest, adapter WorkflowAdapter) (WorkflowRunResult, error) {
	if adapter.RunGroup == nil {
		return WorkflowRunResult{}, errors.New("probe workflow missing RunGroup adapter")
	}

	groups := normalizedWorkflowGroups(req)
	if len(groups) <= 1 {
		return runSingleGroupWorkflow(req, groups, adapter)
	}

	return runMultiGroupWorkflow(req, groups, adapter)
}

func runSingleGroupWorkflow(req WorkflowRunRequest, groups []PortGroup, adapter WorkflowAdapter) (WorkflowRunResult, error) {
	groupReq := singleWorkflowGroupRequest(req, groups)
	groupResult, err := adapter.RunGroup(groupReq)
	return workflowResultFromGroup(req, groupReq, groupResult), err
}

func runMultiGroupWorkflow(req WorkflowRunRequest, groups []PortGroup, adapter WorkflowAdapter) (WorkflowRunResult, error) {
	now := time.Now
	if adapter.Now != nil {
		now = adapter.Now
	}
	lifecycle := WorkflowLifecycle{StartedAt: now()}
	if adapter.BeginMultiGroup != nil {
		started, err := adapter.BeginMultiGroup(req)
		if err != nil {
			return WorkflowRunResult{}, err
		}
		lifecycle = started
		if lifecycle.StartedAt.IsZero() {
			lifecycle.StartedAt = now()
		}
	}
	if lifecycle.Close != nil {
		defer lifecycle.Close()
	}

	combinedRaw := make([]utils.CloudflareIPData, 0, req.Source.Summary.ValidCount)
	combinedRows := make([]ProbeRow, 0, req.Source.Summary.ValidCount)
	completedStages := make([]string, 0, len(groups)*3)
	warnings := append([]string(nil), lifecycle.Warnings...)
	summaryTotal := 0
	debugLogPath := lifecycle.DebugLogPath

	for _, group := range groups {
		groupReq := WorkflowGroupRequest{
			DisableDebugLog: true,
			DisableExport:   true,
			Group:           group,
			SourceText:      strings.Join(group.IPs, "\n"),
			TaskContext:     taskContextForWorkflowPort(req.TaskContext, group.Port),
			TaskID:          req.TaskID,
		}
		groupResult, err := adapter.RunGroup(groupReq)
		completedStages = append(completedStages, groupResult.CompletedStages...)
		warnings = append(warnings, groupResult.Warnings...)
		if groupResult.DebugLogPath != "" {
			debugLogPath = groupResult.DebugLogPath
		}
		if err != nil {
			failed := workflowResultFromGroup(req, groupReq, groupResult)
			failed.DebugLogPath = debugLogPath
			failed.CompletedStages = append([]string(nil), completedStages...)
			failed.Warnings = DedupeStrings(warnings)
			return failed, err
		}
		if groupResult.Summary.Total > 0 {
			summaryTotal += groupResult.Summary.Total
		}
		for _, item := range groupResult.RawResults {
			combinedRaw = append(combinedRaw, item)
			combinedRows = append(combinedRows, ConvertProbeRow(item, sourcePortForIP(req.SourcePorts, item.IP.String()), group.Port))
		}
	}

	selectedRaw, selectedRows := LimitFinalProbeResults(combinedRaw, combinedRows, req.Config.PrintNum, req.Config.DownloadSpeedMetric)
	outputFile := ""
	if len(selectedRaw) > 0 && adapter.Export != nil {
		exportResult, err := adapter.Export(WorkflowExportRequest{
			DebugLogPath: debugLogPath,
			RawResults:   selectedRaw,
			TaskID:       req.TaskID,
		})
		warnings = append(warnings, exportResult.Warnings...)
		if err != nil {
			warnings = append(warnings, err.Error())
		} else {
			outputFile = exportResult.OutputFile
		}
	}

	if summaryTotal <= 0 {
		summaryTotal = len(combinedRows)
		if summaryTotal == 0 {
			summaryTotal = req.Source.Summary.ValidCount
		}
	}

	taskContext := req.TaskContext
	taskContext.CurrentTestPort = 0

	return WorkflowRunResult{
		CompletedStages: append([]string(nil), completedStages...),
		DebugLogPath:    debugLogPath,
		DurationMS:      now().Sub(lifecycle.StartedAt).Milliseconds(),
		OutputFile:      outputFile,
		RawResults:      append([]utils.CloudflareIPData(nil), selectedRaw...),
		Results:         selectedRows,
		Source:          req.Source.Summary,
		StartedAt:       lifecycle.StartedAt.Format(time.RFC3339),
		Summary:         SummarizeProbeRows(selectedRows, summaryTotal),
		TaskContext:     taskContext,
		Warnings:        DedupeStrings(append(warnings, req.Source.Warnings...)),
	}, nil
}

func normalizedWorkflowGroups(req WorkflowRunRequest) []PortGroup {
	if len(req.Groups) > 0 {
		return req.Groups
	}
	globalPort := req.TaskContext.GlobalTCPPort
	if globalPort <= 0 {
		globalPort = req.Config.TCPPort
	}
	if len(req.Source.Summary.Valid) > 0 {
		return PortGroups(req.Source.Summary.Valid, req.SourcePorts, globalPort, req.TaskContext.PortPolicy)
	}
	return nil
}

func singleWorkflowGroupRequest(req WorkflowRunRequest, groups []PortGroup) WorkflowGroupRequest {
	sourceText := req.Source.Text
	taskContext := req.TaskContext
	group := PortGroup{}
	if len(groups) == 1 {
		group = groups[0]
		if len(group.IPs) > 0 {
			sourceText = strings.Join(group.IPs, "\n")
		}
		taskContext = taskContextForWorkflowPort(taskContext, group.Port)
	} else if taskContext.CurrentTestPort <= 0 {
		taskContext = taskContextForWorkflowPort(taskContext, req.Config.TCPPort)
	}
	return WorkflowGroupRequest{
		Group:       group,
		SourceText:  sourceText,
		TaskContext: taskContext,
		TaskID:      req.TaskID,
	}
}

func workflowResultFromGroup(req WorkflowRunRequest, groupReq WorkflowGroupRequest, groupResult WorkflowGroupResult) WorkflowRunResult {
	rows := groupResult.Results
	if len(rows) == 0 && len(groupResult.RawResults) > 0 {
		rows = make([]ProbeRow, 0, len(groupResult.RawResults))
		for _, item := range groupResult.RawResults {
			rows = append(rows, ConvertProbeRow(item, sourcePortForIP(req.SourcePorts, item.IP.String()), groupReq.Group.Port))
		}
	}
	source := groupResult.Source
	if source.CandidateCount == 0 && source.ValidCount == 0 {
		source = req.Source.Summary
	}
	taskContext := groupResult.TaskContext
	if taskContext.PortPolicy == "" {
		taskContext = groupReq.TaskContext
	}
	summary := groupResult.Summary
	if summary.Total <= 0 {
		summary = SummarizeProbeRows(rows, len(rows))
	}
	return WorkflowRunResult{
		CompletedStages:  append([]string(nil), groupResult.CompletedStages...),
		DebugLogPath:     groupResult.DebugLogPath,
		DurationMS:       groupResult.DurationMS,
		FailureStage:     groupResult.FailureStage,
		OutputFile:       groupResult.OutputFile,
		RawResults:       append([]utils.CloudflareIPData(nil), groupResult.RawResults...),
		Results:          rows,
		Source:           source,
		StartedAt:        groupResult.StartedAt,
		Summary:          summary,
		TaskContext:      taskContext,
		TraceDiagnostics: groupResult.TraceDiagnostics,
		Warnings:         DedupeStrings(groupResult.Warnings),
	}
}

func taskContextForWorkflowPort(taskContext TaskContext, port int) TaskContext {
	if taskContext.PortPolicy == "" {
		taskContext.PortPolicy = PortPolicySourceOverrideGlobal
	}
	if port > 0 {
		taskContext.CurrentTestPort = port
		if taskContext.GlobalTCPPort <= 0 {
			taskContext.GlobalTCPPort = port
		}
	}
	return taskContext
}

func sourcePortForIP(sourcePorts map[string]int, ip string) int {
	if len(sourcePorts) == 0 {
		return 0
	}
	return sourcePorts[strings.TrimSpace(ip)]
}
