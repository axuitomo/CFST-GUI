package app

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/appcore"
	"github.com/axuitomo/CFST-GUI/internal/probecore"
	"github.com/axuitomo/CFST-GUI/internal/task"
	"github.com/axuitomo/CFST-GUI/internal/utils"
)

func (a *App) executeSelectSourcesNode(node appcore.PipelineNode, runtimeCtx *pipelineRuntimeContext) (pipelineNodeExecutionResult, error) {
	sources, err := pipelineSourceGroupSourcesForNode(runtimeCtx, node)
	if err != nil {
		return pipelineNodeExecutionResult{Message: err.Error(), Status: "failed"}, err
	}
	runtimeCtx.SelectedSources = cloneDesktopSources(sources)
	enabledCount := 0
	for _, source := range sources {
		if source.Enabled {
			enabledCount++
		}
	}
	message := fmt.Sprintf("输入源组已选择 %d 个输入源。", len(sources))
	if len(sources) == 0 {
		message = "输入源组没有选中可用输入源。"
	}
	return pipelineNodeExecutionResult{
		Message: message,
		Metrics: map[string]any{
			"enabled_sources":  enabledCount,
			"selected_sources": len(sources),
		},
		Output:        cloneDesktopSources(sources),
		OutputSummary: fmt.Sprintf("%d 个输入源", len(sources)),
		Status:        "completed",
	}, nil
}

func (a *App) executeFilterSourcesNode(node appcore.PipelineNode, runtimeCtx *pipelineRuntimeContext) (pipelineNodeExecutionResult, error) {
	sources := pipelineProbeSourcesForNode(runtimeCtx, node)
	runtimeCtx.SelectedSources = cloneDesktopSources(sources)
	enabledCount := 0
	for _, source := range sources {
		if source.Enabled {
			enabledCount++
		}
	}
	message := fmt.Sprintf("输入源筛选已输出 %d 个输入源。", len(sources))
	if len(sources) == 0 {
		message = "输入源筛选后没有可用输入源。"
	}
	return pipelineNodeExecutionResult{
		Message: message,
		Metrics: map[string]any{
			"enabled_sources":  enabledCount,
			"selected_sources": len(sources),
		},
		Output:        cloneDesktopSources(sources),
		OutputSummary: fmt.Sprintf("%d 个输入源", len(sources)),
		Status:        "completed",
	}, nil
}

func (a *App) executeProbeTCPNode(node appcore.PipelineNode, runtimeCtx *pipelineRuntimeContext) (pipelineNodeExecutionResult, error) {
	stage, snapshot, err := a.preparePipelineProbeStage(node, runtimeCtx)
	if err != nil {
		return pipelineNodeExecutionResult{Message: err.Error(), Status: "failed"}, err
	}
	cfg := stage.Config
	if err := applyProbeConfig(cfg); err != nil {
		return pipelineNodeExecutionResult{Message: err.Error(), Status: "failed"}, err
	}
	task.SourceColoFilters = task.CloneSourceColoFilterMap(stage.Prepared.SourceColoFilters)
	task.Httping = false
	info := probecore.StageInfo{Stage: probecore.StageTCP, Total: stage.Source.ValidCount}
	tcpData, err := probecore.RunTCPStage(info, probecore.StageWorkflowAdapter{
		RunTCP: func() (utils.PingDelaySet, error) {
			return desktopTCPProbeRunner()
		},
	})
	if err != nil {
		return pipelineNodeExecutionResult{Message: err.Error(), Status: "failed"}, err
	}
	stage.TCPData = append(utils.PingDelaySet(nil), tcpData...)
	stage.CompletedStages = append(stage.CompletedStages, probecore.StageTCP)
	runtimeCtx.ProbeStage = stage
	runtimeCtx.ProbeStageSnapshot = snapshot
	return pipelineNodeExecutionResult{
		Message: "TCP 延迟测速已完成。",
		Metrics: map[string]any{
			"input_count":  stage.Source.ValidCount,
			"passed_count": len(tcpData),
		},
		Output:        append(utils.PingDelaySet(nil), tcpData...),
		OutputSummary: fmt.Sprintf("%d / %d 个候选", len(tcpData), stage.Source.ValidCount),
		Status:        "completed",
	}, nil
}

func (a *App) executeProbeTraceNode(node appcore.PipelineNode, runtimeCtx *pipelineRuntimeContext) (pipelineNodeExecutionResult, error) {
	stage, err := pipelineExistingProbeStage(runtimeCtx, probecore.StageTCP)
	if err != nil {
		return pipelineNodeExecutionResult{Message: err.Error(), Status: "failed"}, err
	}
	snapshot := pipelineProbeSnapshotForNode(&pipelineRuntimeContext{ConfigSnapshot: runtimeCtx.ProbeStageSnapshot}, node)
	cfg, configWarnings := desktopConfigToProbeConfig(snapshot)
	cfg = applyDesktopExportConfig(cfg, snapshot, runtimeCtx.TaskID, runtimeCtx.Profile.Name)
	stage.Config = cfg
	stage.ConfigWarnings = dedupeStrings(append(stage.ConfigWarnings, configWarnings...))
	stage.TaskContext.ConfigSource = firstNonEmptyString(runtimeCtx.Payload.ConfigSource, "pipeline")
	if err := applyProbeConfig(cfg); err != nil {
		return pipelineNodeExecutionResult{Message: err.Error(), Status: "failed"}, err
	}
	task.SourceColoFilters = task.CloneSourceColoFilterMap(stage.Prepared.SourceColoFilters)
	traceTotal := task.EstimateTraceProbeCount(len(stage.TCPData))
	info := probecore.StageInfo{Stage: probecore.StageTrace, Input: len(stage.TCPData), Total: traceTotal}
	traceData, err := probecore.RunTraceStage(info, stage.TCPData, probecore.StageWorkflowAdapter{RunTrace: desktopTraceProbeRunner})
	if err != nil {
		return pipelineNodeExecutionResult{Message: err.Error(), Status: "failed"}, err
	}
	stage.TraceData = append(utils.PingDelaySet(nil), traceData...)
	stage.CompletedStages = append(stage.CompletedStages, probecore.StageTrace)
	runtimeCtx.ProbeStage = stage
	runtimeCtx.ProbeStageSnapshot = snapshot
	message := "追踪测试已完成。"
	if len(traceData) == 0 && len(stage.TCPData) > 0 {
		message = "追踪测试未命中可用候选。"
	}
	return pipelineNodeExecutionResult{
		Message: message,
		Metrics: map[string]any{
			"input_count":  len(stage.TCPData),
			"passed_count": len(traceData),
		},
		Output:        append(utils.PingDelaySet(nil), traceData...),
		OutputSummary: fmt.Sprintf("%d / %d 个候选", len(traceData), len(stage.TCPData)),
		Status:        "completed",
	}, nil
}

func (a *App) executeProbeDownloadNode(node appcore.PipelineNode, runtimeCtx *pipelineRuntimeContext) (pipelineNodeExecutionResult, error) {
	stage, err := pipelineExistingProbeStage(runtimeCtx, probecore.StageTrace)
	if err != nil {
		return pipelineNodeExecutionResult{Message: err.Error(), Status: "failed"}, err
	}
	snapshot := pipelineProbeSnapshotForNode(&pipelineRuntimeContext{ConfigSnapshot: runtimeCtx.ProbeStageSnapshot}, node)
	cfg, configWarnings := desktopConfigToProbeConfig(snapshot)
	cfg = applyDesktopExportConfig(cfg, snapshot, runtimeCtx.TaskID, runtimeCtx.Profile.Name)
	stage.Config = cfg
	stage.ConfigWarnings = dedupeStrings(append(stage.ConfigWarnings, configWarnings...))
	if err := applyProbeConfig(cfg); err != nil {
		return pipelineNodeExecutionResult{Message: err.Error(), Status: "failed"}, err
	}
	task.SourceColoFilters = task.CloneSourceColoFilterMap(stage.Prepared.SourceColoFilters)
	downloadInput := probecore.LimitPingDelaySet(stage.TraceData, cfg.Stage3Limit)
	downloadTotal := probecore.EstimateDownloadProbeCount(len(downloadInput))
	info := probecore.StageInfo{Stage: probecore.StageDownload, Input: len(downloadInput), Total: downloadTotal}
	speedData, err := probecore.RunDownloadStage(info, downloadInput, probecore.StageWorkflowAdapter{RunDownload: desktopDownloadProbeRunner})
	if err != nil {
		return pipelineNodeExecutionResult{Message: err.Error(), Status: "failed"}, err
	}
	stage.CompletedStages = append(stage.CompletedStages, probecore.StageDownload)
	resultData := []utils.CloudflareIPData(speedData)
	resultData = probecore.LimitFinalResults(resultData, cfg.PrintNum, cfg.DownloadSpeedMetric)
	rows := pipelineRowsFromRawResults(resultData, stage.SourcePorts, stage.TestPorts, cfg.TCPPort)
	warnings := probecore.BuildProbeWarnings(stage.Source)
	warnings = append(warnings, stage.ConfigWarnings...)
	warnings = append(warnings, stage.Warnings...)
	if len(stage.TraceData) == 0 && len(stage.TCPData) > 0 {
		warnings = append(warnings, "追踪探测未命中可用候选，已无可导出的结果。")
	}
	outputFile := ""
	if len(resultData) > 0 {
		outputFile = currentOutputFile(cfg)
		if outputFile != "" {
			if err := applyProbeConfig(cfg); err != nil {
				warnings = append(warnings, fmt.Sprintf("结果导出配置失败：%v", err))
				outputFile = ""
			} else if exportErr := utils.ExportCsv(resultData); exportErr != nil {
				warnings = append(warnings, fmt.Sprintf("结果导出失败：%v", exportErr))
				outputFile = ""
			}
		}
	}
	probeResult := ProbeRunResult{
		Config:         cfg,
		DurationMS:     time.Since(stage.StartedAt).Milliseconds(),
		OutputFile:     outputFile,
		Results:        rows,
		Source:         stage.Source,
		SourceStatuses: stage.Prepared.SourceStatuses,
		StartedAt:      stage.StartedAt.Format(time.RFC3339),
		Summary:        probecore.SummarizeProbeRows(rows, downloadTotal),
		TaskContext:    stage.TaskContext,
		Warnings:       dedupeStrings(warnings),
		SchemaVersion:  guiSchemaVersion,
		RawResults:     append([]utils.CloudflareIPData(nil), resultData...),
	}
	runtimeCtx.ProbeResult = &probeResult
	runtimeCtx.FilteredRows = nil
	runtimeCtx.Warnings = dedupeStrings(append(runtimeCtx.Warnings, probeResult.Warnings...))
	runtimeCtx.ProbeStage = stage
	runtimeCtx.ProbeStageSnapshot = snapshot
	return pipelineNodeExecutionResult{
		Message: "下载测速已完成。",
		Metrics: map[string]any{
			"input_count":  len(downloadInput),
			"result_count": len(probeResult.Results),
		},
		Output:        probeResult,
		OutputSummary: fmt.Sprintf("%d 条测速结果", len(probeResult.Results)),
		Status:        "completed",
	}, nil
}

func (a *App) executeFilterResultsNode(node appcore.PipelineNode, runtimeCtx *pipelineRuntimeContext) (pipelineNodeExecutionResult, error) {
	selection, err := pipelineEnsureUploadSelection(runtimeCtx, node)
	if err != nil {
		return pipelineNodeExecutionResult{
			Message: err.Error(),
			Status:  "failed",
		}, err
	}
	runtimeCtx.FilteredRows = slices.Clone(selection.FilteredRows)
	runtimeCtx.Warnings = dedupeStrings(append(runtimeCtx.Warnings, selection.Warnings...))
	message := fmt.Sprintf("结果筛选保留 %d / %d 条结果。", len(selection.FilteredRows), len(selection.InputRows))
	if len(selection.FilteredRows) == 0 {
		message = "结果筛选后没有剩余结果。"
	}
	return pipelineNodeExecutionResult{
		Message: message,
		Metrics: map[string]any{
			"cloudflare_count": len(selection.CloudflareRows),
			"filtered_count":   len(selection.FilteredRows),
			"github_count":     len(selection.GitHubRows),
			"input_count":      len(selection.InputRows),
		},
		Output:        selection,
		OutputSummary: fmt.Sprintf("%d / %d 条", len(selection.FilteredRows), len(selection.InputRows)),
		Status:        "completed",
	}, nil
}

func (a *App) executeBranchHasResultsNode(node appcore.PipelineNode, runtimeCtx *pipelineRuntimeContext) (pipelineNodeExecutionResult, error) {
	rows := pipelineRowsForNodeSource(runtimeCtx, stringValue(pipelineNodeConfig(node)["source"], ""))
	outcome := "false"
	message := "没有可用结果，进入回退路径。"
	if len(rows) > 0 {
		outcome = "true"
		message = "命中可用结果，继续后续投递。"
	}
	return pipelineNodeExecutionResult{
		Message: message,
		Metrics: map[string]any{
			"result_count": len(rows),
		},
		Outcome:       outcome,
		OutputSummary: fmt.Sprintf("result_count=%d", len(rows)),
		Status:        "completed",
	}, nil
}

func (a *App) executeDeliverDNSNode(node appcore.PipelineNode, runtimeCtx *pipelineRuntimeContext) (pipelineNodeExecutionResult, error) {
	if runtimeCtx.SchedulerOverrides.AllowDNSPush != nil && !*runtimeCtx.SchedulerOverrides.AllowDNSPush {
		return pipelineNodeExecutionResult{
			Message:       "定时调度已关闭自动 DNS 推送，本节点跳过。",
			OutputSummary: "scheduler skipped",
			Status:        "completed",
		}, nil
	}
	if !appcore.PipelineDNSPushEnabled(runtimeCtx.Target.DNSPushPolicy) {
		return pipelineNodeExecutionResult{
			Message:       "目标已配置为跳过 DNS 推送。",
			OutputSummary: "target skipped",
			Status:        "skipped",
		}, nil
	}
	selection, err := pipelineEnsureUploadSelection(runtimeCtx, node)
	if err != nil {
		return pipelineNodeExecutionResult{
			Message: err.Error(),
			Status:  "failed",
		}, err
	}
	dnsSnapshot := pipelineDNSSnapshotForNode(runtimeCtx, node)
	recordType := stringValue(mapValue(dnsSnapshot["cloudflare"])["record_type"], cloudflareRecordTypeA)
	rows := filterRowsForCloudflareRecordType(selection.CloudflareRows, recordType)
	if len(rows) == 0 {
		return pipelineNodeExecutionResult{
			Message:       "筛选后没有可推送到 Cloudflare 的 IP。",
			Metrics:       map[string]any{"cloudflare_count": 0},
			OutputSummary: "0 条",
			Status:        "skipped",
		}, nil
	}
	dnsResult := a.PushCloudflareDNSRecords(map[string]any{
		"config": dnsSnapshot,
		"ipsRaw": probeRowsIPList(rows),
	})
	runtimeCtx.DNSResult = dnsResult
	runtimeCtx.Warnings = dedupeStrings(append(runtimeCtx.Warnings, dnsResult.Warnings...))
	status := "completed"
	if !dnsResult.OK {
		status = "failed"
	}
	result := pipelineNodeExecutionResult{
		Message: dnsResult.Message,
		Metrics: map[string]any{
			"cloudflare_count": len(rows),
		},
		Output:        dnsResult,
		OutputSummary: fmt.Sprintf("%d 条", len(rows)),
		Status:        status,
	}
	if !dnsResult.OK {
		return result, errors.New(dnsResult.Message)
	}
	return result, nil
}

func (a *App) executeDeliverGitHubNode(node appcore.PipelineNode, runtimeCtx *pipelineRuntimeContext) (pipelineNodeExecutionResult, error) {
	if runtimeCtx.SchedulerOverrides.AllowGitHubExport != nil && !*runtimeCtx.SchedulerOverrides.AllowGitHubExport {
		return pipelineNodeExecutionResult{
			Message:       "定时调度已关闭自动 GitHub 导出，本节点跳过。",
			OutputSummary: "scheduler skipped",
			Status:        "completed",
		}, nil
	}
	selection, err := pipelineEnsureUploadSelection(runtimeCtx, node)
	if err != nil {
		return pipelineNodeExecutionResult{
			Message: err.Error(),
			Status:  "failed",
		}, err
	}
	if len(selection.GitHubRows) == 0 {
		return pipelineNodeExecutionResult{
			Message:       "筛选后没有可导出的 GitHub 结果。",
			Metrics:       map[string]any{"github_count": 0},
			OutputSummary: "0 条",
			Status:        "skipped",
		}, nil
	}
	exportResult := a.ExportResultsToGitHub(map[string]any{
		"config":  runtimeCtx.ConfigSnapshot,
		"results": selection.GitHubRows,
		"task_id": runtimeCtx.TaskID,
	})
	runtimeCtx.Warnings = dedupeStrings(append(runtimeCtx.Warnings, exportResult.Warnings...))
	status := "completed"
	if !exportResult.OK {
		status = "failed"
	}
	result := pipelineNodeExecutionResult{
		Message: exportResult.Message,
		Metrics: map[string]any{
			"github_count": len(selection.GitHubRows),
		},
		Output:        exportResult,
		OutputSummary: fmt.Sprintf("%d 条", len(selection.GitHubRows)),
		Status:        status,
	}
	if !exportResult.OK {
		return result, errors.New(exportResult.Message)
	}
	return result, nil
}

func (a *App) executeCheckOutputNode(node appcore.PipelineNode, runtimeCtx *pipelineRuntimeContext) (pipelineNodeExecutionResult, error) {
	nodeConfig := pipelineNodeConfig(node)
	sourceRows := pipelineRowsForNodeSource(runtimeCtx, stringValue(nodeConfig["source"], "probe_results"))
	if len(sourceRows) == 0 {
		return pipelineNodeExecutionResult{
			Message:       "没有可输出的测速结果，需要人工复核。",
			Metrics:       map[string]any{"result_count": 0},
			OutputSummary: "0 条结果",
			Status:        "manual_review",
		}, nil
	}
	selection, err := pipelineEnsureUploadSelection(runtimeCtx, node)
	if err != nil {
		return pipelineNodeExecutionResult{
			Message: err.Error(),
			Status:  "failed",
		}, err
	}
	rows := slices.Clone(selection.FilteredRows)
	runtimeCtx.Warnings = dedupeStrings(append(runtimeCtx.Warnings, selection.Warnings...))
	if len(rows) == 0 {
		return pipelineNodeExecutionResult{
			Message:       "结果筛选后没有可输出的测速结果，需要人工复核。",
			Metrics:       map[string]any{"input_count": len(sourceRows), "result_count": 0},
			OutputSummary: "0 条结果",
			Status:        "manual_review",
		}, nil
	}

	requireCSV := boolValue(nodeConfig["require_csv"], true)
	exportIfMissing := boolValue(nodeConfig["export_if_missing"], true)
	outputFile := ""
	if runtimeCtx.ProbeResult != nil {
		outputFile = strings.TrimSpace(runtimeCtx.ProbeResult.OutputFile)
	}
	csvWritten := false
	if outputFile != "" {
		if info, err := os.Stat(outputFile); err == nil && !info.IsDir() && info.Size() > 0 {
			csvWritten = true
		}
	}
	exportMessage := ""
	if requireCSV && !csvWritten && exportIfMissing {
		exportResult := a.ExportResultsCSV(map[string]any{
			"config":  runtimeCtx.ConfigSnapshot,
			"results": rows,
			"task_id": runtimeCtx.TaskID,
		})
		runtimeCtx.Warnings = dedupeStrings(append(runtimeCtx.Warnings, exportResult.Warnings...))
		exportMessage = exportResult.Message
		if !exportResult.OK {
			return pipelineNodeExecutionResult{
				Message:       firstNonEmptyString(exportResult.Message, "CSV 导出失败。"),
				Metrics:       map[string]any{"csv_written": false, "result_count": len(rows)},
				Output:        exportResult,
				OutputSummary: fmt.Sprintf("%d 条结果", len(rows)),
				Status:        "failed",
			}, errors.New(firstNonEmptyString(exportResult.Message, "CSV 导出失败"))
		}
		if data := mapValue(exportResult.Data); len(data) > 0 {
			outputFile = strings.TrimSpace(stringValue(firstNonNil(data["path"], data["target_path"], data["targetPath"]), outputFile))
		}
		csvWritten = true
	}

	if requireCSV && !csvWritten {
		return pipelineNodeExecutionResult{
			Message:       "测速结果存在，但 CSV 尚未写入。",
			Metrics:       map[string]any{"csv_written": false, "result_count": len(rows)},
			OutputSummary: fmt.Sprintf("%d 条结果", len(rows)),
			Status:        "manual_review",
		}, nil
	}

	message := fmt.Sprintf("结果检查完成：%d 条结果，CSV 已写入。", len(rows))
	if !requireCSV {
		message = fmt.Sprintf("结果检查完成：%d 条结果。", len(rows))
	} else if exportMessage != "" {
		message = exportMessage
	}
	return pipelineNodeExecutionResult{
		Message: message,
		Metrics: map[string]any{
			"csv_written":  csvWritten,
			"result_count": len(rows),
		},
		Output: map[string]any{
			"output_file":  outputFile,
			"result_count": len(rows),
		},
		OutputSummary: fmt.Sprintf("%d 条结果", len(rows)),
		Status:        "completed",
	}, nil
}

func (a *App) executeRecoveryMarkNode(node appcore.PipelineNode, runtimeCtx *pipelineRuntimeContext) (pipelineNodeExecutionResult, error) {
	nodeConfig := pipelineNodeConfig(node)
	status := firstNonEmptyString(strings.TrimSpace(stringValue(nodeConfig["status"], "")), "manual_review")
	message := firstNonEmptyString(strings.TrimSpace(stringValue(nodeConfig["message"], "")), "需要人工复核。")
	runtimeCtx.Warnings = dedupeStrings(append(runtimeCtx.Warnings, message))
	return pipelineNodeExecutionResult{
		Message:       message,
		Output:        map[string]any{"status": status},
		OutputSummary: status,
		Status:        "completed",
	}, nil
}

func (a *App) executeEndNode(node appcore.PipelineNode, _ *pipelineRuntimeContext) (pipelineNodeExecutionResult, error) {
	nodeConfig := pipelineNodeConfig(node)
	status := normalizePipelineProfileStatus(stringValue(nodeConfig["status"], "completed"))
	message := strings.TrimSpace(stringValue(nodeConfig["message"], ""))
	if message == "" {
		message = pipelineDefaultProfileMessage(status, 0)
	}
	return pipelineNodeExecutionResult{
		Message:       message,
		Output:        map[string]any{"status": status},
		OutputSummary: status,
		Status:        status,
	}, nil
}

func pipelineNextNodeID(node appcore.PipelineNode, edges []appcore.PipelineEdge, outcome string) (string, error) {
	if appcore.NormalizePipelineNodeType(node.NodeType) == appcore.PipelineNodeTypeEnd {
		return "", nil
	}
	if appcore.NormalizePipelineNodeType(node.NodeType) == appcore.PipelineNodeTypeBranch {
		for _, edge := range edges {
			if strings.TrimSpace(edge.Outcome) == strings.TrimSpace(outcome) {
				return strings.TrimSpace(edge.TargetNode), nil
			}
		}
		return "", fmt.Errorf("分支节点 %s 缺少 outcome=%s 的出边", node.ID, outcome)
	}
	if len(edges) == 0 {
		return "", nil
	}
	return strings.TrimSpace(edges[0].TargetNode), nil
}

func (a *App) preparePipelineProbeStage(node appcore.PipelineNode, runtimeCtx *pipelineRuntimeContext) (*pipelineProbeStageState, map[string]any, error) {
	snapshot := pipelineProbeSnapshotForNode(runtimeCtx, node)
	cfg, configWarnings := desktopConfigToProbeConfig(snapshot)
	cfg = applyDesktopExportConfig(cfg, snapshot, runtimeCtx.TaskID, runtimeCtx.Profile.Name)
	sources := pipelineProbeSourcesForNode(runtimeCtx, node)
	prepared := prepareDesktopSources(cfg, sources)
	if len(prepared.FatalErrors) > 0 {
		return nil, snapshot, errors.New(strings.Join(prepared.FatalErrors, "；"))
	}
	preparedSummary := summarizeSource(prepared.Text)
	prepared.Text = strings.Join(preparedSummary.Valid, "\n")
	if strings.TrimSpace(prepared.Text) == "" {
		message := "没有可用的 IP/CIDR/域名输入"
		if len(prepared.Warnings) > 0 {
			message = strings.Join(prepared.Warnings, "；")
		}
		return nil, snapshot, errors.New(message)
	}
	taskContext, portWarnings := probeTaskContextForPorts(cfg, prepared.SourcePorts)
	taskContext.ConfigSource = firstNonEmptyString(runtimeCtx.Payload.ConfigSource, "pipeline")
	prepared.Warnings = append(prepared.Warnings, portWarnings...)
	cfg.IPText = strings.Join(preparedSummary.Valid, ",")
	return &pipelineProbeStageState{
		Config:         cfg,
		ConfigWarnings: configWarnings,
		Prepared:       prepared,
		Source: SourceSummary{
			CandidateCount: preparedSummary.CandidateCount,
			DuplicateCount: preparedSummary.DuplicateCount,
			Duplicates:     preparedSummary.Duplicates,
			Invalid:        preparedSummary.Invalid,
			InvalidCount:   preparedSummary.InvalidCount + prepared.InvalidCount,
			RawLineCount:   preparedSummary.RawLineCount,
			UniqueCount:    preparedSummary.UniqueCount,
			Valid:          preparedSummary.Valid,
			ValidCount:     preparedSummary.ValidCount,
		},
		SourcePorts: prepared.SourcePorts,
		StartedAt:   time.Now(),
		TaskContext: taskContext,
		TestPorts:   pipelineTestPortsForIPs(preparedSummary.Valid, prepared.SourcePorts, cfg.TCPPort, cfg.PortPolicy),
		Warnings:    prepared.Warnings,
	}, snapshot, nil
}

func pipelineExistingProbeStage(runtimeCtx *pipelineRuntimeContext, requiredStage string) (*pipelineProbeStageState, error) {
	if runtimeCtx.ProbeStage == nil {
		return nil, errors.New("缺少上游测速阶段输出")
	}
	for _, stage := range runtimeCtx.ProbeStage.CompletedStages {
		if stage == requiredStage {
			return runtimeCtx.ProbeStage, nil
		}
	}
	return nil, fmt.Errorf("缺少上游 %s 阶段输出", requiredStage)
}

func pipelineTestPortsForIPs(ips []string, sourcePorts map[string]int, globalPort int, portPolicy string) map[string]int {
	groups := probecore.PortGroups(ips, sourcePorts, globalPort, portPolicy)
	result := make(map[string]int, len(ips))
	for _, group := range groups {
		port := group.Port
		if port <= 0 {
			port = globalPort
		}
		for _, ip := range group.IPs {
			result[strings.TrimSpace(ip)] = port
		}
	}
	return result
}

func pipelineRowsFromRawResults(raw []utils.CloudflareIPData, sourcePorts map[string]int, testPorts map[string]int, fallbackPort int) []ProbeRow {
	rows := make([]ProbeRow, 0, len(raw))
	for _, item := range raw {
		ip := item.IP.String()
		testPort := testPorts[strings.TrimSpace(ip)]
		if testPort <= 0 {
			testPort = fallbackPort
		}
		rows = append(rows, probecore.ConvertProbeRow(item, sourcePorts[strings.TrimSpace(ip)], testPort))
	}
	return rows
}

func pipelineEnsureUploadSelection(runtimeCtx *pipelineRuntimeContext, node appcore.PipelineNode) (UploadSelectionResult, error) {
	if output, ok := runtimeCtx.nodeOutput(node.ID); ok {
		existing, ok := output.(UploadSelectionResult)
		if !ok {
			return UploadSelectionResult{}, fmt.Errorf("节点 %s 的缓存输出不是上传筛选结果", node.ID)
		}
		runtimeCtx.FilteredRows = slices.Clone(existing.FilteredRows)
		return existing, nil
	}
	sourceRows := pipelineRowsForNodeSource(runtimeCtx, stringValue(pipelineNodeConfig(node)["source"], ""))
	if len(sourceRows) == 0 {
		return UploadSelectionResult{}, errors.New("缺少可筛选的测速结果")
	}
	metric := "average"
	if runtimeCtx.ProbeResult != nil {
		metric = runtimeCtx.ProbeResult.Config.DownloadSpeedMetric
	}
	selectionSnapshot := pipelineSelectionSnapshotForNode(runtimeCtx, node)
	selection, err := BuildUploadSelection(selectionSnapshot, sourceRows, metric)
	if err != nil {
		return UploadSelectionResult{}, err
	}
	if topN, ok := pipelineTopNOverride(node); ok && topN > 0 {
		selection.FilteredRows = pipelineLimitProbeRows(selection.FilteredRows, topN, metric)
		selection.CloudflareRows = pipelineLimitProbeRows(selection.FilteredRows, topN, metric)
		selection.GitHubRows = pipelineLimitProbeRows(selection.FilteredRows, topN, metric)
	}
	runtimeCtx.putNodeOutput(node.ID, selection)
	runtimeCtx.FilteredRows = slices.Clone(selection.FilteredRows)
	return selection, nil
}

func pipelineRowsForNodeSource(runtimeCtx *pipelineRuntimeContext, source string) []ProbeRow {
	switch strings.ToLower(strings.TrimSpace(source)) {
	case "filtered_rows":
		return slices.Clone(runtimeCtx.FilteredRows)
	case "probe_results":
		if runtimeCtx.ProbeResult != nil && len(runtimeCtx.ProbeResult.Results) > 0 {
			return slices.Clone(runtimeCtx.ProbeResult.Results)
		}
	}
	return nil
}

func pipelineNodeConfig(node appcore.PipelineNode) map[string]any {
	return mapValue(node.Config)
}

func pipelineProbeSnapshotForNode(runtimeCtx *pipelineRuntimeContext, node appcore.PipelineNode) map[string]any {
	snapshot := sanitizeDesktopConfigSnapshot(deepCloneMap(runtimeCtx.ConfigSnapshot))
	nodeConfig := pipelineNodeConfig(node)
	probe := mapValue(snapshot["probe"])
	concurrency := mapValue(probe["concurrency"])
	thresholds := mapValue(probe["thresholds"])
	stageLimits := mapValue(firstNonNil(probe["stage_limits"], probe["stageLimits"]))
	timeouts := mapValue(probe["timeouts"])

	switch appcore.NormalizePipelineNodeAction(node.Action) {
	case appcore.PipelineNodeActionProbeTCP, appcore.PipelineNodeActionProbeTrace, appcore.PipelineNodeActionProbeDownload:
		probe["strategy"] = "full"
		probe["disable_download"] = false
	}
	if value, ok := nodeConfig["concurrency_stage1"]; ok {
		concurrency["stage1"] = intValue(value, 200)
	}
	if value, ok := nodeConfig["concurrency_stage2"]; ok {
		concurrency["stage2"] = intValue(value, 30)
	}
	if value, ok := nodeConfig["concurrency_stage3"]; ok {
		concurrency["stage3"] = intValue(value, 1)
	}
	if value, ok := nodeConfig["tcp_port"]; ok {
		probe["tcp_port"] = intValue(value, 443)
	}
	if value, ok := nodeConfig["port_policy"]; ok {
		probe["port_policy"] = strings.TrimSpace(stringValue(value, ""))
	}
	if value, ok := nodeConfig["ping_times"]; ok {
		probe["ping_times"] = intValue(value, 4)
	}
	if value, ok := nodeConfig["min_delay_ms"]; ok {
		probe["min_delay_ms"] = intValue(value, 0)
	}
	if value, ok := nodeConfig["timeout_stage1_ms"]; ok {
		timeouts["stage1_ms"] = intValue(value, 1000)
	}
	if value, ok := nodeConfig["timeout_stage2_ms"]; ok {
		timeouts["stage2_ms"] = intValue(value, 1000)
	}
	if value, ok := nodeConfig["timeout_stage3_ms"]; ok {
		timeouts["stage3_ms"] = intValue(value, 10000)
	}
	if value, ok := nodeConfig["download_speed_metric"]; ok {
		probe["download_speed_metric"] = strings.TrimSpace(stringValue(value, ""))
	}
	if value, ok := nodeConfig["download_count"]; ok {
		count := intValue(value, 0)
		if count > 0 {
			probe["download_count"] = count
			if _, hasStage3Limit := nodeConfig["stage3_limit"]; !hasStage3Limit {
				stageLimits["stage3"] = count
			}
		}
	}
	if value, ok := nodeConfig["stage3_limit"]; ok {
		count := intValue(value, 0)
		if count > 0 {
			stageLimits["stage3"] = count
		}
	}
	if value, ok := nodeConfig["print_num"]; ok {
		probe["print_num"] = intValue(value, 0)
	}
	if value, ok := nodeConfig["download_get_concurrency"]; ok {
		probe["download_get_concurrency"] = intValue(value, 4)
	}
	if value, ok := nodeConfig["download_time_seconds"]; ok {
		probe["download_time_seconds"] = intValue(value, 4)
	}
	if value, ok := nodeConfig["download_warmup_seconds"]; ok {
		probe["download_warmup_seconds"] = intValue(value, 1)
	}
	if value, ok := nodeConfig["download_speed_sample_interval_ms"]; ok {
		probe["download_speed_sample_interval_ms"] = intValue(value, 500)
	}
	if value, ok := nodeConfig["download_buffer_kb"]; ok {
		probe["download_buffer_kb"] = intValue(value, 256)
	}
	if value, ok := nodeConfig["download_http_protocol"]; ok {
		probe["download_http_protocol"] = strings.TrimSpace(stringValue(value, ""))
	}
	if value, ok := nodeConfig["url"]; ok {
		probe["url"] = strings.TrimSpace(stringValue(value, ""))
	}
	if value, ok := nodeConfig["max_loss_rate"]; ok {
		probe["max_loss_rate"] = floatValue(value, 0)
	}
	if value, ok := nodeConfig["max_tcp_latency_ms"]; ok {
		if value == nil {
			thresholds["max_tcp_latency_ms"] = nil
		} else {
			thresholds["max_tcp_latency_ms"] = intValue(value, 0)
		}
	}
	if value, ok := nodeConfig["max_trace_latency_ms"]; ok {
		if value == nil {
			thresholds["max_http_latency_ms"] = nil
		} else {
			thresholds["max_http_latency_ms"] = intValue(value, 0)
		}
	}
	if value, ok := nodeConfig["min_download_mbps"]; ok {
		thresholds["min_download_mbps"] = floatValue(value, 0)
	}
	if value, ok := nodeConfig["trace_url"]; ok {
		probe["trace_url"] = strings.TrimSpace(stringValue(value, ""))
	}
	if value, ok := nodeConfig["trace_colo_mode"]; ok {
		probe["trace_colo_mode"] = strings.TrimSpace(stringValue(value, ""))
	}
	if value, ok := nodeConfig["source_colo_filter_phase"]; ok {
		probe["source_colo_filter_phase"] = strings.TrimSpace(stringValue(value, ""))
	}
	if value, ok := nodeConfig["httping_status_code"]; ok {
		probe["httping_status_code"] = intValue(value, 0)
	}
	if value, ok := nodeConfig["httping_cf_colo"]; ok {
		probe["httping_cf_colo"] = strings.TrimSpace(stringValue(value, ""))
	}
	if value, ok := nodeConfig["httping_cf_colo_mode"]; ok {
		probe["httping_cf_colo_mode"] = strings.TrimSpace(stringValue(value, ""))
	}

	probe["concurrency"] = concurrency
	probe["thresholds"] = thresholds
	probe["stage_limits"] = stageLimits
	probe["timeouts"] = timeouts
	snapshot["probe"] = probe
	return sanitizeDesktopConfigSnapshot(snapshot)
}

func pipelineProbeSourcesForNode(runtimeCtx *pipelineRuntimeContext, node appcore.PipelineNode) []DesktopSource {
	nodeConfig := pipelineNodeConfig(node)
	sourceMode := strings.ToLower(strings.TrimSpace(stringValue(nodeConfig["source_mode"], "inherit")))

	var sources []DesktopSource
	if sourceMode == "custom" {
		sources = desktopSourcesFromAny(nodeConfig["sources"])
	} else if runtimeCtx.SelectedSources != nil {
		sources = cloneDesktopSources(runtimeCtx.SelectedSources)
	} else {
		sources = desktopSourcesFromAny(runtimeCtx.ConfigSnapshot["sources"])
	}
	if len(sources) == 0 {
		return nil
	}

	overridden := make([]DesktopSource, 0, len(sources))
	sourceIPLimit, hasSourceIPLimit := nodeConfig["source_ip_limit"]
	sourceIPMode, hasSourceIPMode := nodeConfig["source_ip_mode"]
	sourceColoFilter, hasSourceColoFilter := nodeConfig["source_colo_filter"]
	sourceColoFilterMode, hasSourceColoFilterMode := nodeConfig["source_colo_filter_mode"]

	for _, source := range sources {
		next := source
		if hasSourceIPLimit {
			limit := intValue(sourceIPLimit, next.IPLimit)
			if limit > 0 {
				next.IPLimit = limit
			}
		}
		if hasSourceIPMode {
			next.IPMode = strings.TrimSpace(stringValue(sourceIPMode, next.IPMode))
		}
		if hasSourceColoFilter {
			next.ColoFilter = stringValue(sourceColoFilter, next.ColoFilter)
		}
		if hasSourceColoFilterMode {
			next.ColoFilterMode = strings.TrimSpace(stringValue(sourceColoFilterMode, next.ColoFilterMode))
		}
		overridden = append(overridden, next)
	}
	return overridden
}

func pipelineSourceGroupSourcesForNode(runtimeCtx *pipelineRuntimeContext, node appcore.PipelineNode) ([]DesktopSource, error) {
	nodeConfig := pipelineNodeConfig(node)
	profileID := strings.TrimSpace(stringValue(nodeConfig["source_profile_id"], ""))
	selectionMode := strings.ToLower(strings.TrimSpace(stringValue(nodeConfig["source_selection"], appcore.PipelineSourceSelectionEnabled)))
	allSources, err := pipelineSourceGroupAllSources(runtimeCtx, profileID)
	if err != nil {
		return nil, err
	}
	if selectionMode != appcore.PipelineSourceSelectionCustom {
		enabled := make([]DesktopSource, 0, len(allSources))
		for _, source := range allSources {
			if source.Enabled {
				enabled = append(enabled, source)
			}
		}
		return enabled, nil
	}
	sourceIDs := stringSliceValue(nodeConfig["source_ids"])
	selectedIDs := make(map[string]struct{}, len(sourceIDs))
	for _, sourceID := range sourceIDs {
		if strings.TrimSpace(sourceID) != "" {
			selectedIDs[strings.TrimSpace(sourceID)] = struct{}{}
		}
	}
	selected := make([]DesktopSource, 0, len(allSources))
	for _, source := range allSources {
		if _, ok := selectedIDs[strings.TrimSpace(source.ID)]; ok {
			selected = append(selected, source)
		}
	}
	return selected, nil
}

func pipelineSourceGroupAllSources(runtimeCtx *pipelineRuntimeContext, profileID string) ([]DesktopSource, error) {
	if strings.TrimSpace(profileID) == "" {
		return desktopSourcesFromAny(runtimeCtx.ConfigSnapshot["sources"]), nil
	}
	store, err := loadSourceProfileStore()
	if err != nil {
		return nil, fmt.Errorf("读取输入组档案失败：%w", err)
	}
	for _, profile := range store.Items {
		if strings.TrimSpace(profile.ID) == profileID {
			return cloneDesktopSources(profile.Sources), nil
		}
	}
	return nil, fmt.Errorf("输入组档案 %s 不存在。", profileID)
}

func pipelineSelectionSnapshotForNode(runtimeCtx *pipelineRuntimeContext, node appcore.PipelineNode) map[string]any {
	snapshot := sanitizeDesktopConfigSnapshot(deepCloneMap(runtimeCtx.ConfigSnapshot))
	nodeConfig := pipelineNodeConfig(node)
	upload := mapValue(snapshot["upload"])
	sharedFilter := mapValue(upload["shared_filter"])
	cloudflare := mapValue(upload["cloudflare"])
	github := mapValue(upload["github"])

	filterKeys := []string{
		"status",
		"ip_version",
		"max_loss_rate",
		"max_tcp_latency_ms",
		"max_trace_latency_ms",
		"min_download_mbps",
		"colo_allow",
		"colo_deny",
	}
	hasFilterOverride := false
	for _, key := range filterKeys {
		if _, ok := nodeConfig[key]; ok {
			hasFilterOverride = true
			break
		}
	}
	if hasFilterOverride {
		sharedFilter["enabled"] = true
	}
	if value, ok := nodeConfig["status"]; ok {
		sharedFilter["status"] = stringValue(value, "passed")
	}
	if value, ok := nodeConfig["ip_version"]; ok {
		sharedFilter["ip_version"] = stringValue(value, "any")
	}
	if value, ok := nodeConfig["max_loss_rate"]; ok {
		sharedFilter["max_loss_rate"] = floatValue(value, 0)
	}
	if value, ok := nodeConfig["max_tcp_latency_ms"]; ok {
		sharedFilter["max_tcp_latency_ms"] = intValue(value, 0)
	}
	if value, ok := nodeConfig["max_trace_latency_ms"]; ok {
		sharedFilter["max_trace_latency_ms"] = intValue(value, 0)
	}
	if value, ok := nodeConfig["min_download_mbps"]; ok {
		sharedFilter["min_download_mbps"] = floatValue(value, 0)
	}
	if value, ok := nodeConfig["colo_allow"]; ok {
		sharedFilter["colo_allow"] = stringValue(value, "")
	}
	if value, ok := nodeConfig["colo_deny"]; ok {
		sharedFilter["colo_deny"] = stringValue(value, "")
	}
	if topN, ok := pipelineTopNOverride(node); ok {
		switch appcore.NormalizePipelineNodeAction(node.Action) {
		case appcore.PipelineNodeActionDeliverDNS:
			cloudflare["top_n"] = topN
		case appcore.PipelineNodeActionDeliverGitHub:
			github["top_n"] = topN
		default:
			cloudflare["top_n"] = topN
			github["top_n"] = topN
		}
	}

	upload["shared_filter"] = sharedFilter
	upload["cloudflare"] = cloudflare
	upload["github"] = github
	snapshot["upload"] = upload
	return sanitizeDesktopConfigSnapshot(snapshot)
}

func pipelineDNSSnapshotForNode(runtimeCtx *pipelineRuntimeContext, node appcore.PipelineNode) map[string]any {
	snapshot := pipelineSelectionSnapshotForNode(runtimeCtx, node)
	nodeConfig := pipelineNodeConfig(node)
	cloudflare := mapValue(snapshot["cloudflare"])

	if value, ok := nodeConfig["record_name"]; ok {
		recordName := strings.TrimSpace(stringValue(value, ""))
		if recordName != "" {
			cloudflare["record_name"] = recordName
		}
	}
	if value, ok := nodeConfig["record_type"]; ok {
		recordType := strings.ToUpper(strings.TrimSpace(stringValue(value, cloudflareRecordTypeA)))
		if recordType == cloudflareRecordTypeAll {
			cloudflare["record_type"] = cloudflareRecordTypeAll
		} else if recordType == cloudflareRecordTypeAAAA {
			cloudflare["record_type"] = cloudflareRecordTypeAAAA
		} else {
			cloudflare["record_type"] = cloudflareRecordTypeA
		}
	}
	if value, ok := nodeConfig["ttl"]; ok {
		ttl := intValue(value, 0)
		if ttl > 0 {
			cloudflare["ttl"] = ttl
		}
	}
	cloudflare["proxied"] = false
	if value, ok := nodeConfig["comment"]; ok {
		cloudflare["comment"] = stringValue(value, "")
	}

	snapshot["cloudflare"] = cloudflare
	return sanitizeDesktopConfigSnapshot(snapshot)
}

func pipelineTopNOverride(node appcore.PipelineNode) (int, bool) {
	nodeConfig := pipelineNodeConfig(node)
	value, ok := nodeConfig["top_n"]
	if !ok {
		return 0, false
	}
	topN := intValue(value, 0)
	if topN < 0 {
		topN = 0
	}
	return topN, true
}

func pipelineLimitProbeRows(rows []ProbeRow, topN int, metric string) []ProbeRow {
	if len(rows) == 0 {
		return nil
	}
	if topN <= 0 || len(rows) <= topN {
		return slices.Clone(rows)
	}
	selected := probecore.SelectTopProbeRowsByMetric(slices.Clone(rows), topN, metric)
	return slices.Clone(selected)
}

func pipelineProfileFailureStatus(action string, status string) string {
	if appcore.NormalizePipelineNodeAction(action) == appcore.PipelineNodeActionDeliverDNS {
		return "dns_failed"
	}
	status = normalizePipelineProfileStatus(status)
	if status == "completed" {
		return "failed"
	}
	return status
}

func normalizePipelineProfileStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "cancelled":
		return "cancelled"
	case "dns_failed":
		return "dns_failed"
	case "failed":
		return "failed"
	case "manual_review":
		return "manual_review"
	case "partial":
		return "partial"
	case "skipped":
		return "skipped"
	default:
		return "completed"
	}
}

func pipelineNodeFallbackMessage(node appcore.PipelineNode, status string) string {
	switch appcore.NormalizePipelineNodeType(node.NodeType) {
	case appcore.PipelineNodeTypeSource:
		return "输入源组已完成。"
	case appcore.PipelineNodeTypeBranch:
		return "分支节点已完成。"
	case appcore.PipelineNodeTypeDeliver:
		if status == "skipped" {
			return "投递节点已跳过。"
		}
		return "投递节点已完成。"
	case appcore.PipelineNodeTypeEnd:
		return "流程已结束。"
	case appcore.PipelineNodeTypeFilter:
		return "筛选节点已完成。"
	case appcore.PipelineNodeTypeRecovery:
		return "恢复节点已完成。"
	default:
		return "节点已完成。"
	}
}

func pipelineDefaultProfileMessage(status string, resultCount int) string {
	switch normalizePipelineProfileStatus(status) {
	case "dns_failed":
		return "DNS 推送失败。"
	case "failed":
		return "流程执行失败。"
	case "manual_review":
		return "流程已结束，等待人工复核。"
	case "partial":
		return "流程部分完成。"
	case "skipped":
		return "流程已跳过。"
	default:
		return fmt.Sprintf("策略完成，可用结果 %d 条。", resultCount)
	}
}

func pipelineRuntimeResultCount(runtimeCtx *pipelineRuntimeContext, node appcore.PipelineNode) int {
	return len(pipelineRowsForNodeSource(runtimeCtx, stringValue(pipelineNodeConfig(node)["source"], "")))
}

func pipelineResultCount(probeResult *ProbeRunResult, filteredRows []ProbeRow) int {
	if len(filteredRows) > 0 {
		return len(filteredRows)
	}
	if probeResult == nil {
		return 0
	}
	return len(probeResult.Results)
}

func pipelineTargetFromProfile(profile PipelineProfile, templateID string) PipelineTarget {
	return PipelineTarget{
		ConfigSnapshot: deepCloneMap(profile.ConfigSnapshot),
		CreatedAt:      profile.CreatedAt,
		DNSPushPolicy:  profile.DNSPushPolicy,
		Domain:         profile.Domain,
		Enabled:        profile.Enabled,
		ID:             profile.ID,
		Name:           profile.Name,
		Region:         profile.Region,
		TemplateID:     firstNonEmptyString(strings.TrimSpace(templateID), appcore.DefaultPipelineTemplateID),
		UpdatedAt:      profile.UpdatedAt,
	}
}
