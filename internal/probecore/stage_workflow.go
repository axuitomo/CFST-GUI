package probecore

import (
	"errors"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/task"
	"github.com/axuitomo/CFST-GUI/internal/utils"
)

const (
	StageTCP      = "stage1_tcp"
	StageTrace    = "stage2_trace"
	StageDownload = "stage3_get"
)

type StageWorkflowConfig struct {
	DisableDownload     bool
	DisableResultLimit  bool
	DownloadSpeedMetric string
	PrintNum            int
	Stage3Limit         int
	TCPPort             int
}

type StageWorkflowRequest struct {
	Config         StageWorkflowConfig
	ConfigWarnings []string
	DebugWarnings  []string
	SourcePorts    map[string]int
	Source         SourceSummary
	TaskContext    TaskContext
}

type StageInfo struct {
	DurationMS int64
	Failed     int
	Input      int
	Passed     int
	Stage      string
	Total      int
}

type StageWorkflowAdapter struct {
	AfterStage        func(StageInfo) error
	BeforeStage       func(StageInfo) error
	ConfigureProgress func(StageInfo)
	Now               func() time.Time
	RunDownload       func(utils.PingDelaySet) utils.DownloadSpeedSet
	RunTCP            func() utils.PingDelaySet
	RunTrace          func(utils.PingDelaySet) utils.PingDelaySet
}

type StageWorkflowResult struct {
	CompletedStages []string
	CurrentStage    string
	RawResults      []utils.CloudflareIPData
	Results         []ProbeRow
	Summary         ProbeSummary
	TaskContext     TaskContext
	Warnings        []string
}

func RunProbeStages(req StageWorkflowRequest, adapter StageWorkflowAdapter) (StageWorkflowResult, error) {
	if adapter.RunTCP == nil {
		return StageWorkflowResult{}, errors.New("probe stage workflow missing RunTCP adapter")
	}
	if adapter.RunTrace == nil {
		return StageWorkflowResult{}, errors.New("probe stage workflow missing RunTrace adapter")
	}

	now := time.Now
	if adapter.Now != nil {
		now = adapter.Now
	}

	completedStages := make([]string, 0, 3)
	baseWarnings := append([]string(nil), req.ConfigWarnings...)
	baseWarnings = append(baseWarnings, req.DebugWarnings...)

	stage1 := StageInfo{Stage: StageTCP, Total: req.Source.ValidCount}
	tcpData, err := runTCPStage(stage1, adapter, now)
	completedStages = append(completedStages, StageTCP)
	if err != nil {
		return failedStageWorkflowResult(req, baseWarnings, completedStages, StageTCP), err
	}

	traceTotal := task.EstimateTraceProbeCount(len(tcpData))
	stage2 := StageInfo{Stage: StageTrace, Input: len(tcpData), Total: traceTotal}
	traceData, err := runTraceStage(stage2, tcpData, adapter, now)
	completedStages = append(completedStages, StageTrace)
	if err != nil {
		return failedStageWorkflowResult(req, baseWarnings, completedStages, StageTrace), err
	}

	warnings := BuildProbeWarnings(req.Source)
	warnings = append(warnings, req.ConfigWarnings...)
	warnings = append(warnings, req.DebugWarnings...)
	if len(traceData) == 0 && len(tcpData) > 0 {
		warnings = append(warnings, "追踪探测未命中可用候选，已无可导出的结果。")
	}

	resultData := []utils.CloudflareIPData(traceData)
	summaryTotal := req.Source.CandidateCount
	if !req.Config.DisableDownload {
		downloadInput := LimitPingDelaySet(traceData, req.Config.Stage3Limit)
		downloadTotal := EstimateDownloadProbeCount(len(downloadInput))
		stage3 := StageInfo{Stage: StageDownload, Input: len(downloadInput), Total: downloadTotal}
		speedData, err := runDownloadStage(stage3, downloadInput, adapter, now)
		completedStages = append(completedStages, StageDownload)
		if err != nil {
			return failedStageWorkflowResult(req, warnings, completedStages, StageDownload), err
		}
		resultData = []utils.CloudflareIPData(speedData)
		summaryTotal = downloadTotal
	}

	if !req.Config.DisableResultLimit {
		resultData = LimitFinalResults(resultData, req.Config.PrintNum, req.Config.DownloadSpeedMetric)
	}
	rows := make([]ProbeRow, 0, len(resultData))
	for _, item := range resultData {
		rows = append(rows, ConvertProbeRow(item, sourcePortForIP(req.SourcePorts, item.IP.String()), req.Config.TCPPort))
	}

	taskContext := req.TaskContext
	if taskContext.PortPolicy == "" {
		taskContext = TaskContext{
			CurrentTestPort: req.Config.TCPPort,
			GlobalTCPPort:   req.Config.TCPPort,
			PortPolicy:      PortPolicySourceOverrideGlobal,
		}
	}
	if taskContext.CurrentTestPort <= 0 {
		taskContext.CurrentTestPort = req.Config.TCPPort
	}

	return StageWorkflowResult{
		CompletedStages: completedStages,
		CurrentStage:    lastStage(completedStages),
		RawResults:      append([]utils.CloudflareIPData(nil), resultData...),
		Results:         rows,
		Summary:         SummarizeProbeRows(rows, summaryTotal),
		TaskContext:     taskContext,
		Warnings:        DedupeStrings(warnings),
	}, nil
}

func RunTCPStage(info StageInfo, adapter StageWorkflowAdapter) (utils.PingDelaySet, error) {
	now := time.Now
	if adapter.Now != nil {
		now = adapter.Now
	}
	return runTCPStage(info, adapter, now)
}

func RunTraceStage(info StageInfo, input utils.PingDelaySet, adapter StageWorkflowAdapter) (utils.PingDelaySet, error) {
	now := time.Now
	if adapter.Now != nil {
		now = adapter.Now
	}
	return runTraceStage(info, input, adapter, now)
}

func RunDownloadStage(info StageInfo, input utils.PingDelaySet, adapter StageWorkflowAdapter) (utils.DownloadSpeedSet, error) {
	now := time.Now
	if adapter.Now != nil {
		now = adapter.Now
	}
	return runDownloadStage(info, input, adapter, now)
}

func runTCPStage(info StageInfo, adapter StageWorkflowAdapter, now func() time.Time) (utils.PingDelaySet, error) {
	if err := beforeStage(info, adapter); err != nil {
		return nil, err
	}
	stageStart := now()
	tcpData := adapter.RunTCP()
	info.DurationMS = now().Sub(stageStart).Milliseconds()
	info.Passed = len(tcpData)
	info.Failed = StageFailedCount(info.Total, info.Passed)
	return tcpData, afterStage(info, adapter)
}

func runTraceStage(info StageInfo, tcpData utils.PingDelaySet, adapter StageWorkflowAdapter, now func() time.Time) (utils.PingDelaySet, error) {
	if err := beforeStage(info, adapter); err != nil {
		return nil, err
	}
	stageStart := now()
	traceData := adapter.RunTrace(tcpData)
	info.DurationMS = now().Sub(stageStart).Milliseconds()
	info.Passed = len(traceData)
	info.Failed = StageFailedCount(info.Total, info.Passed)
	return traceData, afterStage(info, adapter)
}

func runDownloadStage(info StageInfo, downloadInput utils.PingDelaySet, adapter StageWorkflowAdapter, now func() time.Time) (utils.DownloadSpeedSet, error) {
	if adapter.RunDownload == nil {
		return utils.DownloadSpeedSet(downloadInput), nil
	}
	if err := beforeStage(info, adapter); err != nil {
		return nil, err
	}
	stageStart := now()
	speedData := adapter.RunDownload(downloadInput)
	info.DurationMS = now().Sub(stageStart).Milliseconds()
	info.Passed = len(speedData)
	info.Failed = StageFailedCount(info.Total, info.Passed)
	return speedData, afterStage(info, adapter)
}

func beforeStage(info StageInfo, adapter StageWorkflowAdapter) error {
	if adapter.ConfigureProgress != nil {
		adapter.ConfigureProgress(info)
	}
	if adapter.BeforeStage != nil {
		return adapter.BeforeStage(info)
	}
	return nil
}

func afterStage(info StageInfo, adapter StageWorkflowAdapter) error {
	if adapter.AfterStage != nil {
		return adapter.AfterStage(info)
	}
	return nil
}

func failedStageWorkflowResult(req StageWorkflowRequest, warnings []string, completedStages []string, currentStage string) StageWorkflowResult {
	return StageWorkflowResult{
		CompletedStages: append([]string(nil), completedStages...),
		CurrentStage:    currentStage,
		TaskContext:     req.TaskContext,
		Warnings:        DedupeStrings(warnings),
	}
}

func EstimateDownloadProbeCount(candidateCount int) int {
	if task.Disable || candidateCount <= 0 {
		return 0
	}
	return candidateCount
}

func LimitPingDelaySet(ipSet utils.PingDelaySet, limit int) utils.PingDelaySet {
	if limit <= 0 || len(ipSet) <= limit {
		return ipSet
	}
	return ipSet[:limit]
}

func StageFailedCount(total, passed int) int {
	failed := total - passed
	if failed < 0 {
		return 0
	}
	return failed
}

func lastStage(stages []string) string {
	if len(stages) == 0 {
		return ""
	}
	return stages[len(stages)-1]
}
