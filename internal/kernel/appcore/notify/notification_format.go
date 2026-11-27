package notify

import (
	"fmt"
	"strings"
	"time"
)

func UploadNotificationText(notification UploadNotification) string {
	lines := uploadNotificationSummaryLines(notification)
	if len(notification.TopEntries) > 0 {
		lines = append(lines, uploadNotificationTopNLines(notification)...)
	}
	if message := strings.TrimSpace(notification.Message); message != "" {
		lines = append(lines, "消息："+truncateTelegramLine(message, 700))
	}
	return strings.Join(lines, "\n")
}

func UploadNotificationSummaryText(notification UploadNotification) string {
	lines := uploadNotificationSummaryLines(notification)
	if message := strings.TrimSpace(notification.Message); message != "" {
		lines = append(lines, "消息："+truncateTelegramLine(message, 700))
	}
	return strings.Join(lines, "\n")
}

func UploadNotificationTopNText(notification UploadNotification) string {
	lines := []string{
		"CFST Top N 上传列表",
		"来源：" + UploadNotificationSourceLabel(notification.Source),
	}
	if taskID := strings.TrimSpace(notification.TaskID); taskID != "" {
		lines = append(lines, "任务："+taskID)
	}
	if createdAt := strings.TrimSpace(notification.CreatedAt); createdAt != "" {
		lines = append(lines, "时间："+notificationTimeLabel(createdAt))
	}
	lines = append(lines, uploadNotificationTopNLines(notification)...)
	return truncateTelegramMessage(strings.Join(lines, "\n"), MaxTelegramMessageLength)
}

func uploadFailureTaskReason(notification UploadNotification) string {
	parts := []string{"上传失败：" + UploadNotificationSourceLabel(notification.Source)}
	for _, provider := range uploadNotificationProviderEntries(notification) {
		if line := uploadFailureProviderLine(provider.Label, provider.Status, provider.UploadCount); line != "" {
			parts = append(parts, line)
		}
	}
	if len(parts) == 1 {
		parts = append(parts, "状态："+UploadNotificationStatusLabel(notification.Status))
	}
	if message := strings.TrimSpace(notification.Message); message != "" {
		parts = append(parts, truncateTelegramLine(message, 160))
	}
	return strings.Join(parts, "；")
}

// uploadNotificationProviderEntries normalizes the provider list, falling back to
// the legacy Cloudflare/GitHub fields stored by older builds.
func uploadNotificationProviderEntries(notification UploadNotification) []UploadProviderResult {
	if len(notification.Providers) > 0 {
		return notification.Providers
	}
	entries := make([]UploadProviderResult, 0, 2)
	if strings.TrimSpace(notification.CloudflareStatus) != "" {
		entries = append(entries, UploadProviderResult{Label: "Cloudflare", Provider: "cloudflare", Status: notification.CloudflareStatus, UploadCount: notification.CloudflareUploadCount})
	}
	if strings.TrimSpace(notification.GitHubStatus) != "" {
		entries = append(entries, UploadProviderResult{Label: "GitHub", Provider: "github", Status: notification.GitHubStatus, UploadCount: notification.GitHubUploadCount})
	}
	return entries
}

func uploadNotificationProviderStatuses(notification UploadNotification) []string {
	entries := uploadNotificationProviderEntries(notification)
	statuses := make([]string, 0, len(entries))
	for _, entry := range entries {
		statuses = append(statuses, entry.Status)
	}
	return statuses
}

func uploadNotificationProviderLines(notification UploadNotification) []string {
	entries := uploadNotificationProviderEntries(notification)
	lines := make([]string, 0, len(entries))
	for _, entry := range entries {
		if strings.TrimSpace(entry.Status) == "" {
			continue
		}
		label := strings.TrimSpace(entry.Label)
		if label == "" {
			label = strings.TrimSpace(entry.Provider)
		}
		if label == "" {
			label = "DNS"
		}
		lines = append(lines, fmt.Sprintf("%s：%s，上传 %d 条", label, UploadNotificationStatusLabel(entry.Status), max(0, entry.UploadCount)))
	}
	return lines
}

func uploadFailureProviderLine(provider string, status string, uploadCount int) string {
	switch normalizeUploadNotificationStatus(status) {
	case UploadNotificationStatusFailed, UploadNotificationStatusPartial:
		return fmt.Sprintf("%s：%s，上传 %d 条", provider, UploadNotificationStatusLabel(status), max(0, uploadCount))
	default:
		return ""
	}
}

func TaskFailureNotificationText(input TaskFailureNotificationInput) string {
	createdAt := input.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	reason := strings.TrimSpace(input.Message)
	if reason == "" {
		reason = "任务异常结束，未提供详细原因。"
	}
	lines := []string{
		"CFST 任务失败",
		"状态：" + UploadNotificationStatusLabel(UploadNotificationStatusFailed),
	}
	if taskID := strings.TrimSpace(input.TaskID); taskID != "" {
		lines = append(lines, "任务："+taskID)
	}
	if stage := strings.TrimSpace(input.Stage); stage != "" {
		lines = append(lines, "阶段："+stage)
	}
	lines = append(lines,
		"原因："+truncateTelegramLine(reason, 300),
		"时间："+notificationTimeLabel(createdAt.Format(time.RFC3339)),
	)
	return truncateTelegramMessage(strings.Join(lines, "\n"), MaxTelegramMessageLength)
}

// UploadNotificationStatusLabel 是所有通知渠道共用的状态文案：图标 + 一句话，
// 让收到通知的人不必先看具体条目就知道这次是成功、部分成功、跳过、失败还是被终止。
func UploadNotificationStatusLabel(status string) string {
	switch normalizeUploadNotificationStatus(status) {
	case UploadNotificationStatusCompleted:
		return "✅ 成功"
	case UploadNotificationStatusFailed:
		return "❌ 失败"
	case UploadNotificationStatusPartial:
		return "⚠️ 部分成功"
	case UploadNotificationStatusAborted:
		return "🚫 熔断/终止"
	case UploadNotificationStatusUnsupported:
		return "❓ 暂不支持"
	default:
		return "⏭️ 跳过"
	}
}

func UploadNotificationSourceLabel(source string) string {
	switch strings.TrimSpace(source) {
	case UploadNotificationSourceManualPush:
		return "手动推送"
	case UploadNotificationSourcePostProbePush:
		return "手动测速后自动上传"
	case UploadNotificationSourceScheduledProbe:
		return "定时任务自动上传"
	default:
		if strings.TrimSpace(source) == "" {
			return "上传任务"
		}
		return strings.TrimSpace(source)
	}
}

func truncateTelegramLine(value string, limit int) string {
	value = strings.TrimSpace(value)
	if limit <= 0 || len([]rune(value)) <= limit {
		return value
	}
	runes := []rune(value)
	return string(runes[:limit]) + "..."
}

func truncateTelegramMessage(value string, limit int) string {
	value = strings.TrimSpace(value)
	if limit <= 0 || len([]rune(value)) <= limit {
		return value
	}
	runes := []rune(value)
	if limit <= 3 {
		return string(runes[:limit])
	}
	return strings.TrimSpace(string(runes[:limit-3])) + "..."
}

func uploadNotificationSummaryLines(notification UploadNotification) []string {
	lines := []string{
		"CFST 上传通知",
		"来源：" + UploadNotificationSourceLabel(notification.Source),
		"状态：" + UploadNotificationStatusLabel(notification.Status),
	}
	lines = append(lines, uploadNotificationProviderLines(notification)...)
	lines = append(lines, uploadNotificationFunnelLines(notification)...)
	if taskID := strings.TrimSpace(notification.TaskID); taskID != "" {
		lines = append(lines, "任务："+taskID)
	}
	if createdAt := strings.TrimSpace(notification.CreatedAt); createdAt != "" {
		lines = append(lines, "时间："+notificationTimeLabel(createdAt))
	}
	if notification.DurationMS > 0 {
		lines = append(lines, "耗时："+notificationDurationLabel(notification.DurationMS))
	}
	return lines
}

func uploadNotificationTopNLines(notification UploadNotification) []string {
	if len(notification.TopEntries) == 0 {
		return nil
	}
	lines := []string{fmt.Sprintf("Top %d：", len(notification.TopEntries))}
	for index, entry := range notification.TopEntries {
		if line := uploadNotificationTopEntryText(index, entry); line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func uploadNotificationTopEntryText(index int, entry UploadNotificationTopEntry) string {
	ip := strings.TrimSpace(entry.IP)
	if ip == "" {
		return ""
	}
	rank := entry.Rank
	if rank <= 0 {
		rank = index + 1
	}
	line := fmt.Sprintf("%d. %s", rank, ip)
	if colo := strings.TrimSpace(entry.Colo); colo != "" && !strings.EqualFold(colo, "N/A") {
		line += " " + strings.ToUpper(colo)
	}
	metrics := make([]string, 0, 3)
	if entry.TCPDelayMS > 0 {
		metrics = append(metrics, fmt.Sprintf("TCP %.2fms", entry.TCPDelayMS))
	}
	if entry.TraceDelayMS > 0 {
		metrics = append(metrics, fmt.Sprintf("追踪 %.2fms", entry.TraceDelayMS))
	}
	if entry.DownloadSpeedMB > 0 {
		metrics = append(metrics, fmt.Sprintf("下载 %.2f MB/s", entry.DownloadSpeedMB))
	}
	if len(metrics) > 0 {
		line += "（" + strings.Join(metrics, "，") + "）"
	}
	return truncateTelegramLine(line, 700)
}

// uploadNotificationFunnelLines 把测速漏斗压成一行：收到通知时先看这四个数，
// 就能区分「链路没跑」「阈值太高」和「结果被上传筛选筛掉」。
func uploadNotificationFunnelLines(notification UploadNotification) []string {
	funnel := notification.Funnel
	if funnel == nil || (funnel.Scanned <= 0 && funnel.Candidates <= 0) {
		return nil
	}
	return []string{fmt.Sprintf("漏斗：扫描 %d → 候选 %d → 通过 %d → 达标 %d", max(0, funnel.Scanned), max(0, funnel.Candidates), max(0, funnel.Passed), max(0, funnel.Qualified))}
}

// NotificationTimeLabel 把 RFC3339 时间转成本机时区的墙上时间：通知面向人看，
// 直接输出 UTC 的 "Z" 会让人对不上自己刚才做了什么。
func NotificationTimeLabel(createdAt string) string {
	createdAt = strings.TrimSpace(createdAt)
	parsed, err := time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return createdAt
	}
	return parsed.Local().Format("2006-01-02 15:04:05 MST")
}

func NotificationDurationLabel(durationMS int64) string {
	seconds := max(0, durationMS) / 1000
	if seconds < 60 {
		return fmt.Sprintf("%d 秒", seconds)
	}
	minutes := seconds / 60
	if minutes < 60 {
		return fmt.Sprintf("%d 分 %d 秒", minutes, seconds%60)
	}
	return fmt.Sprintf("%d 时 %d 分", minutes/60, minutes%60)
}
