package appcore

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/kernel/appcore/contracts"
	"github.com/axuitomo/CFST-GUI/internal/kernel/appcore/sources"
	"github.com/axuitomo/CFST-GUI/internal/kernel/domain/probe/probecore"
	"github.com/axuitomo/CFST-GUI/internal/kernel/domain/probe/task"
	"github.com/axuitomo/CFST-GUI/internal/kernel/infra/utils"
)

type ProbeExecutionHooks struct {
	RunTCP      func(*task.Engine) (utils.PingDelaySet, error)
	RunTrace    func(*task.Engine, utils.PingDelaySet) utils.PingDelaySet
	RunDownload func(*task.Engine, utils.PingDelaySet) utils.DownloadSpeedSet
}

type ProbePostProcessResult struct {
	Notification *contracts.UploadNotification
	Warnings     []string
}

func (s *Service) RunProbe(payload contracts.ProbePayload) (contracts.ProbeRunResult, error) {
	payload, cfg, warnings, taskID, err := s.prepareProbePayload(payload)
	if err != nil {
		return contracts.ProbeRunResult{}, err
	}
	value, runErr := s.runProbeWithRunner(taskID, func(ctx context.Context) (any, error) {
		return s.executeProbe(ctx, payload, cfg, warnings, taskID)
	})
	if runErr != nil {
		s.publishProbeFailure(taskID, runErr, "", "")
		return contracts.ProbeRunResult{}, runErr
	}
	result, ok := value.(contracts.ProbeRunResult)
	if !ok {
		return contracts.ProbeRunResult{}, fmt.Errorf("unexpected probe result type %T", value)
	}
	return result, nil
}

func (s *Service) StartProbe(payload contracts.ProbePayload) CommandResult {
	payload, cfg, warnings, taskID, err := s.prepareProbePayload(payload)
	if err != nil {
		return NewCommandResult("PROBE_PAYLOAD_INVALID", nil, err.Error(), false, nil, warnings)
	}
	return s.startProbeWithRunner(taskID, func(ctx context.Context) (any, error) {
		return s.executeProbe(ctx, payload, cfg, warnings, taskID)
	})
}

func (s *Service) prepareProbePayload(payload contracts.ProbePayload) (contracts.ProbePayload, probecore.ProbeConfig, []string, string, error) {
	snapshot := cloneServiceMap(payload.Config)
	if len(snapshot) == 0 {
		loaded, err := s.LoadConfig()
		if err != nil {
			return payload, probecore.ProbeConfig{}, nil, "", err
		}
		snapshot = loaded.Snapshot
	}
	if len(payload.Sources) == 0 {
		payload.Sources = sources.SourcesFromAny(snapshot["sources"])
	}
	payload.Sources = sources.SourcesWithGroupIPMode(payload.Sources, sources.SourceGroupModeFromSnapshot(snapshot))
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
	warnings = append(warnings, sources.SourceGroupWarnings(snapshot, payload.Sources)...)
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

func (s *Service) executeProbe(ctx context.Context, payload contracts.ProbePayload, cfg probecore.ProbeConfig, configWarnings []string, taskID string) (contracts.ProbeRunResult, error) {
	// 进度事件节流在整次探测内沿用配置值，含输入源池（stage0）阶段。
	s.runtime.ConfigureProgressThrottle(time.Duration(cfg.EventThrottleMS) * time.Millisecond)
	s.probeMu.Lock()
	defer s.probeMu.Unlock()

	prepared, summary, invalidCount, err := s.prepareProbeSources(ctx, payload, cfg, taskID)
	if err != nil {
		return contracts.ProbeRunResult{}, err
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
		return contracts.ProbeRunResult{}, probecore.ErrProbeCanceled
	}

	result.SourceStatuses = prepared.SourceStatuses
	result.Warnings = probecore.DedupeStrings(append(result.Warnings, prepared.Warnings...))
	s.waitProbePaused(taskID, "post_probe_push", "")
	processed := s.processPostProbePush(ctx, payload, result, func() { s.waitProbePaused(taskID, "post_probe_push", "") })
	result.Warnings = probecore.DedupeStrings(append(result.Warnings, processed.Warnings...))
	result.UploadNotification = processed.Notification
	if processed.Notification == nil && s.probeCancelled(ctx, taskID) {
		// 推送跑到一半被终止：任务 ctx 已取消，发送必须用不会被它掐断的 context，
		// 否则这条「熔断/终止」通知永远发不出去。
		if aborted := s.buildPostProbeAbortedNotification(payload.Config, result); aborted != nil {
			result.Warnings = append(result.Warnings, s.sendUploadNotification(context.WithoutCancel(ctx), payload.Config, *aborted)...)
			result.UploadNotification = aborted
		}
	}
	if s.probeCancelled(ctx, taskID) {
		s.publishProbeCancelled(taskID, "persist_results", result.DebugLogPath)
		return contracts.ProbeRunResult{}, probecore.ErrProbeCanceled
	}

	s.waitProbePaused(taskID, "persist_results", "")
	if err := s.PersistCompletedProbe(taskID, result); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("保存任务结果失败：%v", err))
		s.appendProbeError("task_results.persist_failed", map[string]any{"debug_log_path": result.DebugLogPath, "message": err.Error(), "task_id": taskID})
	}
	if s.probeCancelled(ctx, taskID) {
		s.publishProbeCancelled(taskID, "persist_results", result.DebugLogPath)
		return contracts.ProbeRunResult{}, probecore.ErrProbeCanceled
	}

	for {
		s.waitProbePaused(taskID, "completed", "")
		committed, active, cancelled := s.runtime.TryCommitCompletion(taskID)
		if committed {
			break
		}
		if !active || cancelled || ctx.Err() != nil {
			s.publishProbeCancelled(taskID, "completed", result.DebugLogPath)
			return contracts.ProbeRunResult{}, probecore.ErrProbeCanceled
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
	s.runtime.Clear(taskID)
	_ = s.PublishProbeEvent(context.Background(), ProbeEvent{Event: "probe.completed", TaskID: taskID, Payload: completedPayload})
	return result, nil
}

func (s *Service) runProbePortGroups(ctx context.Context, cfg probecore.ProbeConfig, configWarnings []string, taskContext probecore.TaskContext, prepared sources.PreparedSources, summary probecore.SourceSummary, taskID string, groups []probecore.PortGroup) (contracts.ProbeRunResult, error) {
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
				MCISCandidates: prepared.MCISCandidates,
				SourcePorts:    prepared.SourcePorts, SourceStatuses: prepared.SourceStatuses,
				Source: request.Source, SourceText: request.SourceText, TaskContext: request.TaskContext, TaskID: taskID,
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
	return contracts.ProbeRunResult{
		Config: resultCfg, DebugLogPath: workflow.DebugLogPath, DurationMS: workflow.DurationMS,
		FailureStage: workflow.FailureStage, OutputFile: workflow.OutputFile, RawResults: workflow.RawResults,
		Results: workflow.Results, SchemaVersion: CommandSchemaVersion, Source: workflow.Source,
		SourceStatuses: prepared.SourceStatuses, StartedAt: workflow.StartedAt, Summary: workflow.Summary,
		TaskID: taskID, TaskContext: workflow.TaskContext, TraceDiagnostics: workflow.TraceDiagnostics,
		Warnings: probecore.DedupeStrings(workflow.Warnings),
	}, err
}

func currentServiceOutputFile(cfg probecore.ProbeConfig) string {
	if !cfg.WriteOutput {
		return ""
	}
	return strings.TrimSpace(cfg.OutputFile)
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
