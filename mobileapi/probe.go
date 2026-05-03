package mobileapi

import (
	"encoding/csv"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/XIU2/CloudflareSpeedTest/internal/httpcfg"
	"github.com/XIU2/CloudflareSpeedTest/task"
	"github.com/XIU2/CloudflareSpeedTest/utils"
)

type probeEventEnvelope struct {
	Event         string         `json:"event"`
	Payload       map[string]any `json:"payload"`
	SchemaVersion string         `json:"schema_version"`
	Seq           int            `json:"seq"`
	TaskID        string         `json:"task_id"`
	TS            string         `json:"ts"`
}

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
	cfg = s.applyExportConfig(cfg, payload.Config, taskID)
	s.setCurrentTask(taskID)
	defer s.clearCurrentTask(taskID)

	prepared := s.prepareSources(cfg, payload.Sources)
	preparedSummary := summarizeSource(prepared.Text)
	preparedSummary, stage1LimitWarnings := applyStage1CandidateLimit(cfg, preparedSummary)
	prepared.Warnings = append(prepared.Warnings, stage1LimitWarnings...)
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
	if strings.TrimSpace(prepared.Text) == "" && len(prepared.Warnings) > 0 {
		err := errors.New(strings.Join(prepared.Warnings, "；"))
		s.emit(taskID, "probe.failed", map[string]any{"message": err.Error(), "recoverable": false})
		return encodeCommand(commandResultFor("PROBE_FAILED", nil, err.Error(), false, &taskID, prepared.Warnings))
	}

	result, err := s.runProbe(taskID, cfg, configWarnings, prepared.Text, prepared.SourceStatuses)
	if err != nil {
		s.emit(taskID, "probe.failed", map[string]any{"message": err.Error(), "recoverable": false})
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
	s.emit(taskID, "probe.completed", map[string]any{
		"exported": exportedCount,
		"failed":   result.Summary.Failed,
		"failure_summary": map[string]any{
			"duplicate_count": preparedSummary.DuplicateCount,
			"invalid_count":   preparedInvalidCount,
		},
		"passed":       result.Summary.Passed,
		"result_count": len(result.Results),
		"target_path":  eventOutputFile,
	})
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
		if s.pauseCond != nil {
			s.pauseCond.Broadcast()
		}
		s.stateMu.Unlock()
		s.emit(taskID, "probe.cooling", map[string]any{
			"reason":      "已收到暂停请求，任务将在当前安全点暂停。",
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

func (s *Service) runProbe(taskID string, cfg probeConfig, configWarnings []string, sourceText string, sourceStatuses []desktopSourceStatus) (probeRunResult, error) {
	s.runMu.Lock()
	defer s.runMu.Unlock()

	start := time.Now()
	cfg, normalizeWarnings := normalizeProbeConfig(cfg)
	cfg.OutputFile = s.exportPath(cfg.OutputFile)
	configWarnings = append(configWarnings, normalizeWarnings...)
	utils.Debug = cfg.Debug
	closeDebugLog, debugWarnings := s.configureProbeDebugRuntime(cfg)
	utils.SetDebugLogContext(taskID)
	defer closeDebugLog()

	completedStages := make([]string, 0, 4)
	_, source, err := resolveProbeSource(cfg, sourceText)
	if err != nil {
		return probeRunResult{Warnings: configWarnings}, err
	}
	var stage1LimitWarnings []string
	source, stage1LimitWarnings = applyStage1CandidateLimit(cfg, source)
	if source.ValidCount == 0 {
		return probeRunResult{Warnings: configWarnings}, errors.New("没有可用的 IP/CIDR 输入")
	}
	if s.isCancelRequested(taskID) {
		return probeRunResult{Warnings: configWarnings}, errors.New("任务已取消")
	}

	cfg.IPText = strings.Join(source.Valid, ",")
	s.applyProbeConfig(cfg)
	task.InitRandSeed()
	completedStages = append(completedStages, "stage0_pool")

	totalWork := source.ValidCount
	task.LatencyProgressHook = func(processed, passed, failed, _ int) {
		s.emitProgress(taskID, "stage1_tcp", processed, passed, failed, totalWork)
	}
	task.HeadProgressHook = nil
	task.TraceProgressHook = nil
	task.DownloadProgressHook = nil
	task.DownloadSpeedSampleHook = nil
	task.ProbePauseHook = nil
	defer func() {
		task.LatencyProgressHook = nil
		task.HeadProgressHook = nil
		task.TraceProgressHook = nil
		task.DownloadProgressHook = nil
		task.DownloadSpeedSampleHook = nil
		task.ProbePauseHook = nil
	}()
	task.ProbePauseHook = func(stage, ip string) {
		s.waitIfProbePaused(taskID, stage, ip)
	}

	s.emitProgress(taskID, "stage1_tcp", 0, 0, 0, totalWork)
	task.Httping = false
	tcpData := task.NewPing().Run().FilterDelay().FilterLossRate()
	completedStages = append(completedStages, "stage1_tcp")
	if s.isCancelRequested(taskID) {
		return probeRunResult{Warnings: configWarnings}, errors.New("任务已取消")
	}

	traceTotal := task.EstimateTraceProbeCount(len(tcpData))
	task.TraceProgressHook = func(processed, passed, failed, total int) {
		s.emitProgress(taskID, "stage2_trace", processed, passed, failed, total)
	}
	s.emitProgress(taskID, "stage2_trace", 0, 0, 0, traceTotal)
	traceData := task.TestTraceAvailability(tcpData)
	completedStages = append(completedStages, "stage2_trace")
	if s.isCancelRequested(taskID) {
		return probeRunResult{Warnings: configWarnings}, errors.New("任务已取消")
	}

	warnings := append(buildProbeWarnings(source), stage1LimitWarnings...)
	warnings = append(warnings, configWarnings...)
	warnings = append(warnings, debugWarnings...)
	if len(traceData) == 0 && len(tcpData) > 0 {
		warnings = append(warnings, "追踪探测未命中可用候选，已无可导出的结果。")
	}

	resultData := []utils.CloudflareIPData(traceData)
	summaryTotal := source.CandidateCount
	if !cfg.DisableDownload {
		downloadInput := limitPingDelaySet(traceData, cfg.Stage3Limit)
		downloadTotal := estimateDownloadProbeCount(len(downloadInput))
		summaryTotal = downloadTotal
		if downloadTotal > 0 {
			task.DownloadProgressHook = func(processed, qualified, _ int) {
				s.emitProgress(taskID, "stage3_get", processed, qualified, processed-qualified, downloadTotal)
			}
			task.DownloadSpeedSampleHook = func(sample task.DownloadSpeedSample) {
				s.emitSpeed(taskID, sample)
			}
			s.emitProgress(taskID, "stage3_get", 0, 0, 0, downloadTotal)
		}
		speedData := task.TestDownloadSpeed(downloadInput)
		completedStages = append(completedStages, "stage3_get")
		if s.isCancelRequested(taskID) {
			return probeRunResult{Warnings: warnings}, errors.New("任务已取消")
		}
		resultData = []utils.CloudflareIPData(speedData)
	}
	resultData = limitCloudflareResultData(resultData, cfg.PrintNum)

	outputFile := ""
	if len(resultData) > 0 {
		outputFile = cfg.OutputFile
		if outputFile != "" {
			if err := os.MkdirAll(filepath.Dir(outputFile), 0o755); err != nil {
				warnings = append(warnings, fmt.Sprintf("创建导出目录失败：%v", err))
				outputFile = ""
			} else if err := utils.ExportCsv(resultData); err != nil {
				warnings = append(warnings, fmt.Sprintf("结果导出失败：%v", err))
				outputFile = ""
			} else {
				s.emit(taskID, "probe.partial_export", map[string]any{
					"target_path": outputFile,
					"written":     len(resultData),
				})
			}
		}
	}

	rows := make([]probeRow, 0, len(resultData))
	for _, item := range resultData {
		rows = append(rows, convertProbeRow(item))
	}
	_ = completedStages
	return probeRunResult{
		Config:         cfg,
		DurationMS:     time.Since(start).Milliseconds(),
		OutputFile:     outputFile,
		Results:        rows,
		Source:         source,
		SourceStatuses: sourceStatuses,
		StartedAt:      start.Format(time.RFC3339),
		Summary:        summarizeProbeRows(rows, summaryTotal),
		Warnings:       dedupeStrings(warnings),
		SchemaVersion:  schemaVersion,
	}, nil
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
	s.emit(taskID, "probe.progress", map[string]any{
		"failed":    failed,
		"passed":    passed,
		"processed": processed,
		"stage":     stage,
		"total":     total,
	})
}

func (s *Service) emitSpeed(taskID string, sample task.DownloadSpeedSample) {
	s.emit(taskID, "probe.speed", map[string]any{
		"average_speed_mb_s": sample.AverageSpeedMBs,
		"bytes_read":         sample.BytesRead,
		"colo":               sample.Colo,
		"current_speed_mb_s": sample.CurrentSpeedMBs,
		"elapsed_ms":         sample.ElapsedMS,
		"ip":                 sample.IP,
		"stage":              sample.Stage,
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
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, errors.New("结果文件路径为空。")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("读取结果文件失败：%w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("解析结果 CSV 失败：%w", err)
	}
	if len(records) == 0 {
		return nil, errors.New("结果文件为空。")
	}
	header := mobileCSVHeaderIndex(records[0])
	rows := make([]probeResultRow, 0, len(records)-1)
	for _, record := range records[1:] {
		address := mobileCSVField(record, header, "IP 地址", "ip", "address")
		if strings.TrimSpace(address) == "" {
			continue
		}
		colo := strings.TrimSpace(mobileCSVField(record, header, "地区码", "colo"))
		rows = append(rows, probeResultRow{
			Address:        strings.TrimSpace(address),
			Colo:           mobileStringPtrOrNil(colo),
			DownloadMbps:   mobileCSVFloatPtr(mobileCSVField(record, header, "下载速度(MB/s)", "downloadSpeedMb", "download_mbps")),
			ExportStatus:   "exported",
			StageStatus:    "completed",
			TCPLatencyMS:   mobileCSVFloatPtr(mobileCSVField(record, header, "TCP延迟(ms)", "平均延迟", "delayMs", "tcp_latency_ms")),
			TraceLatencyMS: mobileCSVFloatPtr(mobileCSVField(record, header, "追踪延迟(ms)", "traceDelayMs", "trace_latency_ms")),
		})
	}
	if len(rows) == 0 {
		return nil, errors.New("结果文件没有可读取的结果行。")
	}
	return rows, nil
}

func mobileCSVHeaderIndex(header []string) map[string]int {
	index := make(map[string]int, len(header))
	for i, name := range header {
		key := strings.ToLower(strings.TrimSpace(name))
		key = strings.ReplaceAll(key, " ", "")
		index[key] = i
	}
	return index
}

func mobileCSVField(record []string, header map[string]int, names ...string) string {
	for _, name := range names {
		key := strings.ToLower(strings.TrimSpace(name))
		key = strings.ReplaceAll(key, " ", "")
		if index, ok := header[key]; ok && index >= 0 && index < len(record) {
			return record[index]
		}
	}
	return ""
}

func mobileCSVFloatPtr(value string) *float64 {
	value = strings.TrimSpace(value)
	if value == "" || strings.EqualFold(value, "N/A") {
		return nil
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil || parsed <= 0 {
		return nil
	}
	return &parsed
}

func mobileStringPtrOrNil(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" || strings.EqualFold(value, "N/A") {
		return nil
	}
	return &value
}

func (s *Service) setCurrentTask(taskID string) {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	s.currentTaskID = taskID
	s.pauseRequested = false
	s.pausedTaskID = ""
	if s.pauseCond != nil {
		s.pauseCond.Broadcast()
	}
	if s.cancelRequested && s.cancelTaskID == taskID {
		return
	}
	s.cancelTaskID = ""
	s.cancelRequested = false
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

func (s *Service) configureProbeDebugRuntime(cfg probeConfig) (func(), []string) {
	path, err := utils.ConfigureDebugLog(cfg.Debug, s.debugLogPath())
	if err != nil {
		return func() {}, []string{fmt.Sprintf("初始化调试日志失败：%v", err)}
	}
	warnings := make([]string, 0, 2)
	if cfg.Debug && path != "" {
		warnings = append(warnings, fmt.Sprintf("调试日志已写入 %s", path))
	}
	if cfg.Debug && strings.TrimSpace(cfg.DebugCaptureAddress) != "" {
		captureAddress := httpcfg.Resolve("", "", "", cfg.DebugCaptureAddress, true).CaptureAddress
		warnings = append(warnings, fmt.Sprintf("调试模式已将请求拨号目标覆盖为 %s", captureAddress))
	}
	return func() {
		_ = utils.CloseDebugLog()
	}, warnings
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
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	summary := sourceSummary{RawLineCount: len(lines)}
	seen := map[string]struct{}{}
	for _, token := range sourceTokens(raw) {
		summary.CandidateCount++
		normalized, ok := normalizeIPToken(token)
		if !ok {
			summary.Invalid = append(summary.Invalid, token)
			continue
		}
		if _, exists := seen[normalized]; exists {
			summary.Duplicates = append(summary.Duplicates, normalized)
			continue
		}
		seen[normalized] = struct{}{}
		summary.Valid = append(summary.Valid, normalized)
	}
	summary.ValidCount = len(summary.Valid)
	summary.InvalidCount = len(summary.Invalid)
	summary.DuplicateCount = len(summary.Duplicates)
	summary.UniqueCount = summary.ValidCount
	return summary
}

func applyStage1CandidateLimit(cfg probeConfig, source sourceSummary) (sourceSummary, []string) {
	if cfg.Stage1Limit <= 0 || source.ValidCount <= cfg.Stage1Limit {
		return source, nil
	}
	originalCount := source.ValidCount
	source.Valid = append([]string(nil), source.Valid[:cfg.Stage1Limit]...)
	source.ValidCount = len(source.Valid)
	source.UniqueCount = source.ValidCount
	return source, []string{fmt.Sprintf("阶段1候选上限为 %d，已从 %d 条候选中截取前 %d 条进行 TCP 探测。", cfg.Stage1Limit, originalCount, source.ValidCount)}
}

func convertProbeRow(item utils.CloudflareIPData) probeRow {
	lossRate := 0.0
	if item.Sended > 0 {
		lossRate = float64(item.Sended-item.Received) / float64(item.Sended)
	}
	colo := item.Colo
	if colo == "" {
		colo = "N/A"
	}
	return probeRow{
		Colo:            colo,
		DelayMS:         item.Delay.Seconds() * 1000,
		DownloadSpeedMB: item.DownloadSpeed / 1024 / 1024,
		IP:              item.IP.String(),
		LossRate:        lossRate,
		Received:        item.Received,
		Sended:          item.Sended,
		TraceDelayMS:    item.HeadDelay.Seconds() * 1000,
	}
}

func summarizeProbeRows(rows []probeRow, total int) probeSummary {
	summary := probeSummary{Failed: total - len(rows), Passed: len(rows), Total: total}
	if summary.Failed < 0 {
		summary.Failed = 0
	}
	if len(rows) == 0 {
		return summary
	}
	var delay float64
	for _, row := range rows {
		delay += row.DelayMS
	}
	summary.AverageDelayMS = delay / float64(len(rows))
	summary.BestIP = rows[0].IP
	summary.BestSpeedMB = rows[0].DownloadSpeedMB
	return summary
}

func estimateDownloadProbeCount(candidateCount int) int {
	if task.Disable || candidateCount <= 0 {
		return 0
	}
	return candidateCount
}

func limitPingDelaySet(ipSet utils.PingDelaySet, limit int) utils.PingDelaySet {
	if limit <= 0 || len(ipSet) <= limit {
		return ipSet
	}
	return ipSet[:limit]
}

func limitCloudflareResultData(data []utils.CloudflareIPData, limit int) []utils.CloudflareIPData {
	return utils.SelectTopWeightedResults(data, limit)
}

func buildProbeWarnings(source sourceSummary) []string {
	warnings := make([]string, 0)
	if source.InvalidCount > 0 {
		warnings = append(warnings, fmt.Sprintf("已忽略 %d 条非法 IP/CIDR。", source.InvalidCount))
	}
	if source.DuplicateCount > 0 {
		warnings = append(warnings, fmt.Sprintf("已忽略 %d 条重复候选。", source.DuplicateCount))
	}
	return warnings
}
