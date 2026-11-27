package scheduler

import (
	"strconv"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/kernel/appcore/contracts"
	configvalue "github.com/axuitomo/CFST-GUI/internal/kernel/domain/config"
)

func SchedulerStageActive(stage string) bool {
	switch strings.TrimSpace(stage) {
	case "probe", "dns", "github":
		return true
	default:
		return false
	}
}

type SchedulerConfig struct {
	Enabled                    bool     `json:"enabled"`
	IntervalMinutes            int      `json:"interval_minutes"`
	DailyTimes                 []string `json:"daily_times"`
	AutoDNSPush                bool     `json:"auto_dns_push"`
	AutoGitHubExport           bool     `json:"auto_github_export"`
	SkipIfActive               bool     `json:"skip_if_active"`
	ConfigSource               string   `json:"config_source"`
	PostRunSourceProfileAction string   `json:"post_run_source_profile_action"`
}

type SchedulerStatus struct {
	Enabled                 bool                             `json:"enabled"`
	NextRunAt               string                           `json:"next_run_at"`
	LastRunAt               string                           `json:"last_run_at"`
	LastTaskID              string                           `json:"last_task_id"`
	LastProbeStatus         string                           `json:"last_probe_status"`
	LastDNSStatus           string                           `json:"last_dns_status"`
	LastGitHubStatus        string                           `json:"last_github_status"`
	LastMessage             string                           `json:"last_message"`
	WorkflowStage           string                           `json:"workflow_stage"`
	ConfigSource            string                           `json:"config_source"`
	LastSourceProfileAction string                           `json:"last_source_profile_action"`
	UploadInputCount        int                              `json:"upload_input_count"`
	UploadFilteredCount     int                              `json:"upload_filtered_count"`
	CloudflareUploadCount   int                              `json:"cloudflare_upload_count"`
	GitHubUploadCount       int                              `json:"github_upload_count"`
	DNSProviders            []contracts.UploadProviderResult `json:"dns_providers,omitempty"`
	UploadNotification      *contracts.UploadNotification    `json:"upload_notification,omitempty"`
}

type SchedulerConfigSelection struct {
	Snapshot map[string]any
	Source   string
}

func SchedulerConfigFromSnapshot(snapshot map[string]any, defaults SchedulerConfig) SchedulerConfig {
	raw, _ := snapshot["scheduler"].(map[string]any)
	return SchedulerConfig{
		Enabled:                    configvalue.Bool(raw["enabled"], defaults.Enabled),
		IntervalMinutes:            max(0, configvalue.Int(configvalue.FirstNonNil(raw["interval_minutes"], raw["intervalMinutes"]), defaults.IntervalMinutes)),
		DailyTimes:                 schedulerStringSlice(configvalue.FirstNonNil(raw["daily_times"], raw["dailyTimes"]), defaults.DailyTimes),
		AutoDNSPush:                configvalue.Bool(configvalue.FirstNonNil(raw["auto_dns_push"], raw["autoDnsPush"]), defaults.AutoDNSPush),
		AutoGitHubExport:           configvalue.Bool(configvalue.FirstNonNil(raw["auto_github_export"], raw["autoGithubExport"]), defaults.AutoGitHubExport),
		SkipIfActive:               configvalue.Bool(configvalue.FirstNonNil(raw["skip_if_active"], raw["skipIfActive"]), defaults.SkipIfActive),
		ConfigSource:               schedulerString(configvalue.FirstNonNil(raw["config_source"], raw["configSource"]), defaults.ConfigSource),
		PostRunSourceProfileAction: schedulerString(configvalue.FirstNonNil(raw["post_run_source_profile_action"], raw["postRunSourceProfileAction"]), defaults.PostRunSourceProfileAction),
	}
}

type SchedulerTimingConfig struct {
	Enabled         bool
	IntervalMinutes int
	DailyTimes      []string
}

func NextSchedulerRun(now, lastRun time.Time, cfg SchedulerTimingConfig) time.Time {
	if !cfg.Enabled {
		return time.Time{}
	}
	var next time.Time
	if cfg.IntervalMinutes > 0 {
		interval := time.Duration(cfg.IntervalMinutes) * time.Minute
		candidate := now.Add(interval)
		if !lastRun.IsZero() {
			candidate = lastRun.Add(interval)
			for !candidate.After(now) {
				candidate = candidate.Add(interval)
			}
		}
		next = earlierSchedulerTime(next, candidate)
	}
	for _, raw := range cfg.DailyTimes {
		hour, minute, second, ok := ParseDailySchedulerTime(raw)
		if !ok {
			continue
		}
		candidate := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, second, 0, now.Location())
		if !candidate.After(now) {
			candidate = candidate.Add(24 * time.Hour)
		}
		next = earlierSchedulerTime(next, candidate)
	}
	return next
}

func ParseDailySchedulerTime(raw string) (int, int, int, bool) {
	parts := strings.Split(strings.ReplaceAll(strings.TrimSpace(raw), "：", ":"), ":")
	if len(parts) < 2 || len(parts) > 3 {
		return 0, 0, 0, false
	}
	hour := parseSchedulerInt(parts[0], -1)
	minute := parseSchedulerInt(parts[1], -1)
	second := 0
	if len(parts) == 3 {
		second = parseSchedulerInt(parts[2], -1)
	}
	if hour < 0 || hour > 23 || minute < 0 || minute > 59 || second < 0 || second > 59 {
		return 0, 0, 0, false
	}
	return hour, minute, second, true
}

func SchedulerCompletionMessage(prefix, dnsStatus, githubStatus string) string {
	base := ""
	if strings.TrimSpace(prefix) != "" {
		base = strings.TrimSpace(prefix) + " "
	}
	switch githubStatus {
	case "completed":
		switch dnsStatus {
		case "completed":
			return base + "定时测速、DNS 推送与 GitHub 导出流程已完成。"
		case "partial":
			return base + "定时测速与 GitHub 导出流程已完成，DNS 推送部分完成。"
		case "failed":
			return base + "定时测速与 GitHub 导出流程已完成，DNS 推送失败。"
		case "skipped":
			return base + "定时测速与 GitHub 导出流程已完成，DNS 推送已跳过。"
		default:
			return base + "定时测速与 GitHub 导出流程已完成。"
		}
	case "failed":
		switch dnsStatus {
		case "completed":
			return base + "定时测速与 DNS 推送流程已完成，GitHub 导出失败。"
		case "partial":
			return base + "定时测速流程已完成，DNS 推送部分完成，GitHub 导出失败。"
		case "failed":
			return base + "定时测速流程已完成，DNS 推送与 GitHub 导出失败。"
		case "skipped":
			return base + "定时测速流程已完成，DNS 推送已跳过，GitHub 导出失败。"
		default:
			return base + "定时测速流程已完成，GitHub 导出失败。"
		}
	case "skipped":
		switch dnsStatus {
		case "completed":
			return base + "定时测速与 DNS 推送流程已完成，GitHub 导出已跳过。"
		case "partial":
			return base + "定时测速流程已完成，DNS 推送部分完成，GitHub 导出已跳过。"
		case "failed":
			return base + "定时测速流程已完成，DNS 推送失败，GitHub 导出已跳过。"
		case "skipped":
			return base + "定时测速流程已完成，DNS 推送与 GitHub 导出已跳过。"
		default:
			return base + "定时测速流程已完成，GitHub 导出已跳过。"
		}
	default:
		switch dnsStatus {
		case "completed":
			return base + "定时测速与 DNS 推送流程已完成。"
		case "partial":
			return base + "定时测速流程已完成，DNS 推送部分完成。"
		case "failed":
			return base + "定时测速流程已完成，DNS 推送失败。"
		default:
			return base + "定时测速流程已完成。"
		}
	}
}

func ShouldOverwriteSchedulerDNSMessage(dnsStatus, githubStatus string, autoDNSPush, autoGitHubExport bool) bool {
	if !autoDNSPush || (dnsStatus != "failed" && dnsStatus != "partial") {
		return true
	}
	return autoGitHubExport && githubStatus == "failed"
}

func earlierSchedulerTime(current, candidate time.Time) time.Time {
	if candidate.IsZero() {
		return current
	}
	if current.IsZero() || candidate.Before(current) {
		return candidate
	}
	return current
}

func parseSchedulerInt(raw string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return fallback
	}
	return value
}

func schedulerString(value any, fallback string) string {
	text, ok := value.(string)
	if !ok || strings.TrimSpace(text) == "" {
		return fallback
	}
	return strings.TrimSpace(text)
}

func schedulerStringSlice(value any, fallback []string) []string {
	values := make([]string, 0)
	switch typed := value.(type) {
	case []string:
		for _, item := range typed {
			values = append(values, splitSchedulerList(item)...)
		}
	case []any:
		for _, item := range typed {
			if text, ok := item.(string); ok {
				values = append(values, splitSchedulerList(text)...)
			}
		}
	case string:
		values = append(values, splitSchedulerList(typed)...)
	}
	if len(values) == 0 && value == nil {
		return append([]string(nil), fallback...)
	}
	return values
}

func splitSchedulerList(value string) []string {
	fields := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ';' || r == '，' || r == '；' || r == '、' || r == '\n' || r == '\r' || r == '\t' || r == ' '
	})
	result := make([]string, 0, len(fields))
	for _, field := range fields {
		if text := strings.TrimSpace(field); text != "" {
			result = append(result, text)
		}
	}
	return result
}
