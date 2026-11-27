package notify

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/kernel/domain/probe/probecore"
)

const (
	UploadNotificationSourceManualPush     = "manual_push"
	UploadNotificationSourcePostProbePush  = "post_probe_push"
	UploadNotificationSourceScheduledProbe = "scheduled_probe"

	UploadNotificationProviderCloudflare = "cloudflare"
	UploadNotificationProviderGitHub     = "github"

	// UploadNotificationStatusAborted 表示任务被终止或冷却熔断，上传链路没有跑完：
	// 与 skipped（链路没有需要上传的结果）和 failed（跑完但报错）都不同。
	UploadNotificationStatusAborted     = "aborted"
	UploadNotificationStatusCompleted   = "completed"
	UploadNotificationStatusFailed      = "failed"
	UploadNotificationStatusPartial     = "partial"
	UploadNotificationStatusSkipped     = "skipped"
	UploadNotificationStatusUnsupported = "unsupported"
)

type UploadNotificationInput struct {
	Cloudflare *UploadProviderReport
	CreatedAt  time.Time
	DurationMS int64
	Funnel     *UploadNotificationFunnel
	GitHub     *UploadProviderReport
	Message    string
	Providers  []UploadProviderResult
	Source     string
	TaskID     string
	TopEntries []UploadNotificationTopEntry
}

type TaskFailureNotificationInput struct {
	CreatedAt time.Time
	Message   string
	Stage     string
	TaskID    string
}

type CommandResultUploadNotificationInput struct {
	CreatedAt  time.Time
	Provider   string
	Result     CommandResult
	Source     string
	TaskID     string
	TopEntries []UploadNotificationTopEntry
}

func BuildUploadNotification(input UploadNotificationInput) UploadNotification {
	createdAt := input.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	notification := UploadNotification{
		CreatedAt:  createdAt.Format(time.RFC3339),
		DurationMS: max(0, input.DurationMS),
		Funnel:     cloneUploadNotificationFunnel(input.Funnel),
		Message:    strings.TrimSpace(input.Message),
		Source:     strings.TrimSpace(input.Source),
		TaskID:     strings.TrimSpace(input.TaskID),
		Status:     UploadNotificationStatusSkipped,
		TopEntries: cloneUploadNotificationTopEntries(input.TopEntries),
	}
	if input.Cloudflare != nil {
		notification.CloudflareStatus = normalizeUploadNotificationStatus(input.Cloudflare.Status)
		notification.CloudflareUploadCount = max(0, input.Cloudflare.UploadCount)
	}
	if input.GitHub != nil {
		notification.GitHubStatus = normalizeUploadNotificationStatus(input.GitHub.Status)
		notification.GitHubUploadCount = max(0, input.GitHub.UploadCount)
	}
	for _, provider := range input.Providers {
		status := normalizeUploadNotificationStatus(provider.Status)
		if status == "" {
			continue
		}
		provider.Status = status
		provider.UploadCount = max(0, provider.UploadCount)
		notification.Providers = append(notification.Providers, provider)
	}
	notification.Status = uploadNotificationOverallStatus(uploadNotificationProviderStatuses(notification)...)
	if notification.Message == "" {
		notification.Message = uploadNotificationDefaultMessage(notification)
	}
	return notification
}

func BuildUploadNotificationFromCommandResult(input CommandResultUploadNotificationInput) UploadNotification {
	report := UploadProviderReportFromCommandResult(input.Provider, input.Result)
	notificationInput := UploadNotificationInput{
		CreatedAt:  input.CreatedAt,
		Message:    input.Result.Message,
		Source:     input.Source,
		TaskID:     input.TaskID,
		TopEntries: input.TopEntries,
	}
	switch input.Provider {
	case UploadNotificationProviderCloudflare:
		notificationInput.Cloudflare = &report
	case UploadNotificationProviderGitHub:
		notificationInput.GitHub = &report
	}
	return BuildUploadNotification(notificationInput)
}

func BuildPostProbeNoRowsUploadNotification(cfg PostProbePushConfig, providers []UploadProviderResult, funnel *UploadNotificationFunnel, durationMS int64, taskID string) *UploadNotification {
	var githubReport *UploadProviderReport
	if cfg.GitHubEnabled {
		githubReport = &UploadProviderReport{Status: UploadNotificationStatusSkipped}
	}
	if len(providers) == 0 && githubReport == nil {
		return nil
	}
	notification := BuildUploadNotification(UploadNotificationInput{
		CreatedAt:  time.Now(),
		DurationMS: durationMS,
		Funnel:     funnel,
		GitHub:     githubReport,
		Message:    "没有可上传结果，测速后自动上传已跳过。",
		Providers:  providers,
		Source:     UploadNotificationSourcePostProbePush,
		TaskID:     taskID,
	})
	return &notification
}

func UploadNotificationHasFailure(notification UploadNotification) bool {
	switch normalizeUploadNotificationStatus(notification.Status) {
	case UploadNotificationStatusFailed, UploadNotificationStatusPartial:
		return true
	}
	for _, status := range uploadNotificationProviderStatuses(notification) {
		switch normalizeUploadNotificationStatus(status) {
		case UploadNotificationStatusFailed, UploadNotificationStatusPartial:
			return true
		}
	}
	return false
}

func TaskFailureNotificationInputFromUploadNotification(notification UploadNotification) TaskFailureNotificationInput {
	createdAt, _ := time.Parse(time.RFC3339, strings.TrimSpace(notification.CreatedAt))
	return TaskFailureNotificationInput{
		CreatedAt: createdAt,
		Message:   uploadFailureTaskReason(notification),
		Stage:     "upload",
		TaskID:    notification.TaskID,
	}
}

func UploadProviderReportFromCommandResult(provider string, result CommandResult) UploadProviderReport {
	status := UploadNotificationStatusCompleted
	if !result.OK {
		status = UploadNotificationStatusFailed
		if strings.Contains(result.Code, "INPUT_EMPTY") {
			status = UploadNotificationStatusSkipped
		}
	}
	if strings.Contains(result.Code, "PARTIAL") {
		status = UploadNotificationStatusPartial
	}
	return UploadProviderReport{
		Status:      status,
		UploadCount: uploadNotificationCountFromCommandData(provider, result.Data),
	}
}

func CommandResultDataWithUploadNotification(data any, notification UploadNotification) any {
	mapped := commandResultDataMap(data)
	if len(mapped) == 0 {
		mapped = map[string]any{}
	}
	mapped["upload_notification"] = notification
	return mapped
}

func CommandResultTaskID(payload map[string]any, result CommandResult) string {
	if result.TaskID != nil {
		return strings.TrimSpace(*result.TaskID)
	}
	return strings.TrimSpace(stringValue(firstNonNil(payload["task_id"], payload["taskId"]), ""))
}

func UploadNotificationTriggerFromPayload(payload map[string]any) string {
	return strings.TrimSpace(stringValue(firstNonNil(payload["notification_trigger"], payload["notificationTrigger"]), ""))
}

func UploadNotificationEventPayload(notification UploadNotification) map[string]any {
	raw, err := json.Marshal(notification)
	if err != nil {
		return uploadNotificationFallbackPayload(notification)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil || payload == nil {
		return uploadNotificationFallbackPayload(notification)
	}
	return payload
}

func BuildUploadNotificationTopEntries(rows []probecore.ProbeRow, limit int, metric string) []UploadNotificationTopEntry {
	limit = normalizeTelegramTopN(limit)
	if limit <= 0 || len(rows) == 0 {
		return nil
	}
	cloned := make([]probecore.ProbeRow, len(rows))
	copy(cloned, rows)
	topRows := probecore.SelectTopProbeRowsByMetric(cloned, limit, metric)
	entries := make([]UploadNotificationTopEntry, 0, len(topRows))
	for _, row := range topRows {
		ip := strings.TrimSpace(row.IP)
		if ip == "" {
			continue
		}
		entries = append(entries, UploadNotificationTopEntry{
			Colo:            strings.TrimSpace(row.Colo),
			DownloadSpeedMB: uploadNotificationSpeedForMetric(row, metric),
			IP:              ip,
			Rank:            len(entries) + 1,
			TCPDelayMS:      row.DelayMS,
			TraceDelayMS:    row.TraceDelayMS,
		})
	}
	return entries
}

func BuildUploadNotificationTopEntriesForSnapshot(snapshot map[string]any, rows []probecore.ProbeRow, metric string) []UploadNotificationTopEntry {
	cfg := TelegramNotificationConfigFromSnapshot(snapshot)
	if !cfg.IncludeTopN {
		return nil
	}
	return BuildUploadNotificationTopEntries(rows, cfg.TopN, metric)
}

func normalizeUploadNotificationStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case UploadNotificationStatusCompleted, "ok", "success", "succeeded":
		return UploadNotificationStatusCompleted
	case UploadNotificationStatusFailed, "error":
		return UploadNotificationStatusFailed
	case UploadNotificationStatusPartial:
		return UploadNotificationStatusPartial
	case UploadNotificationStatusUnsupported:
		return UploadNotificationStatusUnsupported
	case UploadNotificationStatusAborted, "cancelled", "canceled", "terminated", "abort":
		return UploadNotificationStatusAborted
	default:
		return UploadNotificationStatusSkipped
	}
}

func uploadNotificationOverallStatus(statuses ...string) string {
	success := 0
	failed := 0
	aborted := 0
	for _, status := range statuses {
		switch normalizeUploadNotificationStatus(status) {
		case UploadNotificationStatusCompleted:
			success++
		case UploadNotificationStatusPartial:
			success++
			failed++
		case UploadNotificationStatusFailed:
			failed++
		case UploadNotificationStatusAborted:
			aborted++
		}
	}
	switch {
	case success > 0 && (failed+aborted) > 0:
		return UploadNotificationStatusPartial
	case failed > 0:
		return UploadNotificationStatusFailed
	case aborted > 0:
		return UploadNotificationStatusAborted
	case success > 0:
		return UploadNotificationStatusCompleted
	default:
		return UploadNotificationStatusSkipped
	}
}

func uploadNotificationDefaultMessage(notification UploadNotification) string {
	switch notification.Status {
	case UploadNotificationStatusCompleted:
		return "上传流程已完成。"
	case UploadNotificationStatusPartial:
		return "上传流程部分完成。"
	case UploadNotificationStatusFailed:
		return "上传流程失败。"
	case UploadNotificationStatusAborted:
		return "任务被终止或熔断，上传流程没有跑完。"
	default:
		return "没有执行上传或没有可上传结果。"
	}
}

func uploadNotificationSpeedForMetric(row probecore.ProbeRow, metric string) float64 {
	switch strings.ToLower(strings.TrimSpace(metric)) {
	case "max", "peak", "highest":
		return row.MaxDownloadSpeedMB
	default:
		return row.DownloadSpeedMB
	}
}

func cloneUploadNotificationTopEntries(entries []UploadNotificationTopEntry) []UploadNotificationTopEntry {
	if len(entries) == 0 {
		return nil
	}
	cloned := make([]UploadNotificationTopEntry, len(entries))
	copy(cloned, entries)
	return cloned
}

// UploadNotificationFunnelForProbe 汇总一次测速的漏斗：扫描行数取输入源原始行数，
// 达标数取上传筛选后的条数，避免通知只有结论没有过程。
func UploadNotificationFunnelForProbe(source probecore.SourceSummary, summary probecore.ProbeSummary, qualified int) *UploadNotificationFunnel {
	candidates := source.ValidCount
	if candidates <= 0 {
		candidates = summary.Total
	}
	scanned := source.RawLineCount
	if scanned < candidates {
		scanned = candidates
	}
	return &UploadNotificationFunnel{
		Candidates: max(0, candidates),
		Passed:     max(0, summary.Passed),
		Qualified:  max(0, qualified),
		Scanned:    max(0, scanned),
	}
}

func cloneUploadNotificationFunnel(funnel *UploadNotificationFunnel) *UploadNotificationFunnel {
	if funnel == nil {
		return nil
	}
	cloned := *funnel
	return &cloned
}

func uploadNotificationCountFromCommandData(provider string, data any) int {
	mapped := commandResultDataMap(data)
	switch provider {
	case UploadNotificationProviderGitHub:
		return intValue(firstNonNil(mapped["written_rows"], mapped["writtenRows"]), 0)
	default:
		return intValue(firstNonNil(mapped["upload_count"], mapped["uploadCount"], mapped["cloudflare_count"], mapped["cloudflareCount"]), 0)
	}
}

func commandResultDataMap(data any) map[string]any {
	if data == nil {
		return map[string]any{}
	}
	if mapped, ok := data.(map[string]any); ok {
		return cloneAnyMap(mapped)
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return map[string]any{"result": data}
	}
	var mapped map[string]any
	if err := json.Unmarshal(raw, &mapped); err != nil || mapped == nil {
		return map[string]any{"result": data}
	}
	return mapped
}

func uploadNotificationFallbackPayload(notification UploadNotification) map[string]any {
	return map[string]any{
		"message": notification.Message,
		"source":  notification.Source,
		"status":  notification.Status,
		"task_id": notification.TaskID,
	}
}
