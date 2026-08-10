package appcore

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type SchedulerRunRequest struct {
	Config       map[string]any `json:"config"`
	ConfigSource string         `json:"config_source"`
	TaskID       string         `json:"task_id"`
}

func (s *Service) invokeSchedulerStatus(payloadJSON string) CommandResult {
	if _, err := decodeCommandObject(payloadJSON); err != nil {
		return NewCommandResult("COMMAND_PAYLOAD_INVALID", nil, err.Error(), false, nil, nil)
	}
	status, err := s.CurrentSchedulerStatus()
	if err != nil {
		return NewCommandResult("SCHEDULER_STATUS_READ_FAILED", nil, err.Error(), false, nil, nil)
	}
	return NewCommandResult("SCHEDULER_STATUS_READY", status, "定时任务状态已读取。", true, nil, nil)
}

func (s *Service) invokeSchedulerRefresh(payloadJSON string) CommandResult {
	payload, err := decodeCommandObject(payloadJSON)
	if err != nil {
		return NewCommandResult("SCHEDULER_REFRESH_FAILED", nil, err.Error(), false, nil, nil)
	}
	snapshot := mapValue(firstNonNil(payload["config"], payload["config_snapshot"], payload["configSnapshot"]))
	if len(snapshot) == 0 {
		loaded, loadErr := s.LoadConfig()
		if loadErr != nil {
			return NewCommandResult("SCHEDULER_REFRESH_FAILED", nil, loadErr.Error(), false, nil, nil)
		}
		snapshot = loaded.Snapshot
	} else {
		snapshot = s.sanitizeConfig(snapshot)
	}
	status, err := s.RefreshSchedulerStatus(snapshot)
	if err != nil {
		return NewCommandResult("SCHEDULER_REFRESH_FAILED", nil, err.Error(), false, nil, nil)
	}
	return NewCommandResult("SCHEDULER_REFRESH_OK", status, "定时任务已刷新。", true, nil, nil)
}

func (s *Service) invokeSchedulerRun(payloadJSON string) CommandResult {
	request, err := decodeCommandPayload[SchedulerRunRequest](payloadJSON)
	if err != nil {
		return NewCommandResult("SCHEDULER_RUN_INVALID", nil, err.Error(), false, nil, nil)
	}
	return s.RunScheduledProbe(context.Background(), request)
}

func (s *Service) CurrentSchedulerStatus() (SchedulerStatus, error) {
	status, existed, err := s.LoadSchedulerStatus()
	if err != nil {
		return SchedulerStatus{}, err
	}
	if !existed {
		status.ConfigSource = s.schedulerDefaults().ConfigSource
		status.LastMessage = "定时任务未启用。"
	}
	return status, nil
}

func (s *Service) RefreshSchedulerStatus(snapshot map[string]any) (SchedulerStatus, error) {
	cfg := s.SchedulerConfig(snapshot)
	status, err := s.CurrentSchedulerStatus()
	if err != nil {
		return status, err
	}
	status.Enabled = cfg.Enabled
	status.ConfigSource = cfg.ConfigSource
	if !cfg.Enabled {
		status.NextRunAt = ""
		status.LastMessage = "定时任务未启用。"
		return status, s.SaveSchedulerStatus(status)
	}
	next := NextSchedulerRun(s.now(), parseSchedulerTimestamp(status.LastRunAt), schedulerTiming(cfg))
	if next.IsZero() {
		status.Enabled = false
		status.NextRunAt = ""
		status.LastMessage = "定时任务已启用，但没有可用的间隔或每日时间规则。"
		return status, s.SaveSchedulerStatus(status)
	}
	status.NextRunAt = next.Format(time.RFC3339)
	status.LastMessage = "定时任务已启用。"
	return status, s.SaveSchedulerStatus(status)
}

func (s *Service) SchedulerConfig(snapshot map[string]any) SchedulerConfig {
	return SchedulerConfigFromSnapshot(snapshot, s.schedulerDefaults())
}

func (s *Service) RunScheduledProbe(ctx context.Context, request SchedulerRunRequest) CommandResult {
	if ctx == nil {
		ctx = context.Background()
	}
	s.schedulerRunMu.Lock()
	defer s.schedulerRunMu.Unlock()

	now := s.now()
	taskID := strings.TrimSpace(request.TaskID)
	if taskID == "" {
		taskID = "scheduled-" + now.Format("20060102-150405")
	}
	selection, err := s.selectSchedulerRunConfig(request)
	if err != nil {
		status, _ := s.CurrentSchedulerStatus()
		status.NextRunAt = ""
		status.LastRunAt = now.Format(time.RFC3339)
		status.LastTaskID = taskID
		status.LastProbeStatus = "failed"
		status.LastDNSStatus = ""
		status.LastGitHubStatus = ""
		status.LastMessage = fmt.Sprintf("读取定时任务配置失败：%v", err)
		status.WorkflowStage = "load_config_failed"
		status.ConfigSource = selection.Source
		status.LastSourceProfileAction = ""
		clearSchedulerProgress(&status)
		_ = s.SaveSchedulerStatus(status)
		return NewCommandResult("SCHEDULER_RUN_FAILED", status, status.LastMessage, false, &taskID, nil)
	}
	snapshot := selection.Snapshot
	cfg := s.SchedulerConfig(snapshot)
	status, _ := s.CurrentSchedulerStatus()
	status.Enabled = cfg.Enabled
	status.LastRunAt = now.Format(time.RFC3339)
	status.LastTaskID = taskID
	status.LastProbeStatus = "running"
	status.LastDNSStatus = "skipped"
	status.LastGitHubStatus = "skipped"
	status.LastMessage = "定时测速开始执行。"
	status.WorkflowStage = "probe"
	status.ConfigSource = selection.Source
	status.LastSourceProfileAction = ""
	clearSchedulerProgress(&status)
	_ = s.SaveSchedulerStatus(status)
	if !cfg.Enabled {
		status.NextRunAt = ""
		status.LastProbeStatus = "skipped"
		status.LastMessage = "定时任务未启用，本次已跳过。"
		status.WorkflowStage = "skipped"
		_ = s.SaveSchedulerStatus(status)
		return NewCommandResult("SCHEDULER_RUN_SKIPPED", status, status.LastMessage, true, &taskID, nil)
	}
	if cfg.SkipIfActive && s.hasActiveProbeRuntime() {
		status.LastProbeStatus = "skipped"
		status.LastMessage = "已有探测任务运行或暂停，本次定时任务已跳过。"
		status.WorkflowStage = "skipped"
		rearmSchedulerStatus(&status, cfg, now)
		_ = s.SaveSchedulerStatus(status)
		return NewCommandResult("SCHEDULER_RUN_SKIPPED", status, status.LastMessage, true, &taskID, nil)
	}

	notifyUpload := cfg.AutoDNSPush || cfg.AutoGitHubExport
	var notificationTopEntries []UploadNotificationTopEntry
	finalize := func(result CommandResult) CommandResult {
		if notifyUpload {
			updated, warnings := s.RecordSchedulerUploadNotification(ctx, snapshot, status, UploadNotificationSourceScheduledProbe, taskID, cfg.AutoDNSPush, cfg.AutoGitHubExport, notificationTopEntries)
			status = updated
			result.Data = status
			result.Warnings = dedupeStrings(append(result.Warnings, warnings...))
		}
		_ = s.SaveSchedulerStatus(status)
		return result
	}

	probeResult, runErr := s.RunProbe(ProbePayload{
		Config:               snapshot,
		ConfigSource:         selection.Source,
		DisablePostProbePush: true,
		Sources:              SourcesFromAny(snapshot["sources"]),
		TaskID:               taskID,
	})
	if runErr != nil {
		status.LastProbeStatus = "failed"
		status.LastDNSStatus = schedulerDownstreamStatus(cfg.AutoDNSPush)
		status.LastGitHubStatus = schedulerDownstreamStatus(cfg.AutoGitHubExport)
		status.LastMessage = runErr.Error()
		status.WorkflowStage = "probe_failed"
		rearmSchedulerStatus(&status, cfg, now)
		return finalize(NewCommandResult("SCHEDULER_RUN_FAILED", status, status.LastMessage, false, &taskID, probeResult.Warnings))
	}

	status.LastProbeStatus = "completed"
	status.LastMessage = fmt.Sprintf("定时测速完成，结果 %d 条。", len(probeResult.Results))
	status.WorkflowStage = "post_run_source_profiles"
	status.LastSourceProfileAction = s.applySchedulerSourceProfileAction(snapshot, cfg.PostRunSourceProfileAction)
	selectionResult, selectionErr := BuildUploadSelectionWithColoPaths(snapshot, probeResult.Results, probeResult.Config.DownloadSpeedMetric, s.ColoPaths())
	if selectionErr != nil {
		status.LastProbeStatus = "failed"
		if cfg.AutoDNSPush {
			status.LastDNSStatus = "failed"
		}
		if cfg.AutoGitHubExport {
			status.LastGitHubStatus = "failed"
		}
		status.LastMessage = fmt.Sprintf("上传筛选失败：%v", selectionErr)
		status.WorkflowStage = "upload_selection_failed"
		rearmSchedulerStatus(&status, cfg, now)
		return finalize(NewCommandResult("SCHEDULER_RUN_FAILED", status, status.LastMessage, false, &taskID, probeResult.Warnings))
	}
	status.UploadInputCount = len(selectionResult.InputRows)
	status.UploadFilteredCount = len(selectionResult.FilteredRows)
	status.CloudflareUploadCount = len(selectionResult.CloudflareRows)
	status.GitHubUploadCount = len(selectionResult.GitHubRows)
	if len(selectionResult.Warnings) > 0 {
		status.LastMessage = fmt.Sprintf("定时测速完成，原始 %d 条，筛选后 %d 条。%s", len(selectionResult.InputRows), len(selectionResult.FilteredRows), strings.Join(selectionResult.Warnings, " "))
	} else {
		status.LastMessage = fmt.Sprintf("定时测速完成，原始 %d 条，筛选后 %d 条。", len(selectionResult.InputRows), len(selectionResult.FilteredRows))
	}
	notificationTopEntries = BuildUploadNotificationTopEntriesForSnapshot(snapshot, selectionResult.FilteredRows, probeResult.Config.DownloadSpeedMetric)

	if cfg.AutoDNSPush {
		status.WorkflowStage = "dns"
		if len(probeResult.Results) == 0 {
			status.LastDNSStatus = "skipped"
		} else {
			dnsResult := s.PushCloudflareDNS(ctx, map[string]any{"config": snapshot, "results": probeResult.Results})
			status.LastDNSStatus = schedulerDNSStatus(dnsResult)
			if !dnsResult.OK || status.LastDNSStatus == UploadNotificationStatusPartial {
				status.LastMessage = dnsResult.Message
			}
			if uploadCount := intValue(mapValue(dnsResult.Data)["upload_count"], -1); uploadCount >= 0 {
				status.CloudflareUploadCount = uploadCount
			}
		}
	}
	if cfg.AutoGitHubExport {
		status.WorkflowStage = "github"
		switch {
		case !GitHubProviderEnabledFromSnapshot(snapshot), len(selectionResult.GitHubRows) == 0:
			status.LastGitHubStatus = "skipped"
		default:
			_, exportErr := s.ExportProbeRowsToGitHub(ctx, snapshot, taskID, selectionResult.GitHubRows)
			if exportErr != nil {
				status.LastGitHubStatus = "failed"
				status.LastMessage = fmt.Sprintf("%s GitHub 错误：%v", schedulerCompletionMessage(status.LastDNSStatus, status.LastGitHubStatus, cfg), exportErr)
			} else {
				status.LastGitHubStatus = "completed"
			}
		}
	}
	status.WorkflowStage = "completed"
	if (cfg.AutoDNSPush || cfg.AutoGitHubExport) && status.LastGitHubStatus != "failed" && ShouldOverwriteSchedulerDNSMessage(status.LastDNSStatus, status.LastGitHubStatus, cfg.AutoDNSPush, cfg.AutoGitHubExport) {
		status.LastMessage = schedulerCompletionMessage(status.LastDNSStatus, status.LastGitHubStatus, cfg)
	}
	rearmSchedulerStatus(&status, cfg, now)
	return finalize(NewCommandResult("SCHEDULER_RUN_COMPLETED", status, status.LastMessage, true, &taskID, probeResult.Warnings))
}

func (s *Service) selectSchedulerRunConfig(request SchedulerRunRequest) (SchedulerConfigSelection, error) {
	source := strings.TrimSpace(request.ConfigSource)
	if source == "" {
		loaded, err := s.LoadConfig()
		if err != nil {
			return SchedulerConfigSelection{Source: s.schedulerDefaults().ConfigSource}, err
		}
		source = s.SchedulerConfig(loaded.Snapshot).ConfigSource
	}
	return s.SelectSchedulerConfig(source, request.Config)
}

func (s *Service) applySchedulerSourceProfileAction(snapshot map[string]any, action string) string {
	store, err := LoadSourceProfileStore(s.StorageLayout().SourceProfilesPath(), DefaultSourceProfilesSchemaVersion)
	if err != nil {
		return "failed"
	}
	store, status, changed := ApplySchedulerSourceProfileAction(store, SourcesFromAny(snapshot["sources"]), action, s.now())
	if !changed {
		return status
	}
	store.ActiveProfileID = RecentRunSourceProfileID
	if err := s.saveSourceProfiles(store); err != nil {
		return "failed"
	}
	return status
}

func (s *Service) schedulerDefaults() SchedulerConfig {
	s.mu.RLock()
	defaults := s.options.SchedulerDefaults
	s.mu.RUnlock()
	if !defaults.Enabled && defaults.IntervalMinutes == 0 && len(defaults.DailyTimes) == 0 &&
		!defaults.AutoDNSPush && !defaults.AutoGitHubExport && !defaults.SkipIfActive &&
		strings.TrimSpace(defaults.ConfigSource) == "" && strings.TrimSpace(defaults.PostRunSourceProfileAction) == "" {
		defaults.AutoDNSPush = true
		defaults.AutoGitHubExport = true
		defaults.SkipIfActive = true
	}
	if strings.TrimSpace(defaults.ConfigSource) == "" {
		defaults.ConfigSource = "saved"
	}
	if strings.TrimSpace(defaults.PostRunSourceProfileAction) == "" {
		defaults.PostRunSourceProfileAction = SchedulerSourceProfileActionUpdate
	}
	return defaults
}

func (s *Service) hasActiveProbeRuntime() bool {
	state := s.runtime.State()
	return strings.TrimSpace(state.CurrentTaskID) != "" || strings.TrimSpace(state.PausedTaskID) != ""
}

func schedulerDNSStatus(result CommandResult) string {
	if result.Code == "DNS_PUSH_PARTIAL" {
		return "partial"
	}
	if result.OK {
		return "completed"
	}
	if result.Code == "DNS_INPUT_EMPTY" {
		return "skipped"
	}
	return "failed"
}

func schedulerDownstreamStatus(enabled bool) string {
	if enabled {
		return "failed"
	}
	return "skipped"
}

func schedulerCompletionMessage(dnsStatus, githubStatus string, cfg SchedulerConfig) string {
	if !cfg.AutoDNSPush {
		dnsStatus = ""
	}
	if !cfg.AutoGitHubExport {
		githubStatus = ""
	}
	return SchedulerCompletionMessage("", dnsStatus, githubStatus)
}

func clearSchedulerProgress(status *SchedulerStatus) {
	status.UploadInputCount = 0
	status.UploadFilteredCount = 0
	status.CloudflareUploadCount = 0
	status.GitHubUploadCount = 0
	status.UploadNotification = nil
}

func rearmSchedulerStatus(status *SchedulerStatus, cfg SchedulerConfig, now time.Time) {
	next := NextSchedulerRun(now, now, schedulerTiming(cfg))
	if next.IsZero() {
		status.NextRunAt = ""
		return
	}
	status.NextRunAt = next.Format(time.RFC3339)
}

func schedulerTiming(cfg SchedulerConfig) SchedulerTimingConfig {
	return SchedulerTimingConfig{Enabled: cfg.Enabled, IntervalMinutes: cfg.IntervalMinutes, DailyTimes: cfg.DailyTimes}
}

func parseSchedulerTimestamp(raw string) time.Time {
	parsed, _ := time.Parse(time.RFC3339, strings.TrimSpace(raw))
	return parsed
}
