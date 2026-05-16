package mobileapi

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/appcore"
	"github.com/axuitomo/CFST-GUI/internal/httpcfg"
	"github.com/axuitomo/CFST-GUI/internal/probecore"
	"github.com/axuitomo/CFST-GUI/task"
	"github.com/axuitomo/CFST-GUI/utils"
)

type probeEventEnvelope struct {
	Event         string         `json:"event"`
	Payload       map[string]any `json:"payload"`
	SchemaVersion string         `json:"schema_version"`
	Seq           int            `json:"seq"`
	TaskID        string         `json:"task_id"`
	TS            string         `json:"ts"`
}

const probeAlreadyRunningMessage = "当前已有探测任务运行或暂停，请完成后再启动新任务。"

var (
	mobileTCPProbeRunner = func() utils.PingDelaySet {
		return task.NewPing().Run().FilterDelay().FilterLossRate()
	}
	mobileTraceProbeRunner    = task.TestTraceAvailability
	mobileDownloadProbeRunner = task.TestDownloadSpeed
)

func (s *Service) RunProbe(payloadJSON string) string {
	var payload desktopProbePayload
	if err := decodeInto(payloadJSON, &payload); err != nil {
		return encodeCommand(commandResultFor("PROBE_PAYLOAD_INVALID", nil, err.Error(), false, nil, nil))
	}
	cfg, configWarnings := configToProbeConfig(payload.Config)
	taskID := strings.TrimSpace(payload.TaskID)
	if taskID == "" {
		taskID = fmt.Sprintf("cfst-mobile-%d", time.Now().UnixNano())
	}
	startedAt := time.Now()
	cfg = s.applyExportConfig(cfg, payload.Config, taskID)
	if ok, _ := s.setCurrentTask(taskID); !ok {
		return encodeCommand(commandResultFor("PROBE_ALREADY_RUNNING", nil, probeAlreadyRunningMessage, false, &taskID, nil))
	}
	defer s.clearCurrentTask(taskID)

	prepared := s.prepareSources(cfg, payload.Sources)
	preparedSummary := summarizeSource(prepared.Text)
	prepared.Text = strings.Join(preparedSummary.Valid, "\n")
	preparedInvalidCount := preparedSummary.InvalidCount + prepared.InvalidCount
	s.emit(taskID, "probe.preprocessed", map[string]any{
		"accepted":        preparedSummary.ValidCount,
		"filtered":        preparedSummary.DuplicateCount,
		"invalid":         preparedInvalidCount,
		"source_statuses": prepared.SourceStatuses,
		"stage":           "stage0_pool",
		"total":           preparedSummary.ValidCount,
	})
	if len(prepared.FatalErrors) > 0 {
		err := errors.New(strings.Join(prepared.FatalErrors, "；"))
		s.logProbePreparationFailure(cfg, taskID, preparedSummary, preparedInvalidCount, prepared.SourceStatuses, time.Since(startedAt), err)
		s.emit(taskID, "probe.failed", s.withDebugLogPath(map[string]any{"message": err.Error(), "recoverable": false}, s.debugLogPathForProbeConfig(cfg)))
		return encodeCommand(commandResultFor("PROBE_FAILED", nil, err.Error(), false, &taskID, prepared.Warnings))
	}
	if strings.TrimSpace(prepared.Text) == "" && len(prepared.Warnings) > 0 {
		err := errors.New(strings.Join(prepared.Warnings, "；"))
		s.logProbePreparationFailure(cfg, taskID, preparedSummary, preparedInvalidCount, prepared.SourceStatuses, time.Since(startedAt), err)
		s.emit(taskID, "probe.failed", s.withDebugLogPath(map[string]any{"message": err.Error(), "recoverable": false}, s.debugLogPathForProbeConfig(cfg)))
		return encodeCommand(commandResultFor("PROBE_FAILED", nil, err.Error(), false, &taskID, prepared.Warnings))
	}

	taskContext, portWarnings := probecore.TaskContextForPorts(cfg.TCPPort, prepared.SourcePorts, cfg.PortPolicy)
	taskContext.ConfigSource = strings.TrimSpace(payload.ConfigSource)
	prepared.Warnings = append(prepared.Warnings, portWarnings...)
	portGroups := probecore.PortGroups(preparedSummary.Valid, prepared.SourcePorts, cfg.TCPPort, cfg.PortPolicy)
	if cfg.PortPolicy == probecore.PortPolicySourceOverrideGlobal && len(portGroups) > 1 {
		prepared.Warnings = append(prepared.Warnings, fmt.Sprintf("输入源端口已按 %d 个测试端口分组执行：%v。", len(portGroups), probecore.PortGroupPorts(portGroups)))
	}

	result, err := s.runProbePortGroups(taskID, cfg, configWarnings, taskContext, prepared, preparedSummary, portGroups)
	if err != nil {
		debugLogPath := result.DebugLogPath
		if debugLogPath == "" {
			debugLogPath = s.debugLogPathForProbeConfig(cfg)
		}
		s.emit(taskID, "probe.failed", s.withDebugLogPath(map[string]any{"message": err.Error(), "recoverable": false}, debugLogPath))
		return encodeCommand(commandResultFor("PROBE_FAILED", nil, err.Error(), false, &taskID, result.Warnings))
	}
	result.SourceStatuses = prepared.SourceStatuses
	result.Warnings = dedupeStrings(append(result.Warnings, prepared.Warnings...))
	s.emitProbeCompleted(taskID, result, preparedSummary, preparedInvalidCount, payload.AndroidExportURI)
	return encodeCommand(commandResultFor("PROBE_COMPLETED", result, "移动端 CFST 探测已完成。", true, &taskID, result.Warnings))
}

func (s *Service) emitProbeCompleted(taskID string, result probeRunResult, preparedSummary sourceSummary, preparedInvalidCount int, androidExportURI string) {
	exportedCount := 0
	if strings.TrimSpace(result.OutputFile) != "" && len(result.Results) > 0 {
		exportedCount = len(result.Results)
	}
	eventOutputFile := result.OutputFile
	if strings.TrimSpace(androidExportURI) != "" && eventOutputFile != "" {
		eventOutputFile = strings.TrimSpace(androidExportURI)
	}
	s.emit(taskID, "probe.completed", s.withDebugLogPath(map[string]any{
		"exported": exportedCount,
		"failed":   result.Summary.Failed,
		"failure_summary": map[string]any{
			"duplicate_count": preparedSummary.DuplicateCount,
			"invalid_count":   preparedInvalidCount,
		},
		"passed":       result.Summary.Passed,
		"result_count": len(result.Results),
		"task_context": result.TaskContext,
		"target_path":  eventOutputFile,
	}, result.DebugLogPath))
}

func (s *Service) CancelProbe(payloadJSON string) string {
	payload, _ := decodeObject(payloadJSON)
	taskID := strings.TrimSpace(stringValue(firstNonNil(payload["task_id"], payload["taskId"]), ""))
	mode := strings.ToLower(strings.TrimSpace(stringValue(payload["mode"], "cancel")))
	if mode == "pause" {
		s.stateMu.Lock()
		if taskID == "" {
			taskID = s.currentTaskID
		}
		if taskID == "" || taskID != s.currentTaskID {
			s.stateMu.Unlock()
			return encodeCommand(commandResultFor("PROBE_PAUSE_UNAVAILABLE", nil, "当前没有可暂停的移动端探测任务。", false, &taskID, nil))
		}
		s.pauseRequested = true
		s.pausedTaskID = taskID
		downloadCancel := s.downloadCancel
		if s.pauseCond != nil {
			s.pauseCond.Broadcast()
		}
		s.stateMu.Unlock()
		if downloadCancel != nil {
			downloadCancel()
		}
		s.emit(taskID, "probe.cooling", map[string]any{
			"reason":      "已收到暂停请求，正在暂停当前测速进程。",
			"recoverable": true,
		})
		return encodeCommand(commandResultFor("PROBE_PAUSE_REQUESTED", nil, "已请求暂停移动端探测任务。", true, &taskID, nil))
	}

	s.stateMu.Lock()
	if taskID == "" {
		taskID = s.currentTaskID
	}
	if taskID != "" {
		s.cancelTaskID = taskID
		s.cancelRequested = true
		s.pauseRequested = false
		s.pausedTaskID = ""
		if s.pauseCond != nil {
			s.pauseCond.Broadcast()
		}
	}
	s.stateMu.Unlock()
	if taskID != "" {
		s.emit(taskID, "probe.cooling", map[string]any{
			"reason":      "已收到取消请求，任务将在当前安全点停止。",
			"recoverable": false,
		})
	}
	return encodeCommand(commandResultFor("PROBE_STOP_REQUESTED", nil, "已请求取消移动端探测任务。", true, &taskID, nil))
}

func (s *Service) ResumeProbe(payloadJSON string) string {
	payload, _ := decodeObject(payloadJSON)
	taskID := strings.TrimSpace(stringValue(firstNonNil(payload["task_id"], payload["taskId"]), ""))

	s.stateMu.Lock()
	if taskID == "" {
		taskID = s.pausedTaskID
	}
	if taskID == "" || taskID != s.pausedTaskID || !s.pauseRequested {
		s.stateMu.Unlock()
		return encodeCommand(commandResultFor("PROBE_RESUME_UNAVAILABLE", nil, "当前没有可继续的移动端探测任务。", false, &taskID, nil))
	}
	s.pauseRequested = false
	s.pausedTaskID = ""
	if s.pauseCond != nil {
		s.pauseCond.Broadcast()
	}
	s.stateMu.Unlock()

	return encodeCommand(commandResultFor("PROBE_RESUME_REQUESTED", nil, "已请求继续移动端探测任务。", true, &taskID, nil))
}

func (s *Service) OpenPath(targetPath string) string {
	_ = targetPath
	return encodeCommand(commandResultFor("OPEN_PATH_UNSUPPORTED", nil, "Android 端暂不直接打开私有导出路径。", true, nil, []string{"如需共享导出文件，后续应接入 Android Storage Access Framework。"}))
}

func (s *Service) ListResultFile(payloadJSON string) string {
	payload, err := decodeObject(payloadJSON)
	if err != nil {
		return encodeCommand(commandResultFor("RESULT_FILE_PAYLOAD_INVALID", nil, err.Error(), false, nil, nil))
	}
	config := mapValue(firstNonNil(payload["config"], payload["config_snapshot"], payload["configSnapshot"]))
	cfg, _ := configToProbeConfig(config)
	taskID := strings.TrimSpace(stringValue(firstNonNil(payload["task_id"], payload["taskId"]), ""))
	sourcePath := s.resolveResultFilePath(payload, cfg)
	rows, err := readMobileProbeResultRowsFromCSV(sourcePath)
	if err != nil {
		return encodeCommand(commandResultFor("RESULT_FILE_UNAVAILABLE", nil, err.Error(), false, &taskID, nil))
	}
	return encodeCommand(commandResultFor("RESULT_FILE_LISTED", map[string]any{
		"count":       len(rows),
		"results":     rows,
		"source_path": sourcePath,
	}, "已从结果文件读取当前结果。", true, &taskID, nil))
}

func (s *Service) runProbePortGroups(taskID string, cfg probeConfig, configWarnings []string, taskContext probeTaskContext, prepared preparedSources, preparedSummary sourceSummary, groups []probecore.PortGroup) (probeRunResult, error) {
	workflowResult, err := probecore.RunProbeWorkflow(probecore.WorkflowRunRequest{
		Config: probecore.WorkflowConfig{
			DownloadSpeedMetric: cfg.DownloadSpeedMetric,
			PrintNum:            cfg.PrintNum,
			TCPPort:             cfg.TCPPort,
		},
		Groups:      groups,
		SourcePorts: prepared.SourcePorts,
		Source: probecore.WorkflowSource{
			Summary:  preparedSummary,
			Text:     prepared.Text,
			Warnings: prepared.Warnings,
		},
		TaskContext: taskContext,
		TaskID:      taskID,
	}, probecore.WorkflowAdapter{
		BeginMultiGroup: func(probecore.WorkflowRunRequest) (probecore.WorkflowLifecycle, error) {
			return probecore.WorkflowLifecycle{
				DebugLogPath: s.debugLogPathForProbeConfig(cfg),
				StartedAt:    time.Now(),
			}, nil
		},
		Export: func(req probecore.WorkflowExportRequest) (probecore.WorkflowExportResult, error) {
			outputFile := s.exportPath(cfg.OutputFile)
			if outputFile == "" {
				return probecore.WorkflowExportResult{}, nil
			}
			exportCfg := cfg
			exportCfg.OutputFile = outputFile
			s.applyProbeConfig(exportCfg)
			if mkdirErr := os.MkdirAll(filepath.Dir(outputFile), 0o755); mkdirErr != nil {
				return probecore.WorkflowExportResult{
					Warnings: []string{fmt.Sprintf("创建导出目录失败：%v", mkdirErr)},
				}, nil
			}
			if exportErr := utils.ExportCsv(req.RawResults); exportErr != nil {
				return probecore.WorkflowExportResult{
					Warnings: []string{fmt.Sprintf("结果导出失败：%v", exportErr)},
				}, nil
			}
			s.emit(taskID, "probe.partial_export", s.withDebugLogPath(map[string]any{
				"target_path": outputFile,
				"written":     len(req.RawResults),
			}, req.DebugLogPath))
			return probecore.WorkflowExportResult{OutputFile: outputFile}, nil
		},
		RunGroup: func(req probecore.WorkflowGroupRequest) (probecore.WorkflowGroupResult, error) {
			groupCfg := cfg
			if req.Group.Port > 0 {
				groupCfg.TCPPort = req.Group.Port
			}
			groupResult, groupErr := s.runProbe(taskID, groupCfg, configWarnings, req.TaskContext, req.SourceText, prepared.SourceStatuses, prepared.SourceColoFilters, prepared.SourcePorts, req.DisableExport, req.DisableDebugLog)
			return probecore.WorkflowGroupResult{
				DebugLogPath: groupResult.DebugLogPath,
				DurationMS:   groupResult.DurationMS,
				OutputFile:   groupResult.OutputFile,
				RawResults:   groupResult.RawResults,
				Results:      groupResult.Results,
				Source:       groupResult.Source,
				StartedAt:    groupResult.StartedAt,
				Summary:      groupResult.Summary,
				TaskContext:  groupResult.TaskContext,
				Warnings:     groupResult.Warnings,
			}, groupErr
		},
	})
	resultCfg := cfg
	if len(groups) == 1 && groups[0].Port > 0 {
		resultCfg.TCPPort = groups[0].Port
	}
	return probeRunResult{
		Config:         resultCfg,
		DebugLogPath:   workflowResult.DebugLogPath,
		DurationMS:     workflowResult.DurationMS,
		OutputFile:     workflowResult.OutputFile,
		Results:        workflowResult.Results,
		Source:         workflowResult.Source,
		SourceStatuses: prepared.SourceStatuses,
		StartedAt:      workflowResult.StartedAt,
		Summary:        workflowResult.Summary,
		TaskContext:    workflowResult.TaskContext,
		Warnings:       dedupeStrings(workflowResult.Warnings),
		SchemaVersion:  schemaVersion,
		RawResults:     workflowResult.RawResults,
	}, err
}

func (s *Service) runProbe(taskID string, cfg probeConfig, configWarnings []string, taskContext probeTaskContext, sourceText string, sourceStatuses []desktopSourceStatus, sourceColoFilters task.SourceColoFilterMap, sourcePorts map[string]int, disableExport bool, disableDebugLog bool) (probeRunResult, error) {
	s.runMu.Lock()
	defer s.runMu.Unlock()

	start := time.Now()
	cfg, normalizeWarnings := normalizeProbeConfig(cfg)
	s.configureProgressThrottle(time.Duration(cfg.EventThrottleMS) * time.Millisecond)
	cfg.OutputFile = s.exportPath(cfg.OutputFile)
	configWarnings = append(configWarnings, normalizeWarnings...)
	utils.Debug = cfg.Debug
	closeDebugLog := func() {}
	debugWarnings := []string{}
	debugLogPath := s.debugLogPathForProbeConfig(cfg)
	if !disableDebugLog {
		closeDebugLog, debugWarnings, debugLogPath = s.configureProbeDebugRuntime(cfg)
		utils.SetDebugLogContext(taskID)
	}
	defer closeDebugLog()

	utils.DebugEvent("probe.start", map[string]any{
		"config":  mobileDebugProbeConfigSummary(cfg),
		"message": "移动端探测任务启动。",
		"source": map[string]any{
			"status":          "pending",
			"source_statuses": sourceStatuses,
		},
		"task_id": taskID,
	})

	completedStages := make([]string, 0, 4)
	currentStage := "stage0_pool"
	stageStart := time.Now()
	utils.DebugEvent("stage.start", map[string]any{
		"message": "开始生成 IP 池。",
		"stage":   currentStage,
		"task_id": taskID,
	})
	_, source, err := resolveProbeSource(cfg, sourceText)
	if err != nil {
		mobileLogProbeFailed(taskID, currentStage, start, completedStages, err, false)
		return probeRunResult{Warnings: configWarnings}, err
	}
	if source.ValidCount == 0 {
		err := errors.New("没有可用的 IP/CIDR/域名输入")
		mobileLogProbeFailed(taskID, currentStage, start, completedStages, err, false)
		return probeRunResult{Warnings: configWarnings}, err
	}
	if s.isCancelRequested(taskID) {
		err := errors.New("任务已取消")
		mobileLogProbeFailed(taskID, currentStage, start, completedStages, err, false)
		return probeRunResult{Warnings: configWarnings}, err
	}

	cfg.IPText = strings.Join(source.Valid, ",")
	s.applyProbeConfig(cfg)
	task.SourceColoFilters = task.CloneSourceColoFilterMap(sourceColoFilters)
	task.InitRandSeed()
	utils.DebugEvent("stage.complete", map[string]any{
		"counts":      mobileDebugStage0Counts(source, source.InvalidCount),
		"duration_ms": time.Since(stageStart).Milliseconds(),
		"message":     "IP 池生成完成。",
		"source":      mobileDebugSourceSummary(source, sourceStatuses),
		"stage":       currentStage,
		"task_id":     taskID,
	})
	completedStages = append(completedStages, currentStage)

	task.HeadProgressHook = nil
	task.LatencyProgressHook = nil
	task.TraceProgressHook = nil
	task.DownloadProgressHook = nil
	task.DownloadSpeedSampleHook = nil
	task.DownloadInterruptHook = nil
	task.ProbePauseHook = nil
	defer func() {
		task.LatencyProgressHook = nil
		task.HeadProgressHook = nil
		task.TraceProgressHook = nil
		task.DownloadProgressHook = nil
		task.DownloadSpeedSampleHook = nil
		task.DownloadInterruptHook = nil
		task.ProbePauseHook = nil
	}()
	task.ProbePauseHook = func(stage, ip string) {
		s.waitIfProbePaused(taskID, stage, ip)
	}
	task.DownloadInterruptHook = func(stage, ip string, interrupt func()) func() {
		return s.registerDownloadInterrupt(taskID, stage, ip, interrupt)
	}

	stageErrorLogged := false
	stageResult, err := probecore.RunProbeStages(probecore.StageWorkflowRequest{
		Config: probecore.StageWorkflowConfig{
			DisableDownload:     cfg.DisableDownload,
			DisableResultLimit:  disableExport,
			DownloadSpeedMetric: cfg.DownloadSpeedMetric,
			PrintNum:            cfg.PrintNum,
			Stage3Limit:         cfg.Stage3Limit,
			TCPPort:             cfg.TCPPort,
		},
		ConfigWarnings: configWarnings,
		DebugWarnings:  debugWarnings,
		SourcePorts:    sourcePorts,
		Source:         source,
		TaskContext:    taskContext,
	}, probecore.StageWorkflowAdapter{
		ConfigureProgress: func(info probecore.StageInfo) {
			s.configureMobileStageProgress(taskID, info)
		},
		BeforeStage: func(info probecore.StageInfo) error {
			s.beforeMobileStage(cfg, taskID, info)
			return nil
		},
		AfterStage: func(info probecore.StageInfo) error {
			s.afterMobileStage(cfg, taskID, info)
			if s.isCancelRequested(taskID) {
				err := errors.New("任务已取消")
				loggedStages := append(append([]string(nil), completedStages...), info.Stage)
				mobileLogProbeFailed(taskID, info.Stage, start, loggedStages, err, false)
				stageErrorLogged = true
				return err
			}
			return nil
		},
		RunTCP: func() utils.PingDelaySet {
			task.Httping = false
			return mobileTCPProbeRunner()
		},
		RunTrace:    mobileTraceProbeRunner,
		RunDownload: mobileDownloadProbeRunner,
	})
	completedStages = append(completedStages, stageResult.CompletedStages...)
	if err != nil {
		if !stageErrorLogged {
			mobileLogProbeFailed(taskID, stageResult.CurrentStage, start, completedStages, err, false)
		}
		return probeRunResult{DebugLogPath: debugLogPath, Warnings: dedupeStrings(stageResult.Warnings)}, err
	}

	resultData := stageResult.RawResults
	warnings := append([]string(nil), stageResult.Warnings...)

	outputFile := ""
	if len(resultData) > 0 && !disableExport {
		outputFile = cfg.OutputFile
		if outputFile != "" {
			if err := os.MkdirAll(filepath.Dir(outputFile), 0o755); err != nil {
				warnings = append(warnings, fmt.Sprintf("创建导出目录失败：%v", err))
				utils.DebugEvent("probe.export", map[string]any{
					"error":       err.Error(),
					"level":       "warn",
					"message":     "创建导出目录失败。",
					"reason":      "mkdir_failed",
					"target_path": outputFile,
					"task_id":     taskID,
				})
				outputFile = ""
			} else if err := utils.ExportCsv(resultData); err != nil {
				warnings = append(warnings, fmt.Sprintf("结果导出失败：%v", err))
				utils.DebugEvent("probe.export", map[string]any{
					"error":       err.Error(),
					"level":       "warn",
					"message":     "CSV 导出失败。",
					"reason":      "csv_export_failed",
					"target_path": outputFile,
					"task_id":     taskID,
				})
				outputFile = ""
			} else {
				s.emit(taskID, "probe.partial_export", s.withDebugLogPath(map[string]any{
					"target_path": outputFile,
					"written":     len(resultData),
				}, debugLogPath))
				utils.DebugEvent("probe.export", map[string]any{
					"counts": map[string]any{
						"written": len(resultData),
					},
					"message":     "CSV 导出完成。",
					"target_path": outputFile,
					"task_id":     taskID,
					"tcp":         map[string]any{"delay_column": "TCP延迟(ms)"},
				})
			}
		}
	}

	result := probeRunResult{
		Config:         cfg,
		DebugLogPath:   debugLogPath,
		DurationMS:     time.Since(start).Milliseconds(),
		OutputFile:     outputFile,
		Results:        stageResult.Results,
		Source:         source,
		SourceStatuses: sourceStatuses,
		StartedAt:      start.Format(time.RFC3339),
		Summary:        stageResult.Summary,
		TaskContext:    stageResult.TaskContext,
		Warnings:       dedupeStrings(warnings),
		SchemaVersion:  schemaVersion,
		RawResults:     append([]utils.CloudflareIPData(nil), resultData...),
	}
	utils.DebugEvent("probe.complete", map[string]any{
		"counts": map[string]any{
			"exported": len(result.Results),
			"failed":   result.Summary.Failed,
			"passed":   result.Summary.Passed,
			"total":    result.Summary.Total,
		},
		"duration_ms":      result.DurationMS,
		"message":          "移动端探测任务完成。",
		"output_file":      result.OutputFile,
		"completed_stages": completedStages,
		"task_id":          taskID,
		"warnings":         result.Warnings,
	})
	return result, nil
}

func (s *Service) configureMobileStageProgress(taskID string, info probecore.StageInfo) {
	switch info.Stage {
	case probecore.StageTCP:
		task.LatencyProgressHook = func(processed, passed, failed, _ int) {
			s.emitProgress(taskID, probecore.StageTCP, processed, passed, failed, info.Total)
		}
	case probecore.StageTrace:
		task.TraceProgressHook = func(processed, passed, failed, total int) {
			s.emitProgress(taskID, probecore.StageTrace, processed, passed, failed, total)
		}
	case probecore.StageDownload:
		if info.Total <= 0 {
			return
		}
		task.DownloadProgressHook = func(processed, qualified, _ int) {
			s.emitProgress(taskID, probecore.StageDownload, processed, qualified, processed-qualified, info.Total)
		}
		task.DownloadSpeedSampleHook = func(sample task.DownloadSpeedSample) {
			s.emitSpeed(taskID, sample)
		}
	}
}

func (s *Service) beforeMobileStage(cfg probeConfig, taskID string, info probecore.StageInfo) {
	switch info.Stage {
	case probecore.StageTCP:
		s.emitProgress(taskID, probecore.StageTCP, 0, 0, 0, info.Total)
		utils.DebugEvent("stage.start", map[string]any{
			"config": map[string]any{
				"concurrency":               cfg.Routines,
				"max_loss_rate":             cfg.MaxLossRate,
				"max_tcp_latency_ms":        cfg.MaxDelayMS,
				"min_delay_ms":              cfg.MinDelayMS,
				"ping_times":                cfg.PingTimes,
				"retry_backoff_ms":          cfg.RetryBackoffMS,
				"retry_max_attempts":        cfg.RetryMaxAttempts,
				"skip_first_latency_sample": cfg.SkipFirstLatency,
				"tcp_port":                  cfg.TCPPort,
				"timeout_ms":                cfg.Stage1TimeoutMS,
			},
			"counts":  map[string]any{"total": info.Total},
			"message": "开始 TCP 测延迟。",
			"stage":   info.Stage,
			"task_id": taskID,
		})
	case probecore.StageTrace:
		s.emitProgress(taskID, probecore.StageTrace, 0, 0, 0, info.Total)
		utils.DebugEvent("stage.start", map[string]any{
			"config": map[string]any{
				"accepted_status_code": cfg.HttpingStatusCode,
				"cf_colo_filter":       cfg.HttpingCFColo,
				"cf_colo_filter_mode":  cfg.HttpingCFColoMode,
				"source_colo_filter":   cfg.SourceColoFilterPhase,
				"trace_colo_mode":      cfg.TraceColoMode,
				"trace_concurrency":    cfg.HeadRoutines,
				"trace_max_latency_ms": cfg.HeadMaxDelayMS,
				"trace_routines_limit": task.MaxTraceRoutines,
				"trace_url":            cfg.TraceURL,
				"retry_backoff_ms":     cfg.RetryBackoffMS,
				"retry_max_attempts":   cfg.RetryMaxAttempts,
				"timeout_ms":           cfg.Stage2TimeoutMS,
			},
			"counts":  map[string]any{"input": info.Input, "total": info.Total},
			"message": "开始追踪探测。",
			"stage":   info.Stage,
			"task_id": taskID,
		})
	case probecore.StageDownload:
		utils.DebugEvent("stage.start", map[string]any{
			"config": map[string]any{
				"concurrency":                  cfg.Stage3Concurrency,
				"download_time_seconds_per_ip": cfg.DownloadTimeSeconds,
				"legacy_download_count":        cfg.TestCount,
				"min_download_mbps":            cfg.MinSpeedMB,
				"min_download_speed_metric":    cfg.DownloadSpeedMetric,
				"retry_backoff_ms":             cfg.RetryBackoffMS,
				"retry_max_attempts":           cfg.RetryMaxAttempts,
				"stage3_limit":                 cfg.Stage3Limit,
			},
			"counts":  map[string]any{"input": info.Input, "total": info.Total},
			"message": "开始文件测速。",
			"stage":   info.Stage,
			"task_id": taskID,
		})
		if info.Total > 0 {
			s.emitProgress(taskID, probecore.StageDownload, 0, 0, 0, info.Total)
		}
	}
}

func (s *Service) afterMobileStage(cfg probeConfig, taskID string, info probecore.StageInfo) {
	switch info.Stage {
	case probecore.StageTCP:
		utils.DebugEvent("stage.complete", map[string]any{
			"counts":      mobileDebugStageCounts(info.Total, info.Passed, info.Failed),
			"duration_ms": info.DurationMS,
			"message":     "TCP 测延迟完成。",
			"stage":       info.Stage,
			"task_id":     taskID,
			"tcp": map[string]any{
				"delay_column":              "TCP延迟(ms)",
				"max_latency_ms":            cfg.MaxDelayMS,
				"ping_times":                cfg.PingTimes,
				"skip_first_latency_sample": cfg.SkipFirstLatency,
			},
		})
	case probecore.StageTrace:
		utils.DebugEvent("stage.complete", map[string]any{
			"counts":      mobileDebugStageCounts(info.Total, info.Passed, info.Failed),
			"duration_ms": info.DurationMS,
			"message":     "追踪探测完成。",
			"stage":       info.Stage,
			"task_id":     taskID,
			"trace": map[string]any{
				"accepted_status_code": cfg.HttpingStatusCode,
				"cf_colo_filter":       cfg.HttpingCFColo,
				"cf_colo_filter_mode":  cfg.HttpingCFColoMode,
				"source_colo_filter":   cfg.SourceColoFilterPhase,
				"trace_colo_mode":      cfg.TraceColoMode,
				"concurrency":          cfg.HeadRoutines,
				"max_latency_ms":       cfg.HeadMaxDelayMS,
				"url":                  cfg.TraceURL,
			},
		})
	case probecore.StageDownload:
		utils.DebugEvent("stage.complete", map[string]any{
			"counts":      mobileDebugStageCounts(info.Total, info.Passed, info.Failed),
			"duration_ms": info.DurationMS,
			"get": map[string]any{
				"concurrency":                  cfg.Stage3Concurrency,
				"download_time_seconds_per_ip": cfg.DownloadTimeSeconds,
				"min_download_mbps":            cfg.MinSpeedMB,
				"min_download_speed_metric":    cfg.DownloadSpeedMetric,
			},
			"message": "文件测速完成。",
			"stage":   info.Stage,
			"task_id": taskID,
		})
	}
}

func (s *Service) logProbePreparationFailure(cfg probeConfig, taskID string, source sourceSummary, invalidCount int, statuses []desktopSourceStatus, duration time.Duration, err error) {
	utils.Debug = cfg.Debug
	closeDebugLog, _, _ := s.configureProbeDebugRuntime(cfg)
	utils.SetDebugLogContext(taskID)
	defer closeDebugLog()

	utils.DebugEvent("probe.start", map[string]any{
		"config":  mobileDebugProbeConfigSummary(cfg),
		"message": "移动端探测任务启动。",
		"source":  mobileDebugSourceSummary(source, statuses),
		"task_id": taskID,
	})
	utils.DebugEvent("stage.start", map[string]any{
		"message": "开始生成 IP 池。",
		"stage":   "stage0_pool",
		"task_id": taskID,
	})
	utils.DebugEvent("stage.complete", map[string]any{
		"counts":      mobileDebugStage0Counts(source, invalidCount),
		"duration_ms": duration.Milliseconds(),
		"message":     "IP 池生成失败。",
		"source":      mobileDebugSourceSummary(source, statuses),
		"stage":       "stage0_pool",
		"task_id":     taskID,
	})
	mobileLogProbeFailed(taskID, "stage0_pool", time.Now().Add(-duration), nil, err, false)
}

func mobileLogProbeFailed(taskID, stage string, startedAt time.Time, completedStages []string, err error, recoverable bool) {
	message := "移动端探测任务失败。"
	errText := ""
	if err != nil {
		message = err.Error()
		errText = err.Error()
	}
	utils.DebugEvent("probe.failed", map[string]any{
		"completed_stages": completedStages,
		"duration_ms":      time.Since(startedAt).Milliseconds(),
		"error":            errText,
		"message":          message,
		"recoverable":      recoverable,
		"stage":            stage,
		"task_id":          taskID,
	})
}

func mobileDebugStageCounts(total, passed, failed int) map[string]any {
	if failed < 0 {
		failed = 0
	}
	return map[string]any{
		"failed": failed,
		"passed": passed,
		"total":  total,
	}
}

func mobileDebugStage0Counts(source sourceSummary, invalidCount int) map[string]any {
	total := source.CandidateCount
	if total == 0 {
		total = source.ValidCount + source.DuplicateCount + invalidCount
	}
	return map[string]any{
		"accepted": source.ValidCount,
		"filtered": source.DuplicateCount,
		"invalid":  invalidCount,
		"total":    total,
	}
}

func mobileDebugSourceSummary(source sourceSummary, statuses []desktopSourceStatus) map[string]any {
	return map[string]any{
		"candidate_count": source.CandidateCount,
		"duplicate_count": source.DuplicateCount,
		"invalid_count":   source.InvalidCount,
		"raw_line_count":  source.RawLineCount,
		"source_statuses": statuses,
		"unique_count":    source.UniqueCount,
		"valid_count":     source.ValidCount,
	}
}

func mobileDebugProbeConfigSummary(cfg probeConfig) map[string]any {
	return map[string]any{
		"debug_capture_address":             cfg.DebugCaptureAddress,
		"debug_capture_enabled":             cfg.DebugCaptureEnabled,
		"debug_log_mode":                    cfg.DebugLogMode,
		"debug_log_verbosity":               cfg.DebugLogVerbosity,
		"disable_download":                  cfg.DisableDownload,
		"download_buffer_kb":                cfg.DownloadBufferKB,
		"download_count":                    cfg.TestCount,
		"download_concurrency":              cfg.Stage3Concurrency,
		"download_get_concurrency":          cfg.DownloadGetConcurrency,
		"download_http_protocol":            cfg.DownloadHTTPProtocol,
		"download_speed_metric":             cfg.DownloadSpeedMetric,
		"download_speed_sample_interval_ms": cfg.DownloadSpeedSampleIntervalMS,
		"download_time_seconds_per_ip":      cfg.DownloadTimeSeconds,
		"download_warmup_seconds":           cfg.DownloadWarmupSeconds,
		"event_throttle_ms":                 cfg.EventThrottleMS,
		"csv_encoding":                      cfg.CSVEncoding,
		"head_routines":                     cfg.HeadRoutines,
		"httping":                           cfg.Httping,
		"httping_cf_colo":                   cfg.HttpingCFColo,
		"httping_cf_colo_mode":              cfg.HttpingCFColoMode,
		"httping_status_code":               cfg.HttpingStatusCode,
		"max_http_latency_ms":               cfg.HeadMaxDelayMS,
		"max_loss_rate":                     cfg.MaxLossRate,
		"max_tcp_latency_ms":                cfg.MaxDelayMS,
		"min_delay_ms":                      cfg.MinDelayMS,
		"min_download_mbps":                 cfg.MinSpeedMB,
		"ping_times":                        cfg.PingTimes,
		"retry_backoff_ms":                  cfg.RetryBackoffMS,
		"retry_max_attempts":                cfg.RetryMaxAttempts,
		"request_headers_count":             httpcfg.RequestHeadersCount(cfg.RequestHeaders),
		"routines":                          cfg.Routines,
		"skip_first_latency_sample":         cfg.SkipFirstLatency,
		"stage3_limit":                      cfg.Stage3Limit,
		"strategy":                          cfg.Strategy,
		"tcp_port":                          cfg.TCPPort,
		"timeout_stage1_ms":                 cfg.Stage1TimeoutMS,
		"timeout_stage2_ms":                 cfg.Stage2TimeoutMS,
		"trace_colo_mode":                   cfg.TraceColoMode,
		"trace_url":                         cfg.TraceURL,
		"source_colo_filter_phase":          cfg.SourceColoFilterPhase,
		"url":                               cfg.URL,
		"user_agent":                        cfg.UserAgent,
		"write_output":                      cfg.WriteOutput,
	}
}

func (s *Service) emit(taskID, event string, payload map[string]any) {
	if payload == nil {
		payload = map[string]any{}
	}
	s.stateMu.Lock()
	s.eventSeq++
	seq := s.eventSeq
	sink := s.eventSink
	s.stateMu.Unlock()
	if sink == nil {
		return
	}
	sink.OnProbeEvent(encodeJSON(probeEventEnvelope{
		Event:         event,
		Payload:       payload,
		SchemaVersion: schemaVersion,
		Seq:           seq,
		TaskID:        taskID,
		TS:            time.Now().Format(time.RFC3339),
	}))
}

func (s *Service) emitProgress(taskID, stage string, processed, passed, failed, total int) {
	if !s.shouldEmitProgress(stage, processed, total) {
		return
	}
	s.emit(taskID, "probe.progress", map[string]any{
		"failed":    failed,
		"passed":    passed,
		"processed": processed,
		"stage":     stage,
		"total":     total,
	})
}

func (s *Service) configureProgressThrottle(throttle time.Duration) {
	if throttle <= 0 {
		throttle = 100 * time.Millisecond
	}
	s.stateMu.Lock()
	s.progressThrottle = throttle
	s.lastProgressStage = ""
	s.lastProgressAt = time.Time{}
	s.stateMu.Unlock()
}

func (s *Service) shouldEmitProgress(stage string, processed, total int) bool {
	now := time.Now()
	s.stateMu.Lock()
	throttle := s.progressThrottle
	if throttle <= 0 {
		throttle = 100 * time.Millisecond
	}
	shouldEmit := processed <= 1 || total <= 0 || processed >= total || stage != s.lastProgressStage || now.Sub(s.lastProgressAt) >= throttle
	if shouldEmit {
		s.lastProgressStage = stage
		s.lastProgressAt = now
	}
	s.stateMu.Unlock()
	return shouldEmit
}

func (s *Service) emitSpeed(taskID string, sample task.DownloadSpeedSample) {
	s.emit(taskID, "probe.speed", map[string]any{
		"average_speed_mb_s":  sample.AverageSpeedMBs,
		"average_ready":       sample.AverageReady,
		"attempt":             sample.Attempt,
		"body_read":           sample.BodyRead,
		"bytes_read":          sample.BytesRead,
		"colo":                sample.Colo,
		"current_ready":       sample.CurrentReady,
		"current_speed_mb_s":  sample.CurrentSpeedMBs,
		"elapsed_ms":          sample.ElapsedMS,
		"ip":                  sample.IP,
		"measured_bytes":      sample.MeasuredBytes,
		"measured_elapsed_ms": sample.MeasuredElapsedMS,
		"sample_bytes":        sample.SampleBytes,
		"sample_elapsed_ms":   sample.SampleElapsedMS,
		"stage":               sample.Stage,
		"transfer_complete":   sample.TransferComplete,
	})
}

func (s *Service) resolveResultFilePath(payload map[string]any, cfg probeConfig) string {
	for _, key := range []string{"path", "source_path", "sourcePath", "export_path", "exportPath"} {
		if path := strings.TrimSpace(stringValue(payload[key], "")); path != "" && !strings.HasPrefix(path, "content://") {
			return path
		}
	}
	if outputFile := s.exportPath(cfg.OutputFile); strings.TrimSpace(outputFile) != "" {
		return outputFile
	}
	return s.exportPath("result.csv")
}

func readMobileProbeResultRowsFromCSV(path string) ([]probeResultRow, error) {
	return appcore.ReadProbeResultRowsFromCSV(path)
}

func (s *Service) setCurrentTask(taskID string) (bool, string) {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	if s.currentTaskID != "" {
		return false, s.currentTaskID
	}
	s.currentTaskID = taskID
	s.pauseRequested = false
	s.pausedTaskID = ""
	s.downloadCancel = nil
	if s.pauseCond != nil {
		s.pauseCond.Broadcast()
	}
	if s.cancelRequested && s.cancelTaskID == taskID {
		return true, taskID
	}
	s.cancelTaskID = ""
	s.cancelRequested = false
	return true, taskID
}

func (s *Service) clearCurrentTask(taskID string) {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	if s.currentTaskID == taskID {
		s.currentTaskID = ""
		s.cancelTaskID = ""
		s.cancelRequested = false
		s.pauseRequested = false
		s.pausedTaskID = ""
		s.downloadCancel = nil
		if s.pauseCond != nil {
			s.pauseCond.Broadcast()
		}
	}
}

func (s *Service) isCancelRequested(taskID string) bool {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	return s.currentTaskID == taskID && s.cancelRequested && s.cancelTaskID == taskID
}

func (s *Service) registerDownloadInterrupt(taskID, stage, ip string, interrupt func()) func() {
	s.stateMu.Lock()
	if s.currentTaskID == taskID && stage == task.DownloadSpeedSampleStage {
		s.downloadCancelSeq++
		seq := s.downloadCancelSeq
		s.downloadCancel = interrupt
		if s.pauseRequested && s.pausedTaskID == taskID && interrupt != nil {
			go interrupt()
		}
		s.stateMu.Unlock()
		return func() {
			s.stateMu.Lock()
			if s.currentTaskID == taskID && s.downloadCancelSeq == seq {
				s.downloadCancel = nil
			}
			s.stateMu.Unlock()
		}
	}
	s.stateMu.Unlock()
	return func() {}
}

func (s *Service) waitIfProbePaused(taskID, stage, ip string) {
	s.stateMu.Lock()
	announced := false
	for s.currentTaskID == taskID && s.pauseRequested && s.pausedTaskID == taskID {
		if !announced {
			s.stateMu.Unlock()
			s.emit(taskID, "probe.cooling", map[string]any{
				"ip":          ip,
				"reason":      fmt.Sprintf("%s 已暂停，点击继续任务后从当前进度继续。", stage),
				"recoverable": true,
				"stage":       stage,
			})
			s.stateMu.Lock()
			announced = true
			continue
		}
		if s.pauseCond == nil {
			s.stateMu.Unlock()
			time.Sleep(25 * time.Millisecond)
			s.stateMu.Lock()
			continue
		}
		s.pauseCond.Wait()
	}
	s.stateMu.Unlock()
}

func (s *Service) configureProbeDebugRuntime(cfg probeConfig) (func(), []string, string) {
	path, err := utils.ConfigureDebugLog(cfg.Debug, s.debugLogPath(), cfg.DebugLogMode, cfg.DebugLogFormat, cfg.DebugLogVerbosity)
	if err != nil {
		return func() {}, []string{fmt.Sprintf("初始化调试日志失败：%v", err)}, ""
	}
	warnings := make([]string, 0, 2)
	if cfg.Debug && path != "" {
		warnings = append(warnings, fmt.Sprintf("调试日志已写入 %s", path))
	}
	if captureAddress := effectiveDebugCaptureAddress(cfg); captureAddress != "" {
		warnings = append(warnings, fmt.Sprintf("调试模式已将请求拨号目标覆盖为 %s", captureAddress))
	}
	return func() {
		_ = utils.CloseDebugLog()
	}, warnings, path
}

func (s *Service) debugLogPathForProbeConfig(cfg probeConfig) string {
	if !cfg.Debug {
		return ""
	}
	return s.debugLogPath()
}

func (s *Service) withDebugLogPath(payload map[string]any, debugLogPath string) map[string]any {
	if strings.TrimSpace(debugLogPath) != "" {
		payload["debug_log_path"] = strings.TrimSpace(debugLogPath)
	}
	return payload
}

func resolveProbeSource(cfg probeConfig, raw string) (string, sourceSummary, error) {
	sourceText := strings.TrimSpace(raw)
	if sourceText == "" && strings.TrimSpace(cfg.IPText) != "" {
		sourceText = cfg.IPText
	}
	if sourceText == "" {
		fileRaw, err := os.ReadFile(cfg.IPFile)
		if err != nil {
			return "", sourceSummary{}, fmt.Errorf("读取 IP 数据文件失败：%w", err)
		}
		sourceText = string(fileRaw)
	}
	return sourceText, summarizeSource(sourceText), nil
}

func summarizeSource(raw string) sourceSummary {
	return probecore.SummarizeSource(raw, sourceParseResolver)
}
