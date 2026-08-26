package appcore

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/colodict"
	"github.com/axuitomo/CFST-GUI/internal/httpcfg"
	"github.com/axuitomo/CFST-GUI/internal/probecore"
	"github.com/axuitomo/CFST-GUI/internal/sourceparse"
	"github.com/axuitomo/CFST-GUI/internal/task"
	"github.com/axuitomo/CFST-GUI/internal/utils"
)

type ProbeExecutionHooks struct {
	RunTCP      func(*task.Engine) (utils.PingDelaySet, error)
	RunTrace    func(*task.Engine, utils.PingDelaySet) utils.PingDelaySet
	RunDownload func(*task.Engine, utils.PingDelaySet) utils.DownloadSpeedSet
}

type ProbePostProcessResult struct {
	Notification *UploadNotification
	Warnings     []string
}

func (s *Service) RunProbe(payload ProbePayload) (ProbeRunResult, error) {
	payload, cfg, warnings, taskID, err := s.prepareProbePayload(payload)
	if err != nil {
		return ProbeRunResult{}, err
	}
	value, runErr := s.runProbeWithRunner(taskID, func(ctx context.Context) (any, error) {
		return s.executeProbe(ctx, payload, cfg, warnings, taskID)
	})
	if runErr != nil {
		s.publishProbeFailure(taskID, runErr, "", "")
		return ProbeRunResult{}, runErr
	}
	result, ok := value.(ProbeRunResult)
	if !ok {
		return ProbeRunResult{}, fmt.Errorf("unexpected probe result type %T", value)
	}
	return result, nil
}

func (s *Service) StartProbe(payload ProbePayload) CommandResult {
	payload, cfg, warnings, taskID, err := s.prepareProbePayload(payload)
	if err != nil {
		return NewCommandResult("PROBE_PAYLOAD_INVALID", nil, err.Error(), false, nil, warnings)
	}
	return s.startProbeWithRunner(taskID, func(ctx context.Context) (any, error) {
		return s.executeProbe(ctx, payload, cfg, warnings, taskID)
	})
}

func (s *Service) prepareProbePayload(payload ProbePayload) (ProbePayload, probecore.ProbeConfig, []string, string, error) {
	snapshot := cloneServiceMap(payload.Config)
	if len(snapshot) == 0 {
		loaded, err := s.LoadConfig()
		if err != nil {
			return payload, probecore.ProbeConfig{}, nil, "", err
		}
		snapshot = loaded.Snapshot
	}
	if len(payload.Sources) == 0 {
		payload.Sources = SourcesFromAny(snapshot["sources"])
	}
	payload.Config = snapshot
	taskID := strings.TrimSpace(payload.TaskID)
	if taskID == "" {
		taskID = fmt.Sprintf("cfst-%d", s.now().UnixNano())
	}
	payload.TaskID = taskID

	s.mu.RLock()
	options := s.options.ProbeConfigOptions
	layout := s.options.Storage
	s.mu.RUnlock()
	if strings.TrimSpace(options.DefaultExportTargetDir) == "" {
		options.DefaultExportTargetDir = layout.ExportsRoot()
	}
	options.Now = s.now()
	cfg, warnings := probecore.ConfigSnapshotToProbeConfig(snapshot, options)
	exportConfig := mapValue(snapshot["export"])
	if fileName := probecore.ExportFileName(exportConfig, taskID, options.ProfileName, s.now()); fileName != "" {
		cfg.OutputFile = probecore.ExportPath(exportConfig, fileName, options.DefaultExportTargetDir)
		cfg.WriteOutput = true
	} else if exportHasFileNameField(exportConfig) {
		cfg.OutputFile = ""
		cfg.WriteOutput = false
	}
	return payload, cfg, warnings, taskID, nil
}

func (s *Service) executeProbe(ctx context.Context, payload ProbePayload, cfg probecore.ProbeConfig, configWarnings []string, taskID string) (ProbeRunResult, error) {
	s.probeMu.Lock()
	defer s.probeMu.Unlock()

	prepared, summary, invalidCount, err := s.prepareProbeSources(ctx, payload, cfg, taskID)
	if err != nil {
		return ProbeRunResult{}, err
	}
	taskContext, portWarnings := probecore.TaskContextForPorts(cfg.TCPPort, prepared.SourcePorts, cfg.PortPolicy)
	taskContext.ConfigSource = strings.TrimSpace(payload.ConfigSource)
	prepared.Warnings = append(prepared.Warnings, portWarnings...)
	groups := probecore.PortGroups(summary.Valid, prepared.SourcePorts, cfg.TCPPort, cfg.PortPolicy)
	if cfg.PortPolicy == probecore.PortPolicySourceOverrideGlobal && len(groups) > 1 {
		prepared.Warnings = append(prepared.Warnings, fmt.Sprintf("输入源端口已按 %d 个测试端口分组执行：%v。", len(groups), probecore.PortGroupPorts(groups)))
	}

	result, err := s.runProbePortGroups(ctx, cfg, configWarnings, taskContext, prepared, summary, taskID, groups)
	if err != nil {
		if errors.Is(err, probecore.ErrProbeCanceled) {
			s.publishProbeCancelled(taskID, result.FailureStage, result.DebugLogPath)
		} else {
			s.publishProbeFailure(taskID, err, result.FailureStage, result.DebugLogPath)
		}
		return result, err
	}
	if s.probeCancelled(ctx, taskID) {
		s.publishProbeCancelled(taskID, "post_probe_push", result.DebugLogPath)
		return ProbeRunResult{}, probecore.ErrProbeCanceled
	}

	result.SourceStatuses = prepared.SourceStatuses
	result.Warnings = probecore.DedupeStrings(append(result.Warnings, prepared.Warnings...))
	s.waitProbePaused(taskID, "post_probe_push", "")
	processed := s.processPostProbePush(ctx, payload, result, func() { s.waitProbePaused(taskID, "post_probe_push", "") })
	result.Warnings = probecore.DedupeStrings(append(result.Warnings, processed.Warnings...))
	result.UploadNotification = processed.Notification
	if s.probeCancelled(ctx, taskID) {
		s.publishProbeCancelled(taskID, "persist_results", result.DebugLogPath)
		return ProbeRunResult{}, probecore.ErrProbeCanceled
	}

	s.waitProbePaused(taskID, "persist_results", "")
	if err := s.PersistCompletedProbe(taskID, result); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("保存任务结果失败：%v", err))
		s.appendProbeError("task_results.persist_failed", map[string]any{"debug_log_path": result.DebugLogPath, "message": err.Error(), "task_id": taskID})
	}
	if s.probeCancelled(ctx, taskID) {
		s.publishProbeCancelled(taskID, "persist_results", result.DebugLogPath)
		return ProbeRunResult{}, probecore.ErrProbeCanceled
	}

	for {
		s.waitProbePaused(taskID, "completed", "")
		committed, active, cancelled := s.runtime.TryCommitCompletion(taskID)
		if committed {
			break
		}
		if !active || cancelled || ctx.Err() != nil {
			s.publishProbeCancelled(taskID, "completed", result.DebugLogPath)
			return ProbeRunResult{}, probecore.ErrProbeCanceled
		}
	}
	exported := 0
	if strings.TrimSpace(result.OutputFile) != "" && len(result.Results) > 0 {
		exported = len(result.Results)
	}
	completedPayload := withProbeDebugLogPath(map[string]any{
		"exported": exported,
		"failed":   result.Summary.Failed,
		"failure_summary": map[string]any{
			"duplicate_count": summary.DuplicateCount,
			"invalid_count":   invalidCount,
		},
		"failure_stage":     result.FailureStage,
		"passed":            result.Summary.Passed,
		"result_count":      len(result.Results),
		"task_context":      result.TaskContext,
		"target_path":       result.OutputFile,
		"trace_diagnostics": result.TraceDiagnostics,
		"warnings":          result.Warnings,
	}, result.DebugLogPath)
	if uri := strings.TrimSpace(payload.AndroidExportURI); uri != "" {
		completedPayload["android_export_pending"] = strings.TrimSpace(result.OutputFile) != ""
		completedPayload["android_export_uri"] = uri
	}
	_ = s.PublishProbeEvent(context.Background(), ProbeEvent{Event: "probe.completed", TaskID: taskID, Payload: completedPayload})
	return result, nil
}

func (s *Service) prepareProbeSources(ctx context.Context, payload ProbePayload, cfg probecore.ProbeConfig, taskID string) (PreparedSources, probecore.SourceSummary, int, error) {
	started := s.now()
	s.waitProbePaused(taskID, "stage0_pool", "")
	client := s.sourceHTTPClient(cfg)
	prepared := PrepareSources(PrepareSourcesOptions{
		Context: ctx,
		Config:  cfg,
		ProcessSource: func(source Source) (SourceProcessResult, error) {
			s.waitProbePaused(taskID, "stage0_pool", "")
			return s.processProbeSource(ctx, cfg, source, client, started)
		},
		Sources: payload.Sources,
	})
	if prepared.Error != nil || s.probeCancelled(ctx, taskID) {
		s.publishProbeCancelled(taskID, "stage0_pool", "")
		return prepared, probecore.SourceSummary{}, 0, probecore.ErrProbeCanceled
	}
	if err := s.PersistSourceStatuses(prepared.SourceStatuses); err != nil {
		prepared.Warnings = append(prepared.Warnings, fmt.Sprintf("更新输入源状态失败：%v", err))
	}
	summary := probecore.SummarizeSource(prepared.Text, net.DefaultResolver)
	prepared.Text = strings.Join(summary.Valid, "\n")
	invalidCount := summary.InvalidCount + prepared.InvalidCount
	_ = s.PublishProbeEvent(context.Background(), ProbeEvent{Event: "probe.preprocessed", TaskID: taskID, Payload: map[string]any{
		"accepted": summary.ValidCount, "filtered": summary.DuplicateCount, "invalid": invalidCount,
		"source_statuses": prepared.SourceStatuses, "stage": "stage0_pool", "total": summary.ValidCount,
	}})
	if len(prepared.FatalErrors) > 0 || (strings.TrimSpace(prepared.Text) == "" && len(prepared.Warnings) > 0) {
		messages := prepared.FatalErrors
		if len(messages) == 0 {
			messages = prepared.Warnings
		}
		err := errors.New(strings.Join(messages, "；"))
		s.logProbePreparationFailure(cfg, taskID, summary, invalidCount, prepared.SourceStatuses, s.now().Sub(started), err)
		s.publishProbeFailure(taskID, err, "stage0_pool", s.probeDebugLogPath(cfg))
		return prepared, summary, invalidCount, err
	}
	return prepared, summary, invalidCount, nil
}

func (s *Service) processProbeSource(ctx context.Context, cfg probecore.ProbeConfig, source Source, client *http.Client, now time.Time) (SourceProcessResult, error) {
	return ProcessSource(source, cfg, client, now,
		func(source Source, cfg probecore.ProbeConfig, client *http.Client) (SourceContentResult, error) {
			return LoadSourceContentContext(ctx, source, cfg, client, sharedSourceContentLoadOptions())
		},
		func(raw string, source Source, cfg probecore.ProbeConfig) (probecore.SourceBuildResult, error) {
			s.mu.RLock()
			limit := s.options.ProbeConfigOptions.DefaultSourceIPLimit
			layout := s.options.Storage
			paths := s.options.ColoPaths
			resolver := s.options.SourceResolver
			s.mu.RUnlock()
			if source.IPLimit < 0 {
				limit = 0
			} else if limit <= 0 {
				limit = probecore.DefaultConfigSnapshotSourceIPLimit
			}
			if paths.Colo == "" {
				paths = colodict.DefaultPaths(layout.Root)
			}
			if resolver == nil {
				resolver = net.DefaultResolver
			}
			return BuildSourceEntriesWithConfig(SourceEntryBuildOptions{
				Context: ctx, Raw: raw, Source: source, Config: cfg, DefaultIPLimit: limit,
				Resolver: resolver, ColoDictionaryPaths: paths,
				MCISRunner: func(tokens []string, source Source, cfg probecore.ProbeConfig, limit int) ([]string, []string, error) {
					return RunMCISSearchContext(ctx, tokens, source, cfg, limit)
				},
			})
		},
	)
}

func (s *Service) runProbePortGroups(ctx context.Context, cfg probecore.ProbeConfig, configWarnings []string, taskContext probecore.TaskContext, prepared PreparedSources, summary probecore.SourceSummary, taskID string, groups []probecore.PortGroup) (ProbeRunResult, error) {
	workflow, err := probecore.RunProbeWorkflow(probecore.WorkflowRunRequest{
		Config: probecore.WorkflowConfig{DownloadSpeedMetric: cfg.DownloadSpeedMetric, PrintNum: cfg.PrintNum, TCPPort: cfg.TCPPort},
		Groups: groups, SourcePorts: prepared.SourcePorts,
		Source:      probecore.WorkflowSource{Summary: summary, Text: prepared.Text, Warnings: prepared.Warnings},
		TaskContext: taskContext, TaskID: taskID,
	}, probecore.WorkflowAdapter{
		BeginMultiGroup: func(probecore.WorkflowRunRequest) (probecore.WorkflowLifecycle, error) {
			closeLog, warnings, path := s.configureProbeDebug(cfg, taskID)
			return probecore.WorkflowLifecycle{DebugLogPath: path, StartedAt: s.now(), Warnings: warnings, Close: closeLog}, nil
		},
		Export: func(request probecore.WorkflowExportRequest) (probecore.WorkflowExportResult, error) {
			return s.exportProbeResults(ctx, cfg, taskID, request.DebugLogPath, request.RawResults)
		},
		Now: s.now,
		RunGroup: func(request probecore.WorkflowGroupRequest) (probecore.WorkflowGroupResult, error) {
			groupCfg := cfg
			if request.Group.Port > 0 {
				groupCfg.TCPPort = request.Group.Port
			}
			result, groupErr := s.runProbeGroup(ctx, probeGroupRequest{
				Config: groupCfg, ConfigWarnings: configWarnings, DisableDebugLog: request.DisableDebugLog,
				DisableExport: request.DisableExport, SourceColoFilters: prepared.SourceColoFilters,
				SourcePorts: prepared.SourcePorts, SourceStatuses: prepared.SourceStatuses,
				SourceText: request.SourceText, TaskContext: request.TaskContext, TaskID: taskID,
			})
			return probecore.WorkflowGroupResult{
				DebugLogPath: result.DebugLogPath, DurationMS: result.DurationMS, FailureStage: result.FailureStage,
				OutputFile: result.OutputFile, RawResults: result.RawResults, Results: result.Results,
				Source: result.Source, StartedAt: result.StartedAt, Summary: result.Summary,
				TaskContext: result.TaskContext, TraceDiagnostics: result.TraceDiagnostics, Warnings: result.Warnings,
			}, groupErr
		},
	})
	resultCfg := cfg
	if len(groups) == 1 && groups[0].Port > 0 {
		resultCfg.TCPPort = groups[0].Port
	}
	return ProbeRunResult{
		Config: resultCfg, DebugLogPath: workflow.DebugLogPath, DurationMS: workflow.DurationMS,
		FailureStage: workflow.FailureStage, OutputFile: workflow.OutputFile, RawResults: workflow.RawResults,
		Results: workflow.Results, SchemaVersion: CommandSchemaVersion, Source: workflow.Source,
		SourceStatuses: prepared.SourceStatuses, StartedAt: workflow.StartedAt, Summary: workflow.Summary,
		TaskID: taskID, TaskContext: workflow.TaskContext, TraceDiagnostics: workflow.TraceDiagnostics,
		Warnings: probecore.DedupeStrings(workflow.Warnings),
	}, err
}

type probeGroupRequest struct {
	Config            probecore.ProbeConfig
	ConfigWarnings    []string
	DisableDebugLog   bool
	DisableExport     bool
	SourceColoFilters task.SourceColoFilterMap
	SourcePorts       map[string]int
	SourceStatuses    []SourceStatus
	SourceText        string
	TaskContext       probecore.TaskContext
	TaskID            string
}

func (s *Service) runProbeGroup(ctx context.Context, request probeGroupRequest) (ProbeRunResult, error) {
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
	sourceText, source, err := resolveServiceProbeSource(cfg, request.SourceText)
	if err != nil || source.ValidCount == 0 {
		if err == nil {
			err = errors.New("没有可用的 IP/CIDR/域名输入")
		}
		s.logProbeFailed(request.TaskID, "stage0_pool", started, nil, err, false, map[string]any{"debug_log_path": debugLogPath})
		return ProbeRunResult{}, err
	}
	if s.probeCancelled(ctx, request.TaskID) {
		return ProbeRunResult{}, probecore.ErrProbeCanceled
	}
	cfg.IPText = strings.Join(source.Valid, ",")
	traceDiagnostics := NewTraceDiagnostics(cfg.TraceColoMode, cfg.TraceURL)
	baseHooks := task.Hooks{
		DebugEvent: s.DebugLogger().Event, TraceDiagnostic: traceDiagnostics.Record,
		ProbePause:   func(stage, ip string) { s.waitProbePaused(request.TaskID, stage, ip) },
		ProbeCancel:  func(stage, ip string) bool { return s.runtime.IsCancelRequested(request.TaskID) },
		ProbeContext: func() context.Context { return ctx },
		TraceInterrupt: func(stage, ip string, interrupt func()) func() {
			return s.runtime.RegisterInterrupt(request.TaskID, stage, interrupt)
		},
		DownloadInterrupt: func(stage, ip string, interrupt func()) func() {
			return s.runtime.RegisterInterrupt(request.TaskID, stage, interrupt)
		},
	}
	engineCfg := cfg
	if request.TaskContext.ConfigSource != "cli" {
		engineCfg.Httping = false
	}
	engine, err := NewProbeEngine(engineCfg, ProbeEngineOptions{
		CaptureAddress: effectiveServiceCaptureAddress(cfg), ColoPaths: colodict.DefaultPaths(s.StorageLayout().Root),
		OutputFile: currentServiceOutputFile(cfg), SourceColoFilter: request.SourceColoFilters, Hooks: baseHooks,
	})
	if err != nil {
		return ProbeRunResult{}, err
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
			Stage3Limit: cfg.Stage3Limit, TCPPort: cfg.TCPPort,
		},
		ConfigWarnings: warnings, DebugWarnings: debugWarnings, SourcePorts: request.SourcePorts,
		Source: source, TaskContext: request.TaskContext,
	}, probecore.StageWorkflowAdapter{
		ConfigureProgress:  func(info probecore.StageInfo) { s.configureStageProgress(engine, baseHooks, request.TaskID, info) },
		EstimateTraceTotal: engine.EstimateTraceProbeCount,
		BeforeStage: func(info probecore.StageInfo) error {
			s.beforeProbeStage(engine, cfg, request.TaskID, info)
			return nil
		},
		AfterStage: func(info probecore.StageInfo) error {
			s.afterProbeStage(cfg, request.TaskID, info)
			if s.runtime.IsCancelRequested(request.TaskID) {
				return probecore.ErrProbeCanceled
			}
			return nil
		},
		Now:         s.now,
		RunTCP:      func() (utils.PingDelaySet, error) { return s.runTCP(engine) },
		RunTrace:    func(input utils.PingDelaySet) utils.PingDelaySet { return s.runTrace(engine, input) },
		RunDownload: func(input utils.PingDelaySet) utils.DownloadSpeedSet { return s.runDownload(engine, input) },
	})
	completedStages = append(completedStages, stageResult.CompletedStages...)
	if err != nil {
		failureStage := ""
		tracePayload := traceDiagnostics.Payload()
		if !errors.Is(err, probecore.ErrProbeCanceled) && !traceDiagnostics.Empty() && stageResult.CurrentStage == probecore.StageTrace {
			failureStage = probecore.StageTrace
			rawError := err.Error()
			summary := StageTraceFailureMessage(traceDiagnostics.Summary(), rawError)
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
		return ProbeRunResult{DebugLogPath: debugLogPath, FailureStage: failureStage, TraceDiagnostics: tracePayload, Warnings: probecore.DedupeStrings(stageResult.Warnings)}, err
	}
	if s.probeCancelled(ctx, request.TaskID) {
		return ProbeRunResult{DebugLogPath: debugLogPath, Warnings: probecore.DedupeStrings(stageResult.Warnings)}, probecore.ErrProbeCanceled
	}
	outputFile := ""
	resultWarnings := append([]string(nil), stageResult.Warnings...)
	if len(stageResult.RawResults) > 0 && !request.DisableExport {
		exported, exportErr := s.exportProbeResults(ctx, cfg, request.TaskID, debugLogPath, stageResult.RawResults)
		resultWarnings = append(resultWarnings, exported.Warnings...)
		if exportErr != nil {
			return ProbeRunResult{DebugLogPath: debugLogPath, Warnings: probecore.DedupeStrings(resultWarnings)}, exportErr
		}
		outputFile = exported.OutputFile
	}
	result := ProbeRunResult{
		Config: cfg, DebugLogPath: debugLogPath, DurationMS: s.now().Sub(started).Milliseconds(),
		OutputFile: outputFile, RawResults: append([]utils.CloudflareIPData(nil), stageResult.RawResults...),
		Results: stageResult.Results, SchemaVersion: CommandSchemaVersion, Source: source,
		StartedAt: started.Format(time.RFC3339), Summary: stageResult.Summary, TaskID: request.TaskID, TaskContext: stageResult.TaskContext,
		TraceDiagnostics: traceDiagnostics.Payload(), Warnings: probecore.DedupeStrings(resultWarnings),
	}
	if ShouldMarkTraceFailureStage(stageResult.CompletedStages, traceDiagnostics, stageResult.RawResults) {
		result.FailureStage = probecore.StageTrace
	}
	s.DebugLogger().Event("probe.complete", map[string]any{
		"counts":      map[string]any{"exported": len(result.Results), "failed": result.Summary.Failed, "passed": result.Summary.Passed, "total": result.Summary.Total},
		"duration_ms": result.DurationMS, "message": "探测任务完成。", "output_file": result.OutputFile,
		"completed_stages": completedStages, "task_id": request.TaskID, "warnings": result.Warnings,
	})
	_ = sourceText
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

func (s *Service) PersistSourceStatuses(statuses []SourceStatus) error {
	if len(statuses) == 0 {
		return nil
	}
	loaded, err := s.LoadConfig()
	if err != nil || !loaded.Existed {
		return err
	}
	sources := SourcesFromAny(loaded.Snapshot["sources"])
	byID := make(map[string]SourceStatus, len(statuses))
	for _, status := range statuses {
		if id := strings.TrimSpace(status.ID); id != "" {
			byID[id] = status
		}
	}
	if len(byID) == 0 {
		return nil
	}
	for index := range sources {
		status, ok := byID[strings.TrimSpace(sources[index].ID)]
		if !ok {
			continue
		}
		sources[index].LastFetchedAt = status.LastFetchedAt
		sources[index].LastFetchedCount = status.LastFetchedCount
		sources[index].StatusText = status.StatusText
	}
	loaded.Snapshot["sources"] = sources
	_, err = s.SaveConfig(loaded.Snapshot)
	return err
}

func (s *Service) sourceHTTPClient(cfg probecore.ProbeConfig) *http.Client {
	s.mu.RLock()
	client, timeout := s.options.HTTPClient, s.options.SourceHTTPTimeout
	s.mu.RUnlock()
	if client != nil && client != http.DefaultClient {
		return client
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return NewSourceHTTPClient(cfg, SourceHTTPClientOptions{Timeout: timeout, DisableProxy: true})
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

func (s *Service) configureStageProgress(engine *task.Engine, base task.Hooks, taskID string, info probecore.StageInfo) {
	hooks := base
	switch info.Stage {
	case probecore.StageTCP:
		hooks.LatencyProgress = func(processed, passed, failed, _ int) {
			s.emitProbeProgress(taskID, info.Stage, processed, passed, failed, info.Total)
		}
	case probecore.StageTrace:
		hooks.TraceProgress = func(processed, passed, failed, total int) {
			s.emitProbeProgress(taskID, info.Stage, processed, passed, failed, total)
		}
	case probecore.StageDownload:
		hooks.DownloadProgress = func(processed, qualified, _ int) {
			s.emitProbeProgress(taskID, info.Stage, processed, qualified, processed-qualified, info.Total)
		}
		hooks.DownloadSpeedSample = func(sample task.DownloadSpeedSample) { s.emitProbeSpeed(taskID, sample) }
	}
	engine.SetHooks(hooks)
}

func (s *Service) beforeProbeStage(engine *task.Engine, cfg probecore.ProbeConfig, taskID string, info probecore.StageInfo) {
	engine.CheckPause(info.Stage, "")
	if info.Stage != probecore.StageDownload || info.Total > 0 {
		s.emitProbeProgress(taskID, info.Stage, 0, 0, 0, info.Total)
	}
	s.DebugLogger().Event("stage.start", map[string]any{"counts": map[string]any{"input": info.Input, "total": info.Total}, "message": "开始探测阶段。", "stage": info.Stage, "task_id": taskID, "config": debugProbeConfigSummary(cfg)})
}

func (s *Service) afterProbeStage(cfg probecore.ProbeConfig, taskID string, info probecore.StageInfo) {
	s.DebugLogger().Event("stage.complete", map[string]any{"counts": debugStageCounts(info.Total, info.Passed, info.Failed), "duration_ms": info.DurationMS, "message": "探测阶段完成。", "stage": info.Stage, "task_id": taskID})
}

func (s *Service) emitProbeProgress(taskID, stage string, processed, passed, failed, total int) {
	if !s.runtime.ShouldEmitProgress(stage, processed, total, s.now()) {
		return
	}
	_ = s.PublishProbeEvent(context.Background(), ProbeEvent{Event: "probe.progress", TaskID: taskID, Payload: map[string]any{"failed": failed, "passed": passed, "processed": processed, "stage": stage, "total": total}})
}

func (s *Service) emitProbeSpeed(taskID string, sample task.DownloadSpeedSample) {
	_ = s.PublishProbeEvent(context.Background(), ProbeEvent{Event: "probe.speed", TaskID: taskID, Payload: map[string]any{
		"average_speed_mb_s": sample.AverageSpeedMBs, "average_ready": sample.AverageReady, "attempt": sample.Attempt,
		"body_read": sample.BodyRead, "bytes_read": sample.BytesRead, "colo": sample.Colo,
		"current_ready": sample.CurrentReady, "current_speed_mb_s": sample.CurrentSpeedMBs,
		"elapsed_ms": sample.ElapsedMS, "ip": sample.IP, "measured_bytes": sample.MeasuredBytes,
		"measured_elapsed_ms": sample.MeasuredElapsedMS, "sample_bytes": sample.SampleBytes,
		"sample_elapsed_ms": sample.SampleElapsedMS, "stage": sample.Stage, "transfer_complete": sample.TransferComplete,
	}})
}

func (s *Service) waitProbePaused(taskID, stage, ip string) {
	s.runtime.WaitWhilePaused(taskID, func() {
		_ = s.PublishProbeEvent(context.Background(), ProbeEvent{Event: "probe.cooling", TaskID: taskID, Payload: map[string]any{"ip": ip, "message": "测速任务已暂停。", "reason": "pause_requested", "recoverable": true, "stage": stage}})
	})
}

func (s *Service) probeCancelled(ctx context.Context, taskID string) bool {
	return ctx.Err() != nil || s.runtime.IsCancelRequested(taskID)
}

func (s *Service) publishProbeCancelled(taskID, stage, debugLogPath string) {
	_ = s.PublishProbeEvent(context.Background(), ProbeEvent{Event: "probe.cancelled", TaskID: taskID, Payload: withProbeDebugLogPath(map[string]any{"message": "测速任务已终止。", "stage": stage}, debugLogPath)})
}

func (s *Service) publishProbeFailure(taskID string, err error, stage, debugLogPath string) {
	if err == nil {
		return
	}
	payload := withProbeDebugLogPath(map[string]any{"message": err.Error(), "recoverable": false}, debugLogPath)
	if strings.TrimSpace(stage) != "" {
		payload["failure_stage"] = stage
		payload["stage"] = stage
	}
	_ = s.PublishProbeEvent(context.Background(), ProbeEvent{Event: "probe.failed", TaskID: taskID, Payload: payload})
}

func (s *Service) configureProbeDebug(cfg probecore.ProbeConfig, taskID string) (func(), []string, string) {
	path, err := s.DebugLogger().Configure(cfg.Debug, s.probeDebugLogPath(cfg), cfg.DebugLogMode, cfg.DebugLogFormat, cfg.DebugLogVerbosity)
	if err != nil {
		return func() {}, []string{fmt.Sprintf("初始化调试日志失败：%v", err)}, ""
	}
	s.DebugLogger().SetContext(taskID)
	warnings := []string{}
	if cfg.Debug && path != "" {
		warnings = append(warnings, fmt.Sprintf("调试日志已写入 %s", path))
	}
	if capture := effectiveServiceCaptureAddress(cfg); capture != "" {
		warnings = append(warnings, fmt.Sprintf("调试模式已将请求拨号目标覆盖为 %s", capture))
	}
	return func() { _ = s.DebugLogger().Close() }, warnings, path
}

func (s *Service) probeDebugLogPath(cfg probecore.ProbeConfig) string {
	if !cfg.Debug {
		return ""
	}
	return filepath.Join(s.StorageLayout().LogsRoot(), "cfip-log.txt")
}

func (s *Service) appendProbeError(event string, fields map[string]any) {
	_ = utils.AppendErrorLog(filepath.Join(s.StorageLayout().LogsRoot(), "error-log.txt"), event, fields)
}

func (s *Service) logProbePreparationFailure(cfg probecore.ProbeConfig, taskID string, source probecore.SourceSummary, invalidCount int, statuses []SourceStatus, duration time.Duration, err error) {
	s.DebugLogger().SetEnabled(cfg.Debug)
	closeLog, _, _ := s.configureProbeDebug(cfg, taskID)
	defer closeLog()
	s.debugProbeStart(cfg, taskID, statuses)
	s.DebugLogger().Event("stage.complete", map[string]any{"counts": debugStage0Counts(source, invalidCount), "duration_ms": duration.Milliseconds(), "message": "IP 池生成失败。", "source": debugSourceSummary(source, statuses), "stage": "stage0_pool", "task_id": taskID})
	s.logProbeFailed(taskID, "stage0_pool", s.now().Add(-duration), nil, err, false, map[string]any{"debug_log_path": s.probeDebugLogPath(cfg)})
}

func (s *Service) debugProbeStart(cfg probecore.ProbeConfig, taskID string, statuses []SourceStatus) {
	s.DebugLogger().SetEnabled(cfg.Debug)
	s.DebugLogger().Event("probe.start", map[string]any{"config": debugProbeConfigSummary(cfg), "message": "探测任务启动。", "source": map[string]any{"status": "pending", "source_statuses": statuses}, "task_id": taskID})
	s.DebugLogger().Event("stage.start", map[string]any{"message": "开始生成 IP 池。", "stage": "stage0_pool", "task_id": taskID})
}

func (s *Service) logProbeFailed(taskID, stage string, started time.Time, completed []string, err error, recoverable bool, extras map[string]any) {
	message, errText := "探测任务失败。", ""
	if err != nil {
		message, errText = err.Error(), err.Error()
	}
	fields := map[string]any{"completed_stages": completed, "duration_ms": s.now().Sub(started).Milliseconds(), "error": errText, "message": message, "recoverable": recoverable, "stage": stage, "task_id": taskID}
	for key, value := range extras {
		fields[key] = value
	}
	s.DebugLogger().Event("probe.failed", fields)
	s.appendProbeError("probe.failed", fields)
}

func resolveServiceProbeSource(cfg probecore.ProbeConfig, raw string) (string, probecore.SourceSummary, error) {
	text := strings.TrimSpace(raw)
	if text == "" {
		text = strings.TrimSpace(cfg.IPText)
	}
	if text == "" {
		loaded, err := loadLocalSourceFile(context.Background(), cfg.IPFile, MaxSourceContentBytes)
		if err != nil {
			return "", probecore.SourceSummary{}, fmt.Errorf("读取 IP 数据文件失败：%w", err)
		}
		text = loaded.Raw
	}
	return text, probecore.SummarizeSource(text, net.DefaultResolver), nil
}

func currentServiceOutputFile(cfg probecore.ProbeConfig) string {
	if !cfg.WriteOutput {
		return ""
	}
	return strings.TrimSpace(cfg.OutputFile)
}

func effectiveServiceCaptureAddress(cfg probecore.ProbeConfig) string {
	if !cfg.Debug || !cfg.DebugCaptureEnabled || strings.TrimSpace(cfg.DebugCaptureAddress) == "" {
		return ""
	}
	return httpcfg.Resolve("", "", "", cfg.DebugCaptureAddress, true).CaptureAddress
}

func withProbeDebugLogPath(payload map[string]any, path string) map[string]any {
	if strings.TrimSpace(path) != "" {
		payload["debug_log_path"] = strings.TrimSpace(path)
	}
	return payload
}

func debugStageCounts(total, passed, failed int) map[string]any {
	if failed < 0 {
		failed = 0
	}
	return map[string]any{"failed": failed, "passed": passed, "total": total}
}

func debugStage0Counts(source probecore.SourceSummary, invalid int) map[string]any {
	total := source.CandidateCount
	if total == 0 {
		total = source.ValidCount + source.DuplicateCount + invalid
	}
	return map[string]any{"accepted": source.ValidCount, "filtered": source.DuplicateCount, "invalid": invalid, "total": total}
}

func debugSourceSummary(source probecore.SourceSummary, statuses []SourceStatus) map[string]any {
	return map[string]any{"candidate_count": source.CandidateCount, "duplicate_count": source.DuplicateCount, "invalid_count": source.InvalidCount, "raw_line_count": source.RawLineCount, "source_statuses": statuses, "unique_count": source.UniqueCount, "valid_count": source.ValidCount}
}

func debugProbeConfigSummary(cfg probecore.ProbeConfig) map[string]any {
	return map[string]any{
		"debug_capture_address": cfg.DebugCaptureAddress, "debug_capture_enabled": cfg.DebugCaptureEnabled,
		"debug_log_mode": cfg.DebugLogMode, "debug_log_verbosity": cfg.DebugLogVerbosity,
		"disable_download": cfg.DisableDownload, "download_buffer_kb": cfg.DownloadBufferKB,
		"download_count": cfg.TestCount, "download_concurrency": cfg.Stage3Concurrency,
		"download_get_concurrency": cfg.DownloadGetConcurrency, "download_http_protocol": cfg.DownloadHTTPProtocol,
		"download_speed_metric": cfg.DownloadSpeedMetric, "download_speed_sample_interval_ms": cfg.DownloadSpeedSampleIntervalMS,
		"download_time_seconds_per_ip": cfg.DownloadTimeSeconds, "download_warmup_seconds": cfg.DownloadWarmupSeconds,
		"event_throttle_ms": cfg.EventThrottleMS, "max_loss_rate": cfg.MaxLossRate,
		"max_tcp_latency_ms": cfg.MaxDelayMS, "min_delay_ms": cfg.MinDelayMS, "min_download_mbps": cfg.MinSpeedMB,
		"ping_times": cfg.PingTimes, "retry_backoff_ms": cfg.RetryBackoffMS, "retry_max_attempts": cfg.RetryMaxAttempts,
		"routines": cfg.Routines, "skip_first_latency_sample": cfg.SkipFirstLatency, "stage3_limit": cfg.Stage3Limit,
		"strategy": cfg.Strategy, "tcp_port": cfg.TCPPort, "trace_colo_mode": cfg.TraceColoMode,
		"trace_url": cfg.TraceURL, "source_colo_filter_phase": cfg.SourceColoFilterPhase,
		"url": cfg.URL, "user_agent": cfg.UserAgent, "write_output": cfg.WriteOutput,
	}
}

func sharedSourceContentLoadOptions() SourceContentLoadOptions {
	return SourceContentLoadOptions{
		BuildAttempts: func(primary string, source Source) []RemoteSourceAttempt {
			if cdn, ok := githubRawToJSDelivrURL(primary); ok && cdn != primary {
				return []RemoteSourceAttempt{{URL: primary}, {URL: cdn}}
			}
			return []RemoteSourceAttempt{{URL: primary}}
		},
		ShouldRetry: func(status int, err error) bool {
			return err != nil && (status == 0 || status == http.StatusTooManyRequests || status >= 500)
		},
		OnFallbackSuccess: func(primary string, used RemoteSourceAttempt, source Source) []string {
			if used.URL == primary {
				return nil
			}
			return []string{fmt.Sprintf("输入源 %s 已通过 jsDelivr CDN 兜底读取。", SourceName(source))}
		},
	}
}

func githubRawToJSDelivrURL(raw string) (string, bool) {
	parsed, err := url.Parse(raw)
	if err != nil || !strings.EqualFold(parsed.Host, "raw.githubusercontent.com") {
		return "", false
	}
	segments := splitURLPath(parsed.Path)
	if len(segments) < 4 {
		return "", false
	}
	branchIndex := 2
	if len(segments) >= 6 && segments[2] == "refs" && segments[3] == "heads" {
		branchIndex = 4
	}
	owner, repo, branch := segments[0], segments[1], segments[branchIndex]
	files := segments[branchIndex+1:]
	if owner == "" || repo == "" || branch == "" || len(files) == 0 {
		return "", false
	}
	return (&url.URL{Scheme: "https", Host: "cdn.jsdelivr.net", Path: "/gh/" + strings.Join(append([]string{owner, repo + "@" + branch}, files...), "/")}).String(), true
}

func splitURLPath(value string) []string {
	parts := strings.Split(strings.Trim(value, "/"), "/")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

func exportHasFileNameField(exportConfig map[string]any) bool {
	if len(exportConfig) == 0 {
		return false
	}
	for _, key := range []string{"file_name", "fileName", "file_name_template", "fileNameTemplate"} {
		if _, ok := exportConfig[key]; ok {
			return true
		}
	}
	return false
}

func cloneServiceMap(input map[string]any) map[string]any {
	if input == nil {
		return map[string]any{}
	}
	result := make(map[string]any, len(input))
	for key, value := range input {
		result[key] = value
	}
	return result
}

var _ sourceparse.Resolver = net.DefaultResolver
