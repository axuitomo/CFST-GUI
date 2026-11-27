package appcore

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/kernel/appcore/contracts"
	"github.com/axuitomo/CFST-GUI/internal/kernel/appcore/diagnostics"
	"github.com/axuitomo/CFST-GUI/internal/kernel/appcore/probe"
	"github.com/axuitomo/CFST-GUI/internal/kernel/domain/probe/colodict"
	"github.com/axuitomo/CFST-GUI/internal/kernel/domain/probe/probecore"
	"github.com/axuitomo/CFST-GUI/internal/kernel/domain/probe/task"
	"github.com/axuitomo/CFST-GUI/internal/kernel/infra/httpcfg"
	"github.com/axuitomo/CFST-GUI/internal/kernel/infra/utils"
)

type probeGroupRequest struct {
	Config            probecore.ProbeConfig
	ConfigWarnings    []string
	DisableDebugLog   bool
	DisableExport     bool
	MCISCandidates    map[string]probecore.MCISCandidate
	SourceColoFilters task.SourceColoFilterMap
	SourcePorts       map[string]int
	SourceStatuses    []contracts.SourceStatus
	SourceText        string
	Source            probecore.SourceSummary
	TaskContext       probecore.TaskContext
	TaskID            string
}

func (s *Service) runProbeGroup(ctx context.Context, request probeGroupRequest) (contracts.ProbeRunResult, error) {
	started := s.now()
	cfg, normalizeWarnings := probecore.NormalizeProbeConfig(request.Config, s.probeNormalizeOptions())
	warnings := append(append([]string(nil), request.ConfigWarnings...), normalizeWarnings...)
	closeLog := func() {}
	debugWarnings := []string{}
	debugLogPath := s.probeDebugLogPath(cfg)
	if !request.DisableDebugLog {
		closeLog, debugWarnings, debugLogPath = s.configureProbeDebug(cfg, request.TaskID)
	}
	defer closeLog()

	s.debugProbeStart(cfg, request.TaskID, request.SourceStatuses)
	source := request.Source
	var err error
	if source.ValidCount == 0 {
		// 调用方没有携带预解析结果时才回退到文本解析（保留 CLI 与测试路径）。
		_, parsed, parseErr := resolveServiceProbeSource(cfg, request.SourceText, s.sourceResolver(cfg))
		source, err = parsed, parseErr
	}
	if err != nil || source.ValidCount == 0 {
		if err == nil {
			err = errors.New("没有可用的 IP/CIDR/域名输入")
		}
		s.logProbeFailed(request.TaskID, "stage0_pool", started, nil, err, false, map[string]any{"debug_log_path": debugLogPath})
		return contracts.ProbeRunResult{}, err
	}
	if s.probeCancelled(ctx, request.TaskID) {
		return contracts.ProbeRunResult{}, probecore.ErrProbeCanceled
	}
	cfg.IPText = strings.Join(source.Valid, ",")
	traceDiagnostics := diagnostics.NewTraceDiagnostics(cfg.TraceColoMode, cfg.TraceURL)
	baseHooks := task.Hooks{
		DebugEvent: s.DebugLogger().Event, DebugEnabled: s.DebugLogger().Enabled,
		EdgeRejected:    func(rejection task.EdgeRejection) { s.publishEdgeRejected(request.TaskID, rejection) },
		TraceDiagnostic: traceDiagnostics.Record,
		ProbePause:      func(stage, ip string) { s.waitProbePaused(request.TaskID, stage, ip) },
		ProbeCancel:     func(stage, ip string) bool { return s.runtime.IsCancelRequested(request.TaskID) },
		ProbeContext:    func() context.Context { return ctx },
		TraceInterrupt: func(stage, ip string, interrupt func()) func() {
			return s.runtime.RegisterInterrupt(request.TaskID, stage, interrupt)
		},
		DownloadInterrupt: func(stage, ip string, interrupt func()) func() {
			return s.runtime.RegisterInterrupt(request.TaskID, stage, interrupt)
		},
	}
	engineCfg := cfg
	reuseMCIS := cfg.TCPPort == 443 && len(request.MCISCandidates) > 0
	if reuseMCIS {
		unmeasured := make([]string, 0, len(source.Valid))
		for _, ip := range source.Valid {
			if _, ok := request.MCISCandidates[ip]; !ok {
				unmeasured = append(unmeasured, ip)
			}
		}
		engineCfg.IPText = strings.Join(unmeasured, ",")
	}
	engine, err := probe.NewProbeEngine(engineCfg, probe.ProbeEngineOptions{
		CaptureAddress: effectiveServiceCaptureAddress(cfg), ColoPaths: colodict.DefaultPaths(s.StorageLayout().Root),
		OutputFile: currentServiceOutputFile(cfg), SourceColoFilter: request.SourceColoFilters, Hooks: baseHooks,
	})
	if err != nil {
		return contracts.ProbeRunResult{}, err
	}
	s.DebugLogger().Event("stage.complete", map[string]any{
		"counts": debugStage0Counts(source, source.InvalidCount), "duration_ms": s.now().Sub(started).Milliseconds(),
		"message": "IP 池生成完成。", "source": debugSourceSummary(source, request.SourceStatuses),
		"stage": "stage0_pool", "task_id": request.TaskID,
	})
	completedStages := []string{"stage0_pool"}
	stageResult, err := probecore.RunProbeStages(probecore.StageWorkflowRequest{
		Config: probecore.StageWorkflowConfig{
			DisableDownload: cfg.DisableDownload, DisableResultLimit: request.DisableExport,
			DownloadSpeedMetric: cfg.DownloadSpeedMetric, PrintNum: cfg.PrintNum,
			Stage2Limit: cfg.HeadTestCount, Stage3Limit: cfg.Stage3Limit, TCPPort: cfg.TCPPort,
		},
		ConfigWarnings: warnings, DebugWarnings: debugWarnings, SourcePorts: request.SourcePorts,
		Source: source, TaskContext: request.TaskContext,
	}, probecore.StageWorkflowAdapter{
		ConfigureProgress: func(info probecore.StageInfo) {
			s.configureStageProgress(engine, baseHooks, request.TaskID, info, cfg.DownloadSuccessLimit)
		},
		EmitEvent:          s.DebugLogger().Event,
		EstimateTraceTotal: engine.EstimateTraceProbeCount,
		BeforeStage: func(info probecore.StageInfo) error {
			s.beforeProbeStage(engine, cfg, request.TaskID, info)
			return nil
		},
		AfterStage: func(info probecore.StageInfo) error {
			s.afterProbeStage(engine, cfg, request.TaskID, info)
			if s.runtime.IsCancelRequested(request.TaskID) {
				return probecore.ErrProbeCanceled
			}
			return nil
		},
		Now: s.now,
		RunTCP: func() (utils.PingDelaySet, error) {
			return s.runTCPWithMCIS(engine, request.MCISCandidates, source.Valid, reuseMCIS)
		},
		RunTrace:    func(input utils.PingDelaySet) utils.PingDelaySet { return s.runTrace(engine, input) },
		RunDownload: func(input utils.PingDelaySet) utils.DownloadSpeedSet { return s.runDownload(engine, input) },
	})
	completedStages = append(completedStages, stageResult.CompletedStages...)
	// worker panic 只写调试日志，关掉 debug 就完全无声；用 warning 把“结果为什么少了”说出来。
	if panics := engine.WorkerPanicCount(); panics > 0 {
		stageResult.Warnings = append(stageResult.Warnings, fmt.Sprintf("有 %d 个探测 worker 异常退出（程序内部错误），对应 IP 已按失败处理；结果可能少于候选数。重新运行并开启调试日志可看到具体 IP 与原因。", panics))
	}
	// 启动自检没补上 COLO 词典时也说出来：否则结果里 COLO 全空，用户无从判断是网络还是程序问题。
	if warning := s.ColoDictionaryWarning(); warning != "" {
		stageResult.Warnings = append(stageResult.Warnings, warning+"；COLO 仍可从 trace 获取，结果里的 COLO 列可能为空。")
	}
	if err != nil {
		failureStage := ""
		tracePayload := traceDiagnostics.Payload()
		if !errors.Is(err, probecore.ErrProbeCanceled) && !traceDiagnostics.Empty() && stageResult.CurrentStage == probecore.StageTrace {
			failureStage = probecore.StageTrace
			rawError := err.Error()
			summary := diagnostics.StageTraceFailureMessage(traceDiagnostics.Summary(), rawError)
			if tracePayload == nil {
				tracePayload = map[string]any{}
			}
			tracePayload["raw_error"], tracePayload["summary"] = rawError, summary
			err = errors.New(summary)
		}
		if !errors.Is(err, probecore.ErrProbeCanceled) {
			extra := tracePayload
			if extra == nil {
				extra = map[string]any{}
			}
			extra["debug_log_path"] = debugLogPath
			s.logProbeFailed(request.TaskID, stageResult.CurrentStage, started, completedStages, err, false, extra)
		}
		return contracts.ProbeRunResult{DebugLogPath: debugLogPath, FailureStage: failureStage, TraceDiagnostics: tracePayload, Warnings: probecore.DedupeStrings(stageResult.Warnings)}, err
	}
	if s.probeCancelled(ctx, request.TaskID) {
		return contracts.ProbeRunResult{DebugLogPath: debugLogPath, Warnings: probecore.DedupeStrings(stageResult.Warnings)}, probecore.ErrProbeCanceled
	}
	outputFile := ""
	resultWarnings := append([]string(nil), stageResult.Warnings...)
	if len(stageResult.RawResults) > 0 && !request.DisableExport {
		exported, exportErr := s.exportProbeResults(ctx, cfg, request.TaskID, debugLogPath, stageResult.RawResults)
		resultWarnings = append(resultWarnings, exported.Warnings...)
		if exportErr != nil {
			return contracts.ProbeRunResult{DebugLogPath: debugLogPath, Warnings: probecore.DedupeStrings(resultWarnings)}, exportErr
		}
		outputFile = exported.OutputFile
	}
	result := contracts.ProbeRunResult{
		Config: cfg, DebugLogPath: debugLogPath, DurationMS: s.now().Sub(started).Milliseconds(),
		OutputFile: outputFile, RawResults: append([]utils.CloudflareIPData(nil), stageResult.RawResults...),
		Results: stageResult.Results, SchemaVersion: CommandSchemaVersion, Source: source,
		StartedAt: started.Format(time.RFC3339), Summary: stageResult.Summary, TaskID: request.TaskID, TaskContext: stageResult.TaskContext,
		TraceDiagnostics: traceDiagnostics.Payload(), Warnings: probecore.DedupeStrings(resultWarnings),
	}
	if diagnostics.ShouldMarkTraceFailureStage(stageResult.CompletedStages, traceDiagnostics, stageResult.RawResults) {
		result.FailureStage = probecore.StageTrace
	}
	s.DebugLogger().Event("probe.complete", map[string]any{
		"counts":      map[string]any{"exported": len(result.Results), "failed": result.Summary.Failed, "passed": result.Summary.Passed, "total": result.Summary.Total},
		"duration_ms": result.DurationMS, "message": "探测任务完成。", "output_file": result.OutputFile,
		"completed_stages": completedStages, "task_id": request.TaskID, "warnings": result.Warnings,
	})
	return result, nil
}

func (s *Service) exportProbeResults(ctx context.Context, cfg probecore.ProbeConfig, taskID, debugLogPath string, rows []utils.CloudflareIPData) (probecore.WorkflowExportResult, error) {
	s.waitProbePaused(taskID, "export", "")
	if s.probeCancelled(ctx, taskID) {
		return probecore.WorkflowExportResult{}, probecore.ErrProbeCanceled
	}
	outputFile := currentServiceOutputFile(cfg)
	if outputFile == "" {
		return probecore.WorkflowExportResult{}, nil
	}
	if err := os.MkdirAll(filepath.Dir(outputFile), 0o755); err != nil {
		return probecore.WorkflowExportResult{Warnings: []string{fmt.Sprintf("创建导出目录失败：%v", err)}}, nil
	}
	writer := utils.CSVWriter{Path: outputFile, Append: cfg.ExportAppend, Encoding: cfg.CSVEncoding}
	if err := writer.ExportContext(ctx, rows, func() { s.waitProbePaused(taskID, "export", "") }); err != nil {
		if errors.Is(err, context.Canceled) {
			return probecore.WorkflowExportResult{}, probecore.ErrProbeCanceled
		}
		return probecore.WorkflowExportResult{Warnings: []string{fmt.Sprintf("结果导出失败：%v", err)}}, nil
	}
	if s.probeCancelled(ctx, taskID) {
		return probecore.WorkflowExportResult{}, probecore.ErrProbeCanceled
	}
	_ = s.PublishProbeEvent(context.Background(), ProbeEvent{Event: "probe.partial_export", TaskID: taskID, Payload: withProbeDebugLogPath(map[string]any{"target_path": outputFile, "written": len(rows)}, debugLogPath)})
	s.DebugLogger().Event("probe.export", map[string]any{"counts": map[string]any{"written": len(rows)}, "message": "CSV 导出完成。", "target_path": outputFile, "task_id": taskID})
	return probecore.WorkflowExportResult{OutputFile: outputFile}, nil
}

func (s *Service) probeNormalizeOptions() probecore.ProbeConfigNormalizeOptions {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.options.ProbeConfigOptions.ProbeNormalizeOptions
}

func (s *Service) runTCP(engine *task.Engine) (utils.PingDelaySet, error) {
	s.mu.RLock()
	run := s.options.ProbeHooks.RunTCP
	s.mu.RUnlock()
	if run != nil {
		return run(engine)
	}
	ping, err := engine.NewPing()
	if err != nil {
		return nil, err
	}
	return engine.FilterPingResults(ping.Run()), nil
}

func (s *Service) runTCPWithMCIS(engine *task.Engine, candidates map[string]probecore.MCISCandidate, sourceIPs []string, reuse bool) (utils.PingDelaySet, error) {
	if !reuse {
		return s.runTCP(engine)
	}

	result := make(utils.PingDelaySet, 0, len(sourceIPs))
	if strings.TrimSpace(engine.Config().IPText) != "" {
		measured, err := s.runTCP(engine)
		if err != nil {
			return nil, err
		}
		result = append(result, measured...)
	}
	for _, ipText := range sourceIPs {
		candidate, ok := candidates[ipText]
		if !ok {
			continue
		}
		sent, received := candidate.PrefixSamples, candidate.PrefixOK
		if sent <= 0 {
			sent, received = 1, 1
		}
		result = append(result, utils.CloudflareIPData{
			PingData:   &utils.PingData{IP: &net.IPAddr{IP: net.ParseIP(candidate.IP)}, Sended: sent, Received: received, Delay: time.Duration(candidate.TotalMS) * time.Millisecond, Colo: candidate.Colo},
			MCISPrefix: candidate.Prefix, MCISConnectMS: candidate.ConnectMS, MCISTLSMS: candidate.TLSMS,
			MCISTTFBMS: candidate.TTFBMS, MCISTotalMS: candidate.TotalMS, MCISPrefixSamples: candidate.PrefixSamples,
			MCISPrefixOK: candidate.PrefixOK, MCISPrefixFail: candidate.PrefixFail,
		})
	}
	return engine.FilterPingResults(result), nil
}

func (s *Service) runTrace(engine *task.Engine, input utils.PingDelaySet) utils.PingDelaySet {
	s.mu.RLock()
	run := s.options.ProbeHooks.RunTrace
	s.mu.RUnlock()
	if run != nil {
		return run(engine, input)
	}
	return engine.TestTraceAvailability(input)
}

func (s *Service) runDownload(engine *task.Engine, input utils.PingDelaySet) utils.DownloadSpeedSet {
	s.mu.RLock()
	run := s.options.ProbeHooks.RunDownload
	s.mu.RUnlock()
	if run != nil {
		return run(engine, input)
	}
	return engine.TestDownloadSpeed(input)
}

func (s *Service) configureStageProgress(engine *task.Engine, base task.Hooks, taskID string, info probecore.StageInfo, qualifiedTarget int) {
	hooks := base
	switch info.Stage {
	case probecore.StageTCP:
		hooks.LatencyProgress = func(processed, passed, failed, _ int) {
			s.emitProbeProgress(taskID, info.Stage, processed, passed, failed, info.Total, 0)
		}
	case probecore.StageTrace:
		hooks.TraceProgress = func(processed, passed, failed, total int) {
			s.emitProbeProgress(taskID, info.Stage, processed, passed, failed, total, 0)
		}
	case probecore.StageDownload:
		hooks.DownloadProgress = func(processed, qualified, _ int) {
			s.emitProbeProgress(taskID, info.Stage, processed, qualified, processed-qualified, info.Total, qualifiedTarget)
		}
		hooks.DownloadSpeedSample = func(sample task.DownloadSpeedSample) { s.emitProbeSpeed(taskID, sample) }
	}
	engine.SetHooks(hooks)
}

func (s *Service) beforeProbeStage(engine *task.Engine, cfg probecore.ProbeConfig, taskID string, info probecore.StageInfo) {
	engine.CheckPause(info.Stage, "")
	if info.Stage != probecore.StageDownload || info.Total > 0 {
		qualifiedTarget := 0
		if info.Stage == probecore.StageDownload {
			qualifiedTarget = cfg.DownloadSuccessLimit
		}
		s.emitProbeProgress(taskID, info.Stage, 0, 0, 0, info.Total, qualifiedTarget)
	}
	s.DebugLogger().Event("stage.start", map[string]any{"counts": map[string]any{"input": info.Input, "total": info.Total}, "message": "开始探测阶段。", "stage": info.Stage, "task_id": taskID, "config": debugProbeConfigSummary(cfg)})
}

func (s *Service) afterProbeStage(engine *task.Engine, cfg probecore.ProbeConfig, taskID string, info probecore.StageInfo) {
	s.DebugLogger().Event("stage.complete", map[string]any{"counts": debugStageCounts(info.Total, info.Passed, info.Failed), "duration_ms": info.DurationMS, "message": "探测阶段完成。", "stage": info.Stage, "task_id": taskID})
	_ = s.PublishProbeEvent(context.Background(), ProbeEvent{Event: "probe.stage_summary", TaskID: taskID, Payload: map[string]any{
		"counts":      map[string]any{"failed": info.Failed, "input": info.Input, "passed": info.Passed, "total": info.Total},
		"duration_ms": info.DurationMS,
		"reasons":     stageRejectReasonEntries(engine.TakeStageRejectCounts(info.Stage)),
		"stage":       info.Stage,
	}})
}

func effectiveServiceCaptureAddress(cfg probecore.ProbeConfig) string {
	if !cfg.Debug || !cfg.DebugCaptureEnabled || strings.TrimSpace(cfg.DebugCaptureAddress) == "" {
		return ""
	}
	return httpcfg.Resolve("", "", "", cfg.DebugCaptureAddress, true).CaptureAddress
}
