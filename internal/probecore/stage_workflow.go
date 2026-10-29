package probecore

import (
	"errors"
	"math"
	"slices"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/utils"
)

const (
	StageTCP      = "stage1_tcp"
	StageTrace    = "stage2_trace"
	StageDownload = "stage3_get"
)

var ErrProbeCanceled = errors.New("task canceled")

type StageWorkflowConfig struct {
	DisableDownload     bool
	DisableResultLimit  bool
	DownloadSpeedMetric string
	PrintNum            int
	Stage2Limit         int
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
	AfterStage         func(StageInfo) error
	BeforeStage        func(StageInfo) error
	ConfigureProgress  func(StageInfo)
	EmitEvent          func(event string, fields map[string]any)
	EstimateTraceTotal func(int) int
	Now                func() time.Time
	RunDownload        func(utils.PingDelaySet) utils.DownloadSpeedSet
	RunTCP             func() (utils.PingDelaySet, error)
	RunTrace           func(utils.PingDelaySet) utils.PingDelaySet
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
	if err != nil {
		return failedStageWorkflowResult(req, baseWarnings, completedStages, StageTCP), err
	}
	completedStages = append(completedStages, StageTCP)

	if len(tcpData) == 0 {
		warnings := BuildProbeWarnings(req.Source)
		warnings = append(warnings, req.ConfigWarnings...)
		warnings = append(warnings, req.DebugWarnings...)
		warnings = append(warnings, "TCP 测延迟未获得可用候选，后续追踪与文件测速未执行。")
		return emptyStageWorkflowResult(req, warnings, completedStages, StageTCP), nil
	}
	traceInput := SortAndLimitPingDelaySet(tcpData, req.Config.Stage2Limit)
	adapter.emitStageTruncation(StageTrace, "stage2_limit", len(tcpData), len(traceInput), req.Config.Stage2Limit)
	traceTotal := len(traceInput)
	if adapter.EstimateTraceTotal != nil {
		traceTotal = adapter.EstimateTraceTotal(len(traceInput))
	}
	stage2 := StageInfo{Stage: StageTrace, Input: len(traceInput), Total: traceTotal}
	traceData, err := runTraceStage(stage2, traceInput, adapter, now)
	if err != nil {
		return failedStageWorkflowResult(req, baseWarnings, completedStages, StageTrace), err
	}
	completedStages = append(completedStages, StageTrace)

	warnings := BuildProbeWarnings(req.Source)
	warnings = append(warnings, req.ConfigWarnings...)
	warnings = append(warnings, req.DebugWarnings...)
	if len(traceData) == 0 {
		warnings = append(warnings, "追踪探测未命中可用候选，后续文件测速未执行。")
		return emptyStageWorkflowResult(req, warnings, completedStages, StageTrace), nil
	}

	resultData := []utils.CloudflareIPData(traceData)
	summaryTotal := req.Source.CandidateCount
	if !req.Config.DisableDownload {
		downloadInput := SortAndLimitHeadDelaySet(traceData, req.Config.Stage3Limit)
		adapter.emitStageTruncation(StageDownload, "stage3_limit", len(traceData), len(downloadInput), req.Config.Stage3Limit)
		downloadTotal := EstimateDownloadProbeCount(len(downloadInput))
		stage3 := StageInfo{Stage: StageDownload, Input: len(downloadInput), Total: downloadTotal}
		speedData, err := runDownloadStage(stage3, downloadInput, adapter, now)
		if err != nil {
			return failedStageWorkflowResult(req, warnings, completedStages, StageDownload), err
		}
		completedStages = append(completedStages, StageDownload)
		resultData = []utils.CloudflareIPData(speedData)
		summaryTotal = downloadTotal
	}

	if !req.Config.DisableResultLimit {
		limited := LimitFinalResults(resultData, req.Config.PrintNum, req.Config.DownloadSpeedMetric)
		adapter.emitStageTruncation("result", "result_limit", len(resultData), len(limited), req.Config.PrintNum)
		resultData = limited
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
			PortPolicy:      PortPolicyFixedGlobal,
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
	tcpData, err := adapter.RunTCP()
	if err != nil {
		info.DurationMS = now().Sub(stageStart).Milliseconds()
		return nil, err
	}
	info.DurationMS = now().Sub(stageStart).Milliseconds()
	info.Passed = len(tcpData)
	info.Failed = StageFailedCount(info.Total, info.Passed)
	adapter.emitStageMetrics(info.Stage, "ms", pingDelayValues(tcpData, false), samplePingRows(tcpData, stageSampleLimit))
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
	adapter.emitStageMetrics(info.Stage, "ms", pingDelayValues(traceData, true), samplePingRows(traceData, stageSampleLimit))
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
	adapter.emitStageMetrics(info.Stage, "mb_per_s", downloadSpeedValues(speedData), sampleSpeedRows(speedData, stageSampleLimit))
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

func emptyStageWorkflowResult(req StageWorkflowRequest, warnings []string, completedStages []string, currentStage string) StageWorkflowResult {
	taskContext := req.TaskContext
	if taskContext.PortPolicy == "" {
		taskContext = TaskContext{
			CurrentTestPort: req.Config.TCPPort,
			GlobalTCPPort:   req.Config.TCPPort,
			PortPolicy:      PortPolicyFixedGlobal,
		}
	}
	if taskContext.CurrentTestPort <= 0 {
		taskContext.CurrentTestPort = req.Config.TCPPort
	}
	return StageWorkflowResult{
		CompletedStages: append([]string(nil), completedStages...),
		CurrentStage:    currentStage,
		Summary:         SummarizeProbeRows(nil, req.Source.CandidateCount),
		TaskContext:     taskContext,
		Warnings:        DedupeStrings(warnings),
	}
}

func EstimateDownloadProbeCount(candidateCount int) int {
	if candidateCount <= 0 {
		return 0
	}
	return candidateCount
}

func SortAndLimitPingDelaySet(ipSet utils.PingDelaySet, limit int) utils.PingDelaySet {
	result := append(utils.PingDelaySet(nil), ipSet...)
	slices.SortStableFunc(result, func(left, right utils.CloudflareIPData) int {
		if left.Delay < right.Delay {
			return -1
		}
		if left.Delay > right.Delay {
			return 1
		}
		return 0
	})
	if limit > 0 && len(result) > limit {
		return result[:limit]
	}
	return result
}

func SortAndLimitHeadDelaySet(ipSet utils.PingDelaySet, limit int) utils.PingDelaySet {
	result := append(utils.PingDelaySet(nil), ipSet...)
	slices.SortStableFunc(result, func(left, right utils.CloudflareIPData) int {
		if left.HeadDelay <= 0 && right.HeadDelay > 0 {
			return 1
		}
		if left.HeadDelay > 0 && right.HeadDelay <= 0 {
			return -1
		}
		if left.HeadDelay < right.HeadDelay {
			return -1
		}
		if left.HeadDelay > right.HeadDelay {
			return 1
		}
		return 0
	})
	if limit > 0 && len(result) > limit {
		return result[:limit]
	}
	return result
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

// stageSampleLimit 限制逐 IP 抽样条数：全量明细在数千 IP 的任务里会把日志归档挤掉。
const stageSampleLimit = 10

func (adapter StageWorkflowAdapter) emitStageEvent(event string, fields map[string]any) {
	if adapter.EmitEvent == nil {
		return
	}
	adapter.EmitEvent(event, fields)
}

// emitStageTruncation 记录阶段间排序截断丢掉多少 IP；此前这些 IP 在日志里静默消失。
func (adapter StageWorkflowAdapter) emitStageTruncation(stage, limitKind string, input, kept, limit int) {
	if input <= kept {
		return
	}
	adapter.emitStageEvent("stage.detail", map[string]any{
		"counts":     map[string]any{"dropped": input - kept, "input": input, "kept": kept},
		"limit":      limit,
		"limit_kind": limitKind,
		"message":    "阶段间排序截断，只保留排序靠前的 IP 进入后续阶段。",
		"reason":     "stage_limit_dropped",
		"stage":      stage,
	})
}

// emitStageMetrics 输出阶段指标分位数与逐 IP 抽样，让漏斗计数带上分布形态。
func (adapter StageWorkflowAdapter) emitStageMetrics(stage, unit string, values []float64, samples []map[string]any) {
	if len(values) == 0 {
		return
	}
	adapter.emitStageEvent("stage.detail", map[string]any{
		"counts":  map[string]any{"count": len(values)},
		"message": "阶段指标分布。",
		"metrics": metricPercentiles(values),
		"reason":  "stage_distribution",
		"stage":   stage,
		"unit":    unit,
	})
	if len(samples) == 0 {
		return
	}
	adapter.emitStageEvent("stage.detail", map[string]any{
		"counts":  map[string]any{"sampled": len(samples)},
		"message": "阶段逐 IP 抽样明细。",
		"reason":  "stage_sample",
		"samples": samples,
		"stage":   stage,
	})
}

func metricPercentiles(values []float64) map[string]any {
	sorted := append([]float64(nil), values...)
	slices.Sort(sorted)
	pick := func(quantile float64) float64 {
		// 秩取最近整数：样本很少时 p90 不该低于 max。
		return roundMetric(sorted[int(math.Round(quantile*float64(len(sorted)-1)))])
	}
	return map[string]any{
		"max": roundMetric(sorted[len(sorted)-1]), "min": roundMetric(sorted[0]),
		"p50": pick(0.5), "p90": pick(0.9), "p99": pick(0.99),
	}
}

func roundMetric(value float64) float64 {
	return math.Round(value*100) / 100
}

func pingDelayValues(data utils.PingDelaySet, head bool) []float64 {
	values := make([]float64, 0, len(data))
	for _, item := range data {
		if item.PingData == nil {
			continue
		}
		delay := item.Delay
		if head {
			delay = item.HeadDelay
		}
		if delay <= 0 {
			continue
		}
		values = append(values, delay.Seconds()*1000)
	}
	return values
}

func downloadSpeedValues(data utils.DownloadSpeedSet) []float64 {
	values := make([]float64, 0, len(data))
	for _, item := range data {
		values = append(values, item.DownloadSpeed/1024/1024)
	}
	return values
}

func samplePingRows(data utils.PingDelaySet, limit int) []map[string]any {
	rows := make([]map[string]any, 0, limit)
	for _, item := range data {
		if len(rows) >= limit || item.PingData == nil {
			continue
		}
		row := map[string]any{"delay_ms": roundMetric(item.Delay.Seconds() * 1000), "ip": item.IP.String()}
		if item.Colo != "" {
			row["colo"] = item.Colo
		}
		if item.HeadDelay > 0 {
			row["head_delay_ms"] = roundMetric(item.HeadDelay.Seconds() * 1000)
		}
		if item.Sended > 0 {
			row["received"], row["sent"] = item.Received, item.Sended
			row["loss_rate"] = roundMetric(float64(item.Sended-item.Received) / float64(item.Sended))
		}
		rows = append(rows, row)
	}
	return rows
}

func sampleSpeedRows(data utils.DownloadSpeedSet, limit int) []map[string]any {
	rows := make([]map[string]any, 0, limit)
	for _, item := range data {
		if len(rows) >= limit || item.PingData == nil {
			continue
		}
		row := map[string]any{"ip": item.IP.String(), "speed_mb_s": roundMetric(item.DownloadSpeed / 1024 / 1024)}
		if item.Colo != "" {
			row["colo"] = item.Colo
		}
		if item.MaxDownloadSpeed > 0 {
			row["max_speed_mb_s"] = roundMetric(item.MaxDownloadSpeed / 1024 / 1024)
		}
		rows = append(rows, row)
	}
	return rows
}
