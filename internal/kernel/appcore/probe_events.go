package appcore

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/kernel/appcore/contracts"
	"github.com/axuitomo/CFST-GUI/internal/kernel/appcore/diagnostics"
	"github.com/axuitomo/CFST-GUI/internal/kernel/appcore/sources"
	mcisengine "github.com/axuitomo/CFST-GUI/internal/kernel/domain/probe/mcis/engine"
	"github.com/axuitomo/CFST-GUI/internal/kernel/domain/probe/probecore"
	"github.com/axuitomo/CFST-GUI/internal/kernel/domain/probe/task"
	"github.com/axuitomo/CFST-GUI/internal/kernel/infra/utils"
)

// stageRejectReasonEntries 把某阶段的淘汰计数翻成界面可直接显示的中文原因，
// 排序固定为条数降序，保证同一份数据每次渲染顺序一致。
func stageRejectReasonEntries(counts map[string]int) []map[string]any {
	if len(counts) == 0 {
		return nil
	}
	entries := make([]map[string]any, 0, len(counts))
	for reason, count := range counts {
		if count <= 0 {
			continue
		}
		entries = append(entries, map[string]any{"count": count, "label": stageRejectReasonLabel(reason), "reason": reason})
	}
	slices.SortFunc(entries, func(left, right map[string]any) int {
		leftCount, _ := left["count"].(int)
		rightCount, _ := right["count"].(int)
		if leftCount != rightCount {
			return rightCount - leftCount
		}
		leftLabel, _ := left["label"].(string)
		rightLabel, _ := right["label"].(string)
		return strings.Compare(leftLabel, rightLabel)
	})
	return entries
}

// stageRejectReasonLabel 覆盖三个探测阶段的淘汰原因；未知原因原样显示，不丢数据。
func stageRejectReasonLabel(reason string) string {
	switch strings.TrimSpace(reason) {
	// 阶段 1：TCP 测延迟
	case "tcp_no_response":
		return "TCP 无响应"
	case "tcp_delay_above_limit":
		return "TCP 延迟超上限"
	case "tcp_delay_below_min":
		return "TCP 延迟低于下限"
	case "tcp_loss_above_limit":
		return "TCP 丢包超上限"
	case "tcp_worker_panic":
		return "TCP 探测异常退出"
	// 阶段 3：文件测速（status_mismatch / get_error / rate_limited 等与追踪阶段共用）
	case "download_speed_below_min":
		return "下载速度低于阈值"
	case "download_invalid":
		return "测速结果无效"
	case "download_no_valid_measurement":
		return "未取得有效测速样本"
	case "download_no_body_read":
		return "未读到响应正文"
	case "download_interrupted":
		return "测速被中断"
	case "body_disconnected":
		return "响应体提前断开"
	case "download_reconnect":
		return "文件测速断流重连"
	case "get_error":
		return "文件测速请求失败"
	case "too_many_redirects":
		return "重定向次数过多"
	case "request_create_failed":
		return "请求创建失败"
	case "download_worker_panic":
		return "测速探测异常退出"
	case "trace_worker_panic":
		return "追踪探测异常退出"
	case "trace_transport_all_failed":
		return "追踪传输全部失败"
	case "colo_dictionary_unavailable":
		return "COLO 词典不可用"
	// 通用
	case "status_mismatch":
		return "状态码不匹配"
	case "rate_limited":
		return "服务端限流"
	case "forbidden":
		return "HTTP 403 拒绝"
	case "consecutive_failures":
		return "连续失败冷却"
	default:
		return diagnostics.TraceReasonLabel(reason)
	}
}

func (s *Service) emitProbeProgress(taskID, stage string, processed, passed, failed, total, qualifiedTarget int) {
	if !s.runtime.ShouldEmitProgress(stage, processed, total, s.now()) {
		return
	}
	payload := map[string]any{"failed": failed, "passed": passed, "processed": processed, "stage": stage, "total": total}
	if qualifiedTarget > 0 {
		// 文件测速设了「达到成功 IP 数后停止」：真实分母是配额，候选总数会提前收工，进度条按配额走。
		payload["qualified_target"] = qualifiedTarget
	}
	_ = s.PublishProbeEvent(context.Background(), ProbeEvent{Event: "probe.progress", TaskID: taskID, Payload: payload})
	// 进度事件此前只走界面事件流，日志里看不到跑到哪一步；每阶段约 10 条落盘即可对齐时间轴。
	if s.runtime.ShouldLogProgress(stage, processed, total) {
		s.DebugLogger().Event("stage.detail", map[string]any{
			"counts":  map[string]any{"failed": failed, "passed": passed, "processed": processed, "total": total},
			"message": "阶段进度。", "reason": "stage_progress", "stage": stage, "task_id": taskID,
		})
	}
}

func (s *Service) emitMCISProgress(taskID string, source contracts.Source, progress mcisengine.Progress) {
	if strings.TrimSpace(taskID) == "" || !s.runtime.ShouldEmitProgress("stage0_mcis", progress.Completed, progress.Total, s.now()) {
		return
	}
	payload := map[string]any{
		"candidate_count": progress.Candidates, "completed": progress.Completed, "concurrency": progress.Concurrency,
		"elapsed_ms": progress.Elapsed.Milliseconds(), "failed": progress.Failed, "last_colo": progress.LastColo,
		"last_ip": progress.LastIP, "last_ok": progress.LastOK, "source_id": source.ID, "source_name": sources.SourceName(source),
		"stage": "stage0_mcis", "succeeded": progress.Succeeded, "total": progress.Total,
	}
	_ = s.PublishProbeEvent(context.Background(), ProbeEvent{Event: "probe.mcis.progress", TaskID: taskID, Payload: payload})
	if s.runtime.ShouldLogProgress("stage0_mcis", progress.Completed, progress.Total) {
		// 复制一份再补日志字段：payload 已经交给事件流，改原 map 会和消费方抢同一份数据。
		fields := make(map[string]any, len(payload)+3)
		for key, value := range payload {
			fields[key] = value
		}
		fields["message"], fields["reason"], fields["task_id"] = "MCIS 候选测速进度。", "stage_progress", taskID
		s.DebugLogger().Event("stage.detail", fields)
	}
}

// publishEdgeRejected 把 403 / 1034 淘汰推给界面事件流。cfip-log.txt 需要打开 debug 才写，
// 但用户要在日志面板里直接看到这批被边缘拒绝的 IP，所以这条事件不受 debug 开关约束。
//
// 明细先进合并窗口（见 edgeRejectionBatch）：逐颗 IP 各发一条会把探测拖成「每次拒绝一次
// eval + 一次快照落盘」，几千颗 IP 的源就是几千次。合并后每条事件带回一批 items。
func (s *Service) publishEdgeRejected(taskID string, rejection task.EdgeRejection) {
	s.edgeRejections.Add(taskID, map[string]any{
		"ignored":     rejection.Ignored,
		"ip":          rejection.IP,
		"message":     edgeRejectedMessage(rejection),
		"note":        rejection.Note,
		"reason":      rejection.Reason,
		"stage":       rejection.Stage,
		"status_code": http.StatusForbidden,
	}, s.now())
}

func edgeRejectedMessage(rejection task.EdgeRejection) string {
	scope := "文件测速"
	if rejection.Stage == "stage2_trace" {
		scope = "追踪探测"
	}
	message := scope + "遇到 403（" + rejection.Note + "）"
	if rejection.Note == task.EdgeNote1034 {
		message = scope + "遇到 Cloudflare 边缘 IP 限制（403 error code 1034）"
	}
	return message + "，已直接淘汰该 IP：不重试、不计入连续失败冷却。"
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

func (s *Service) logProbePreparationFailure(cfg probecore.ProbeConfig, taskID string, source probecore.SourceSummary, invalidCount int, statuses []contracts.SourceStatus, duration time.Duration, err error) {
	s.DebugLogger().SetEnabled(cfg.Debug)
	closeLog, _, _ := s.configureProbeDebug(cfg, taskID)
	defer closeLog()
	s.debugProbeStart(cfg, taskID, statuses)
	s.DebugLogger().Event("stage.complete", map[string]any{"counts": debugStage0Counts(source, invalidCount), "duration_ms": duration.Milliseconds(), "message": "IP 池生成失败。", "source": debugSourceSummary(source, statuses), "stage": "stage0_pool", "task_id": taskID})
	s.logProbeFailed(taskID, "stage0_pool", s.now().Add(-duration), nil, err, false, map[string]any{"debug_log_path": s.probeDebugLogPath(cfg)})
}

func (s *Service) debugProbeStart(cfg probecore.ProbeConfig, taskID string, statuses []contracts.SourceStatus) {
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

// logSourceObservation 记一条源级明细：合并后的 stage.complete 只剩总量，单源的读取失败原因会被抹平。
func (s *Service) logSourceObservation(taskID string, source contracts.Source, result sources.SourceProcessResult, err error, elapsed time.Duration) {
	fields := map[string]any{
		"counts":      map[string]any{"entries": len(result.Entries), "invalid": result.InvalidCount},
		"duration_ms": elapsed.Milliseconds(),
		"kind":        sources.SourceKind(source),
		"message":     "输入源处理完成。",
		"reason":      "stage_source",
		"source_id":   strings.TrimSpace(source.ID),
		"source_name": sources.SourceName(source),
		"stage":       "stage0_pool",
		"status_text": strings.TrimSpace(result.Status.StatusText),
		"task_id":     taskID,
	}
	if err != nil {
		fields["error"], fields["level"], fields["message"] = err.Error(), "error", "输入源处理失败。"
	}
	if len(result.Warnings) > 0 {
		fields["warnings"] = result.Warnings[:min(len(result.Warnings), 5)]
	}
	s.DebugLogger().Event("stage.detail", fields)
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
	// 用分项求和而不是 CandidateCount：候选池的 CandidateCount 只覆盖聚合条目，
	// 与过滤/非法条数不是同一口径。
	total := source.ValidCount + source.DuplicateCount + invalid
	return map[string]any{"accepted": source.ValidCount, "filtered": source.DuplicateCount, "invalid": invalid, "total": total}
}

func debugSourceSummary(source probecore.SourceSummary, statuses []contracts.SourceStatus) map[string]any {
	return map[string]any{"candidate_count": source.CandidateCount, "duplicate_count": source.DuplicateCount, "invalid_count": source.InvalidCount, "raw_line_count": source.RawLineCount, "source_statuses": statuses, "unique_count": source.UniqueCount, "valid_count": source.ValidCount}
}

func debugProbeConfigSummary(cfg probecore.ProbeConfig) map[string]any {
	return map[string]any{
		"debug_capture_address": cfg.DebugCaptureAddress, "debug_capture_enabled": cfg.DebugCaptureEnabled,
		"debug_log_mode": cfg.DebugLogMode, "debug_log_verbosity": cfg.DebugLogVerbosity,
		"disable_download": cfg.DisableDownload, "download_buffer_kb": cfg.DownloadBufferKB, "download_success_limit": cfg.DownloadSuccessLimit,
		"download_concurrency":     cfg.Stage3Concurrency,
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
