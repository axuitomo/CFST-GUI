package mobileapi

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/appcore"
)

const (
	defaultMobileSchedulerRunMode = "probe"
	mobileSchedulerConfigSource   = "saved"
)

func (s *Service) LoadSchedulerStatus() string {
	status := s.currentSchedulerStatus()
	return encodeCommand(commandResultFor("SCHEDULER_STATUS_READY", status, "Android 定时任务状态已读取。", true, nil, nil))
}

func (s *Service) RunScheduledProbe(payloadJSON string) (response string) {
	payload, _ := decodeObject(payloadJSON)
	now := time.Now()
	taskID := strings.TrimSpace(stringValue(firstNonNil(payload["task_id"], payload["taskId"]), ""))
	if taskID == "" {
		taskID = "scheduled-" + now.Format("20060102-150405")
	}
	snapshot, err := s.loadConfigSnapshotFromDisk()
	if err != nil {
		status := s.currentSchedulerStatus()
		status.NextRunAt = ""
		status.LastRunAt = now.Format(time.RFC3339)
		status.LastTaskID = taskID
		status.LastProbeStatus = "failed"
		status.LastDNSStatus = ""
		status.LastGitHubStatus = ""
		status.LastMessage = fmt.Sprintf("读取定时任务配置失败：%v", err)
		status.WorkflowStage = "load_config_failed"
		status.ConfigSource = mobileSchedulerConfigSource
		clearMobileSchedulerUploadProgress(&status)
		_ = s.writeSchedulerStatus(status)
		return encodeCommand(commandResultFor("SCHEDULER_RUN_FAILED", status, status.LastMessage, false, &taskID, nil))
	}
	cfg := mobileSchedulerConfigFromSnapshot(snapshot)
	notifyUpload := false
	var notificationTopEntries []appcore.UploadNotificationTopEntry
	defer func() {
		if !notifyUpload {
			return
		}
		status, warnings := s.recordSchedulerUploadNotification(snapshot, appcore.UploadNotificationSourceScheduledProbe, taskID, cfg.AutoDNSPush, cfg.AutoGitHubExport, notificationTopEntries)
		command := decodeCommandResult(response)
		if command.Code == "" {
			return
		}
		command.Data = status
		command.Warnings = dedupeStrings(append(command.Warnings, warnings...))
		response = encodeCommand(command)
	}()
	status := s.currentSchedulerStatus()
	status.Enabled = cfg.Enabled
	status.LastRunAt = now.Format(time.RFC3339)
	status.LastTaskID = taskID
	status.LastProbeStatus = "running"
	status.LastDNSStatus = "skipped"
	status.LastGitHubStatus = "skipped"
	status.LastMessage = "Android 定时测速开始执行。"
	status.WorkflowStage = "probe"
	status.ConfigSource = mobileSchedulerConfigSource
	clearMobileSchedulerUploadProgress(&status)
	_ = s.writeSchedulerStatus(status)
	if !cfg.Enabled {
		status.NextRunAt = ""
		status.LastProbeStatus = "skipped"
		status.LastMessage = "Android 定时任务未启用，本次已跳过。"
		status.WorkflowStage = "skipped"
		_ = s.writeSchedulerStatus(status)
		return encodeCommand(commandResultFor("SCHEDULER_RUN_SKIPPED", status, status.LastMessage, true, &taskID, nil))
	}
	if cfg.SkipIfActive && s.hasActiveTask() {
		status.LastProbeStatus = "skipped"
		status.LastMessage = "已有探测任务运行或暂停，本次 Android 定时任务已跳过。"
		status.WorkflowStage = "skipped"
		rearmMobileSchedulerStatus(&status, cfg)
		_ = s.writeSchedulerStatus(status)
		return encodeCommand(commandResultFor("SCHEDULER_RUN_SKIPPED", status, status.LastMessage, true, &taskID, nil))
	}

	notifyUpload = cfg.AutoDNSPush || cfg.AutoGitHubExport
	resultCommand := decodeCommandResult(s.RunProbe(encodeJSON(map[string]any{
		"config":                  snapshot,
		"config_source":           mobileSchedulerConfigSource,
		"disable_post_probe_push": true,
		"sources":                 firstNonNil(snapshot["sources"], []any{}),
		"task_id":                 taskID,
	})))
	status = s.currentSchedulerStatus()
	if !resultCommand.OK {
		status.LastProbeStatus = "failed"
		status.LastDNSStatus = mobileSchedulerDownstreamStatusAfterProbeFailure(cfg.AutoDNSPush)
		status.LastGitHubStatus = mobileSchedulerDownstreamStatusAfterProbeFailure(cfg.AutoGitHubExport)
		status.LastMessage = resultCommand.Message
		status.WorkflowStage = "probe_failed"
		rearmMobileSchedulerStatus(&status, cfg)
		_ = s.writeSchedulerStatus(status)
		return encodeCommand(commandResultFor("SCHEDULER_RUN_FAILED", status, status.LastMessage, false, &taskID, resultCommand.Warnings))
	}

	probeResult := mobileProbeRunResultFromAny(resultCommand.Data)
	rows := probeResult.Results
	metric := probeResult.Config.DownloadSpeedMetric
	selection, selectErr := appcore.BuildUploadSelectionWithColoPaths(snapshot, rows, metric, s.coloDictionaryPaths())
	if selectErr == nil {
		status.UploadInputCount = len(selection.InputRows)
		status.UploadFilteredCount = len(selection.FilteredRows)
		status.CloudflareUploadCount = len(selection.CloudflareRows)
		status.GitHubUploadCount = len(selection.GitHubRows)
		notificationTopEntries = appcore.BuildUploadNotificationTopEntriesForSnapshot(snapshot, selection.FilteredRows, metric)
	}
	status.LastProbeStatus = "completed"
	status.LastMessage = fmt.Sprintf("Android 定时测速完成，结果 %d 条。", len(rows))
	status.WorkflowStage = "post_run"

	if selectErr != nil {
		status.LastProbeStatus = "failed"
		if cfg.AutoDNSPush {
			status.LastDNSStatus = "failed"
		}
		if cfg.AutoGitHubExport {
			status.LastGitHubStatus = "failed"
		}
		status.LastMessage = fmt.Sprintf("上传筛选失败：%v", selectErr)
		status.WorkflowStage = "upload_selection_failed"
		rearmMobileSchedulerStatus(&status, cfg)
		_ = s.writeSchedulerStatus(status)
		return encodeCommand(commandResultFor("SCHEDULER_RUN_FAILED", status, status.LastMessage, false, &taskID, nil))
	} else {
		if cfg.AutoDNSPush {
			status.WorkflowStage = "dns"
			status.CloudflareUploadCount = len(selection.CloudflareRows)
			if len(rows) == 0 {
				status.LastDNSStatus = "skipped"
			} else {
				dnsCommand := decodeCommandResult(s.PushCloudflareDNSRecords(encodeJSON(map[string]any{
					"config":  snapshot,
					"results": rows,
				})))
				if dnsCommand.OK {
					status.LastDNSStatus = "completed"
				} else {
					status.LastDNSStatus = mobileSchedulerDNSStatusFromCommand(dnsCommand)
					status.LastMessage = dnsCommand.Message
				}
				if uploadCount := intValue(mapValue(dnsCommand.Data)["upload_count"], -1); uploadCount >= 0 {
					status.CloudflareUploadCount = uploadCount
				}
			}
		}
		if cfg.AutoGitHubExport {
			status.WorkflowStage = "github"
			if !appcore.GitHubProviderEnabledFromSnapshot(snapshot) {
				status.LastGitHubStatus = "skipped"
			} else if len(selection.GitHubRows) == 0 {
				status.LastGitHubStatus = "skipped"
			} else {
				githubCommand := decodeCommandResult(s.ExportResultsToGitHub(encodeJSON(map[string]any{
					"config":  snapshot,
					"results": selection.GitHubRows,
					"task_id": taskID,
				})))
				if githubCommand.OK {
					status.LastGitHubStatus = "completed"
				} else {
					status.LastGitHubStatus = "failed"
					status.LastMessage = fmt.Sprintf("%s GitHub 错误：%s", mobileSchedulerSingleTaskCompletionMessageForConfig(status.LastDNSStatus, status.LastGitHubStatus, cfg), githubCommand.Message)
				}
			}
		}
	}
	status.WorkflowStage = "completed"
	if (cfg.AutoDNSPush || cfg.AutoGitHubExport) && status.LastGitHubStatus != "failed" {
		status.LastMessage = mobileSchedulerSingleTaskCompletionMessageForConfig(status.LastDNSStatus, status.LastGitHubStatus, cfg)
	}
	rearmMobileSchedulerStatus(&status, cfg)
	_ = s.writeSchedulerStatus(status)
	return encodeCommand(commandResultFor("SCHEDULER_RUN_COMPLETED", status, status.LastMessage, true, &taskID, nil))
}

func (s *Service) RefreshScheduler(payloadJSON string) string {
	_ = payloadJSON
	snapshot, err := s.loadConfigSnapshotFromDisk()
	if err != nil {
		return encodeCommand(commandResultFor("SCHEDULER_REFRESH_FAILED", nil, err.Error(), false, nil, nil))
	}
	status := s.refreshSchedulerStatusForSnapshot(snapshot)
	return encodeCommand(commandResultFor("SCHEDULER_REFRESH_OK", status, "Android 定时任务已刷新。", true, nil, nil))
}

func (s *Service) refreshSchedulerStatusForSnapshot(snapshot map[string]any) mobileSchedulerStatus {
	cfg := mobileSchedulerConfigFromSnapshot(snapshot)
	status := s.currentSchedulerStatus()
	status.Enabled = cfg.Enabled
	if !cfg.Enabled {
		status.NextRunAt = ""
		status.LastMessage = "Android 定时任务未启用。"
		_ = s.writeSchedulerStatus(status)
		return status
	}
	next := mobileNextSchedulerRun(time.Now(), parseMobileSchedulerTime(status.LastRunAt), cfg)
	if next.IsZero() {
		status.Enabled = false
		status.NextRunAt = ""
		status.LastMessage = "Android 定时任务已启用，但没有可用的间隔或每日时间规则。"
		_ = s.writeSchedulerStatus(status)
		return status
	}
	status.NextRunAt = next.Format(time.RFC3339)
	status.LastMessage = "Android 定时任务已启用。"
	_ = s.writeSchedulerStatus(status)
	return status
}

func (s *Service) currentSchedulerStatus() mobileSchedulerStatus {
	status, err := s.loadSchedulerStatus()
	if err != nil {
		return mobileSchedulerStatus{
			Enabled:          false,
			LastProbeStatus:  "",
			LastDNSStatus:    "",
			LastGitHubStatus: "",
			LastMessage:      "Android 定时任务未启用。",
			ConfigSource:     mobileSchedulerConfigSource,
		}
	}
	return status
}

func (s *Service) loadSchedulerStatus() (mobileSchedulerStatus, error) {
	raw, err := os.ReadFile(s.schedulerStatusPath())
	if err != nil {
		return mobileSchedulerStatus{}, err
	}
	var status mobileSchedulerStatus
	if err := json.Unmarshal(raw, &status); err != nil {
		return mobileSchedulerStatus{}, err
	}
	return status, nil
}

func (s *Service) writeSchedulerStatus(status mobileSchedulerStatus) error {
	status.ConfigSource = mobileSchedulerConfigSource
	raw, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return err
	}
	return appcore.WriteFileAtomic(s.schedulerStatusPath(), raw, 0o600)
}

func mobileSchedulerConfigFromSnapshot(snapshot map[string]any) mobileSchedulerConfig {
	raw := mapValue(snapshot["scheduler"])
	return mobileSchedulerConfig{
		Enabled:          boolValue(raw["enabled"], false),
		IntervalMinutes:  max(0, intValue(firstNonNil(raw["interval_minutes"], raw["intervalMinutes"]), 0)),
		DailyTimes:       stringSliceValue(firstNonNil(raw["daily_times"], raw["dailyTimes"])),
		AutoDNSPush:      boolValue(firstNonNil(raw["auto_dns_push"], raw["autoDnsPush"]), true),
		AutoGitHubExport: boolValue(firstNonNil(raw["auto_github_export"], raw["autoGithubExport"]), true),
		SkipIfActive:     boolValue(firstNonNil(raw["skip_if_active"], raw["skipIfActive"]), true),
		RunMode:          defaultMobileSchedulerRunMode,
	}
}

func clearMobileSchedulerUploadProgress(status *mobileSchedulerStatus) {
	status.UploadInputCount = 0
	status.UploadFilteredCount = 0
	status.CloudflareUploadCount = 0
	status.GitHubUploadCount = 0
	status.UploadNotification = nil
}

func rearmMobileSchedulerStatus(status *mobileSchedulerStatus, cfg mobileSchedulerConfig) {
	if next := mobileNextSchedulerRun(time.Now(), time.Now(), cfg); !next.IsZero() {
		status.NextRunAt = next.Format(time.RFC3339)
		return
	}
	status.NextRunAt = ""
}

func mobileSchedulerDNSStatusFromCommand(result commandResult) string {
	if result.OK {
		return "completed"
	}
	if result.Code == "DNS_INPUT_EMPTY" {
		return "skipped"
	}
	return "failed"
}

func mobileSchedulerDownstreamStatusAfterProbeFailure(enabled bool) string {
	if enabled {
		return "failed"
	}
	return "skipped"
}

func mobileSchedulerSingleTaskCompletionMessage(dnsStatus, githubStatus string) string {
	switch githubStatus {
	case "completed":
		switch dnsStatus {
		case "completed":
			return "Android 定时测速、DNS 推送与 GitHub 导出流程已完成。"
		case "failed":
			return "Android 定时测速与 GitHub 导出流程已完成，DNS 推送失败。"
		case "skipped":
			return "Android 定时测速与 GitHub 导出流程已完成，DNS 推送已跳过。"
		default:
			return "Android 定时测速与 GitHub 导出流程已完成。"
		}
	case "failed":
		switch dnsStatus {
		case "completed":
			return "Android 定时测速与 DNS 推送流程已完成，GitHub 导出失败。"
		case "failed":
			return "Android 定时测速流程已完成，DNS 推送与 GitHub 导出失败。"
		case "skipped":
			return "Android 定时测速流程已完成，DNS 推送已跳过，GitHub 导出失败。"
		default:
			return "Android 定时测速流程已完成，GitHub 导出失败。"
		}
	case "skipped":
		switch dnsStatus {
		case "completed":
			return "Android 定时测速与 DNS 推送流程已完成，GitHub 导出已跳过。"
		case "failed":
			return "Android 定时测速流程已完成，DNS 推送失败，GitHub 导出已跳过。"
		case "skipped":
			return "Android 定时测速流程已完成，DNS 推送与 GitHub 导出已跳过。"
		default:
			return "Android 定时测速流程已完成，GitHub 导出已跳过。"
		}
	default:
		switch dnsStatus {
		case "completed":
			return "Android 定时测速与 DNS 推送流程已完成。"
		case "failed":
			return "Android 定时测速流程已完成，DNS 推送失败。"
		default:
			return "Android 定时测速流程已完成。"
		}
	}
}

func mobileSchedulerSingleTaskCompletionMessageForConfig(dnsStatus, githubStatus string, cfg mobileSchedulerConfig) string {
	if !cfg.AutoDNSPush {
		dnsStatus = ""
	}
	if !cfg.AutoGitHubExport {
		githubStatus = ""
	}
	return mobileSchedulerSingleTaskCompletionMessage(dnsStatus, githubStatus)
}

func mobileNextSchedulerRun(now time.Time, lastRun time.Time, cfg mobileSchedulerConfig) time.Time {
	if !cfg.Enabled {
		return time.Time{}
	}
	candidates := make([]time.Time, 0, len(cfg.DailyTimes)+1)
	if cfg.IntervalMinutes > 0 {
		base := now
		if !lastRun.IsZero() {
			base = lastRun
		}
		next := base.Add(time.Duration(cfg.IntervalMinutes) * time.Minute)
		for !next.After(now) {
			next = next.Add(time.Duration(cfg.IntervalMinutes) * time.Minute)
		}
		candidates = append(candidates, next)
	}
	for _, entry := range cfg.DailyTimes {
		hour, minute, second, ok := parseMobileDailyTime(entry)
		if !ok {
			continue
		}
		next := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, second, 0, now.Location())
		if !next.After(now) {
			next = next.Add(24 * time.Hour)
		}
		candidates = append(candidates, next)
	}
	if len(candidates) == 0 {
		return time.Time{}
	}
	next := candidates[0]
	for _, candidate := range candidates[1:] {
		if candidate.Before(next) {
			next = candidate
		}
	}
	return next
}

func parseMobileDailyTime(value string) (int, int, int, bool) {
	parts := strings.Split(strings.ReplaceAll(strings.TrimSpace(value), "：", ":"), ":")
	if len(parts) < 2 || len(parts) > 3 {
		return 0, 0, 0, false
	}
	hour := intValue(parts[0], -1)
	minute := intValue(parts[1], -1)
	second := 0
	if len(parts) == 3 {
		second = intValue(parts[2], -1)
	}
	if hour < 0 || hour > 23 || minute < 0 || minute > 59 || second < 0 || second > 59 {
		return 0, 0, 0, false
	}
	return hour, minute, second, true
}

func parseMobileSchedulerTime(value string) time.Time {
	if strings.TrimSpace(value) == "" {
		return time.Time{}
	}
	parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(value))
	if err != nil {
		return time.Time{}
	}
	return parsed
}

func (s *Service) hasActiveTask() bool {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	return strings.TrimSpace(s.currentTaskID) != "" || strings.TrimSpace(s.pausedTaskID) != "" || strings.TrimSpace(s.currentPipelineID) != ""
}

func mobileProbeRunResultFromAny(value any) probeRunResult {
	var result probeRunResult
	raw, err := json.Marshal(value)
	if err != nil {
		return result
	}
	_ = json.Unmarshal(raw, &result)
	return result
}
