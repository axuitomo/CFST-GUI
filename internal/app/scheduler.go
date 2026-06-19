package app

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/appcore"
)

type SchedulerConfig struct {
	Enabled                    bool     `json:"enabled"`
	IntervalMinutes            int      `json:"interval_minutes"`
	DailyTimes                 []string `json:"daily_times"`
	AutoDNSPush                bool     `json:"auto_dns_push"`
	AutoGitHubExport           bool     `json:"auto_github_export"`
	PipelineTemplateID         string   `json:"pipeline_template_id"`
	SkipIfActive               bool     `json:"skip_if_active"`
	ConfigSource               string   `json:"config_source"`
	PostRunSourceProfileAction string   `json:"post_run_source_profile_action"`
	RunMode                    string   `json:"run_mode"`
	legacySelectorWarnings     []string
}

type SchedulerStatus struct {
	Enabled                 bool                        `json:"enabled"`
	NextRunAt               string                      `json:"next_run_at"`
	LastRunAt               string                      `json:"last_run_at"`
	LastTaskID              string                      `json:"last_task_id"`
	LastProbeStatus         string                      `json:"last_probe_status"`
	LastDNSStatus           string                      `json:"last_dns_status"`
	LastGitHubStatus        string                      `json:"last_github_status"`
	LastMessage             string                      `json:"last_message"`
	WorkflowStage           string                      `json:"workflow_stage"`
	ConfigSource            string                      `json:"config_source"`
	LastSourceProfileAction string                      `json:"last_source_profile_action"`
	UploadInputCount        int                         `json:"upload_input_count"`
	UploadFilteredCount     int                         `json:"upload_filtered_count"`
	CloudflareUploadCount   int                         `json:"cloudflare_upload_count"`
	GitHubUploadCount       int                         `json:"github_upload_count"`
	UploadNotification      *appcore.UploadNotification `json:"upload_notification,omitempty"`
}

func (a *App) LoadSchedulerStatus() DesktopCommandResult {
	a.schedulerMu.Lock()
	status := a.schedulerStatus
	a.schedulerMu.Unlock()
	return desktopCommandResult("SCHEDULER_STATUS_READY", status, "定时任务状态已读取。", true, nil, nil)
}

func (a *App) reloadSchedulerFromDisk() {
	snapshot, err := loadDesktopConfigSnapshotFromDisk()
	if err != nil {
		a.setSchedulerStatus(func(status *SchedulerStatus) {
			status.Enabled = false
			status.LastMessage = fmt.Sprintf("读取定时任务配置失败：%v", err)
		})
		return
	}
	a.reloadSchedulerFromSnapshot(snapshot)
}

func (a *App) reloadSchedulerFromSnapshot(snapshot map[string]any) {
	cfg := schedulerConfigFromSnapshot(snapshot)
	a.stopScheduler()
	a.setSchedulerStatus(func(status *SchedulerStatus) {
		status.Enabled = cfg.Enabled
		status.NextRunAt = ""
		if cfg.Enabled {
			status.LastMessage = schedulerStatusMessage("定时任务已启用。", cfg.legacySelectorWarnings)
			return
		}
		status.LastMessage = schedulerStatusMessage("定时任务未启用。", cfg.legacySelectorWarnings)
	})
	if !cfg.Enabled {
		return
	}
	next := nextSchedulerRun(time.Now(), parseSchedulerTime(a.currentSchedulerStatus().LastRunAt), cfg)
	if next.IsZero() {
		a.setSchedulerStatus(func(status *SchedulerStatus) {
			status.Enabled = false
			status.LastMessage = schedulerStatusMessage("定时任务已启用，但没有可用的间隔或每日时间规则。", cfg.legacySelectorWarnings)
		})
		return
	}
	a.setSchedulerStatus(func(status *SchedulerStatus) {
		status.Enabled = cfg.Enabled
		status.NextRunAt = next.Format(time.RFC3339)
	})
	ctx, cancel := context.WithCancel(context.Background())
	a.schedulerMu.Lock()
	a.schedulerCancel = cancel
	a.schedulerMu.Unlock()
	go a.schedulerLoop(ctx, cfg)
}

func (a *App) stopScheduler() {
	a.schedulerMu.Lock()
	cancel := a.schedulerCancel
	a.schedulerCancel = nil
	a.schedulerMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (a *App) schedulerLoop(ctx context.Context, cfg SchedulerConfig) {
	for {
		status := a.currentSchedulerStatus()
		next := nextSchedulerRun(time.Now(), parseSchedulerTime(status.LastRunAt), cfg)
		if next.IsZero() {
			a.setSchedulerStatus(func(status *SchedulerStatus) {
				status.NextRunAt = ""
				status.LastMessage = "定时任务没有下一次运行时间。"
			})
			return
		}
		a.setSchedulerStatus(func(status *SchedulerStatus) {
			status.Enabled = cfg.Enabled
			status.NextRunAt = next.Format(time.RFC3339)
		})
		timer := time.NewTimer(time.Until(next))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			a.runScheduledProbe(ctx, cfg)
		}
	}
}

func (a *App) runScheduledProbe(ctx context.Context, cfg SchedulerConfig) {
	now := time.Now()
	taskID := "scheduled-" + now.Format("20060102-150405")
	var notificationSnapshot map[string]any
	notifyUpload := false
	defer func() {
		if notifyUpload {
			a.recordSchedulerUploadNotification(notificationSnapshot, appcore.UploadNotificationSourceScheduledProbe, taskID, cfg.AutoDNSPush, cfg.AutoGitHubExport)
		}
	}()
	if cfg.SkipIfActive && a.hasActiveProbeTask() {
		a.setSchedulerStatus(func(status *SchedulerStatus) {
			status.LastRunAt = now.Format(time.RFC3339)
			status.LastTaskID = taskID
			status.LastProbeStatus = "skipped"
			status.LastDNSStatus = ""
			status.LastGitHubStatus = ""
			status.LastMessage = "已有探测任务运行或暂停，本次定时任务已跳过。"
			status.WorkflowStage = "skipped"
			status.ConfigSource = ""
		})
		return
	}
	if normalizeSchedulerRunMode(cfg.RunMode) == "pipeline" {
		a.runScheduledPipeline(ctx, cfg, now, taskID)
		return
	}
	snapshot, configSource, err := schedulerSnapshotForRun(cfg)
	if err != nil {
		a.setSchedulerStatus(func(status *SchedulerStatus) {
			status.LastRunAt = now.Format(time.RFC3339)
			status.LastTaskID = taskID
			status.LastProbeStatus = "failed"
			status.LastDNSStatus = ""
			status.LastGitHubStatus = ""
			status.LastMessage = fmt.Sprintf("读取配置失败：%v", err)
			status.WorkflowStage = "load_config_failed"
			status.ConfigSource = configSource
			status.LastSourceProfileAction = ""
			status.UploadInputCount = 0
			status.UploadFilteredCount = 0
			status.CloudflareUploadCount = 0
			status.GitHubUploadCount = 0
		})
		return
	}
	notificationSnapshot = snapshot
	notifyUpload = cfg.AutoDNSPush || cfg.AutoGitHubExport
	a.setSchedulerStatus(func(status *SchedulerStatus) {
		status.LastRunAt = now.Format(time.RFC3339)
		status.LastTaskID = taskID
		status.LastProbeStatus = "running"
		status.LastDNSStatus = ""
		status.LastGitHubStatus = ""
		status.LastMessage = "定时测速开始执行。"
		status.WorkflowStage = "probe"
		status.ConfigSource = configSource
		status.LastSourceProfileAction = ""
		status.UploadInputCount = 0
		status.UploadFilteredCount = 0
		status.CloudflareUploadCount = 0
		status.GitHubUploadCount = 0
		status.UploadNotification = nil
	})
	payload := DesktopProbePayload{
		Config:               snapshot,
		ConfigSource:         configSource,
		DisablePostProbePush: true,
		Sources:              desktopSourcesFromAny(snapshot["sources"]),
		TaskID:               taskID,
	}
	result, err := a.RunDesktopProbe(payload)
	if err != nil {
		a.setSchedulerStatus(func(status *SchedulerStatus) {
			status.LastRunAt = now.Format(time.RFC3339)
			status.LastTaskID = taskID
			status.LastProbeStatus = "failed"
			status.LastDNSStatus = ""
			status.LastGitHubStatus = ""
			status.LastMessage = err.Error()
			status.WorkflowStage = "probe_failed"
			status.ConfigSource = configSource
		})
		return
	}
	sourceProfileAction := updateRecentRunSourceProfile(desktopSourcesFromAny(snapshot["sources"]))
	a.setSchedulerStatus(func(status *SchedulerStatus) {
		status.LastRunAt = now.Format(time.RFC3339)
		status.LastTaskID = taskID
		status.LastProbeStatus = "completed"
		status.LastDNSStatus = "skipped"
		status.LastGitHubStatus = "skipped"
		status.LastMessage = fmt.Sprintf("定时测速完成，结果 %d 条。", len(result.Results))
		status.WorkflowStage = "post_run_source_profiles"
		status.ConfigSource = configSource
		status.LastSourceProfileAction = sourceProfileAction
	})
	if len(result.Results) == 0 {
		a.setSchedulerStatus(func(status *SchedulerStatus) {
			status.WorkflowStage = "completed"
		})
		return
	}
	selection, err := BuildUploadSelection(snapshot, result.Results, result.Config.DownloadSpeedMetric)
	if err != nil {
		a.setSchedulerStatus(func(status *SchedulerStatus) {
			status.LastProbeStatus = "failed"
			status.LastMessage = fmt.Sprintf("上传筛选失败：%v", err)
			status.WorkflowStage = "upload_selection_failed"
		})
		return
	}
	a.setSchedulerStatus(func(status *SchedulerStatus) {
		status.UploadInputCount = len(selection.InputRows)
		status.UploadFilteredCount = len(selection.FilteredRows)
		status.CloudflareUploadCount = len(selection.CloudflareRows)
		status.GitHubUploadCount = len(selection.GitHubRows)
		if len(selection.Warnings) > 0 {
			status.LastMessage = fmt.Sprintf("定时测速完成，原始 %d 条，筛选后 %d 条。%s", len(selection.InputRows), len(selection.FilteredRows), strings.Join(selection.Warnings, " "))
		} else {
			status.LastMessage = fmt.Sprintf("定时测速完成，原始 %d 条，筛选后 %d 条。", len(selection.InputRows), len(selection.FilteredRows))
		}
	})
	if cfg.AutoDNSPush {
		a.setSchedulerStatus(func(status *SchedulerStatus) {
			status.WorkflowStage = "dns"
			status.CloudflareUploadCount = len(selection.CloudflareRows)
		})
		dnsResult := a.PushCloudflareDNSRecords(map[string]any{
			"config":  snapshot,
			"results": result.Results,
		})
		a.setSchedulerStatus(func(status *SchedulerStatus) {
			if dnsResult.OK {
				status.LastDNSStatus = "completed"
			} else {
				status.LastDNSStatus = "failed"
			}
			if uploadCount := intValue(mapValue(dnsResult.Data)["upload_count"], -1); uploadCount >= 0 {
				status.CloudflareUploadCount = uploadCount
			}
			status.LastMessage = dnsResult.Message
		})
	}
	if cfg.AutoGitHubExport {
		a.setSchedulerStatus(func(status *SchedulerStatus) {
			status.WorkflowStage = "github"
		})
		if !githubExportEnabledFromSnapshot(snapshot) {
			a.setSchedulerStatus(func(status *SchedulerStatus) {
				status.LastGitHubStatus = "skipped"
				status.LastMessage = "GitHub 导出未启用，本次定时任务已跳过 GitHub 导出。"
				status.WorkflowStage = "completed"
			})
			return
		}
		if len(selection.GitHubRows) == 0 {
			a.setSchedulerStatus(func(status *SchedulerStatus) {
				status.LastGitHubStatus = "skipped"
				status.LastMessage = "GitHub 导出没有可上传结果。"
				status.WorkflowStage = "completed"
			})
			return
		}
		_, err := exportProbeRowsToGitHub(ctx, snapshot, taskID, selection.GitHubRows, time.Now())
		a.setSchedulerStatus(func(status *SchedulerStatus) {
			if err != nil {
				status.LastGitHubStatus = "failed"
				status.LastMessage = fmt.Sprintf("GitHub 导出失败：%v", err)
				return
			}
			status.LastGitHubStatus = "completed"
			status.LastMessage = "定时测速、DNS 推送与 GitHub 导出流程已完成。"
			status.WorkflowStage = "completed"
		})
	}
	a.setSchedulerStatus(func(status *SchedulerStatus) {
		if status.WorkflowStage != "completed" {
			status.WorkflowStage = "completed"
		}
	})
}

func (a *App) runScheduledPipeline(ctx context.Context, cfg SchedulerConfig, now time.Time, taskID string) {
	_ = ctx
	profiles, templateID, templateName, warnings, err := schedulerPipelineProfilesForRun(cfg)
	if err != nil {
		a.setSchedulerStatus(func(status *SchedulerStatus) {
			status.LastRunAt = now.Format(time.RFC3339)
			status.LastTaskID = taskID
			status.LastProbeStatus = "failed"
			status.LastDNSStatus = ""
			status.LastGitHubStatus = ""
			status.LastMessage = schedulerStatusMessage(fmt.Sprintf("读取工作流调度失败：%v", err), warnings)
			status.WorkflowStage = "load_pipeline_workspace_failed"
			status.ConfigSource = "pipeline_workspace"
			status.LastSourceProfileAction = "skipped"
			status.UploadInputCount = 0
			status.UploadFilteredCount = 0
			status.CloudflareUploadCount = 0
			status.GitHubUploadCount = 0
		})
		return
	}
	notifyUpload := cfg.AutoDNSPush || cfg.AutoGitHubExport
	notificationSnapshot := map[string]any{}
	if len(profiles) > 0 {
		notificationSnapshot = profiles[0].ConfigSnapshot
	}
	defer func() {
		if notifyUpload {
			a.recordSchedulerUploadNotification(notificationSnapshot, appcore.UploadNotificationSourceScheduledPipeline, taskID, cfg.AutoDNSPush, cfg.AutoGitHubExport)
		}
	}()
	a.setSchedulerStatus(func(status *SchedulerStatus) {
		status.LastRunAt = now.Format(time.RFC3339)
		status.LastTaskID = taskID
		status.LastProbeStatus = "running"
		status.LastDNSStatus = ""
		status.LastGitHubStatus = ""
		status.LastMessage = schedulerStatusMessage(fmt.Sprintf("定时工作流开始执行：模板 %s，使用 %d 份绑定配置。", templateName, len(profiles)), warnings)
		status.WorkflowStage = "pipeline"
		status.ConfigSource = "pipeline_workspace"
		status.LastSourceProfileAction = "skipped"
		status.UploadInputCount = 0
		status.UploadFilteredCount = 0
		status.CloudflareUploadCount = 0
		status.GitHubUploadCount = 0
		status.UploadNotification = nil
	})
	result, runErr := a.runPipeline(PipelineRunPayload{
		ConfigSource: "scheduler_pipeline",
		PipelineID:   taskID,
		Profiles:     profiles,
		SchedulerOverrides: appcore.PipelineRuntimeOverrides{
			AllowDNSPush:         boolPointer(cfg.AutoDNSPush),
			AllowGitHubExport:    boolPointer(cfg.AutoGitHubExport),
			DisablePostProbePush: true,
		},
		TaskID:     taskID,
		TemplateID: templateID,
	})
	if result.PipelineID == "" {
		message := "定时工作流执行失败。"
		if runErr != nil {
			message = runErr.Error()
		}
		a.setSchedulerStatus(func(status *SchedulerStatus) {
			status.LastRunAt = now.Format(time.RFC3339)
			status.LastTaskID = taskID
			status.LastProbeStatus = "failed"
			status.LastDNSStatus = ""
			status.LastGitHubStatus = ""
			status.LastMessage = schedulerStatusMessage(message, warnings)
			status.WorkflowStage = "pipeline_failed"
			status.ConfigSource = "pipeline_workspace"
		})
		return
	}
	dnsStatus, dnsAttempted := schedulerPipelineDNSStatus(profiles, result, cfg.AutoDNSPush)
	lastGitHubStatus := "skipped"
	message := fmt.Sprintf("定时工作流完成：模板 %s，成功 %d，失败 %d，跳过 %d。", templateName, result.Succeeded, result.Failed, result.Skipped)
	if dnsAttempted > 0 {
		message = fmt.Sprintf("%s DNS 已处理 %d 份绑定配置。", message, dnsAttempted)
	}
	if cfg.AutoGitHubExport {
		lastGitHubStatus = "unsupported"
		message = fmt.Sprintf("%s GitHub 导出暂未接入策略管道定时模式，已跳过。", message)
	}
	if runErr != nil && strings.TrimSpace(runErr.Error()) != "" {
		message = fmt.Sprintf("%s %s", message, runErr.Error())
	}
	message = schedulerStatusMessage(message, warnings)
	a.setSchedulerStatus(func(status *SchedulerStatus) {
		status.LastRunAt = now.Format(time.RFC3339)
		status.LastTaskID = taskID
		status.LastProbeStatus = firstNonEmptyString(result.Status, "failed")
		status.LastDNSStatus = dnsStatus
		status.LastGitHubStatus = lastGitHubStatus
		status.LastMessage = message
		status.WorkflowStage = "completed"
		status.ConfigSource = "pipeline_workspace"
		status.CloudflareUploadCount = dnsAttempted
	})
}

func schedulerSnapshotForRun(cfg SchedulerConfig) (map[string]any, string, error) {
	if strings.EqualFold(strings.TrimSpace(cfg.ConfigSource), defaultSchedulerConfigSource) {
		status := desktopDraftStatusPayload()
		if boolValue(status["exists"], false) && boolValue(status["is_newer_than_saved"], false) {
			snapshot := mapValue(status["config_snapshot"])
			if len(snapshot) > 0 {
				return sanitizeDesktopConfigSnapshot(snapshot), "draft", nil
			}
		}
	}
	snapshot, err := loadDesktopConfigSnapshotFromDisk()
	return snapshot, "saved", err
}

func updateRecentRunSourceProfile(sources []DesktopSource) string {
	store, err := loadSourceProfileStore()
	if err != nil {
		return "failed"
	}
	now := time.Now().Format(time.RFC3339)
	updated := false
	for index := range store.Items {
		if store.Items[index].ID != recentRunSourceProfileID {
			continue
		}
		store.Items[index].Name = recentRunSourceProfileName
		store.Items[index].Sources = cloneDesktopSources(sources)
		if store.Items[index].CreatedAt == "" {
			store.Items[index].CreatedAt = now
		}
		store.Items[index].UpdatedAt = now
		updated = true
		break
	}
	if !updated {
		store.Items = append(store.Items, sourceProfileItem{
			CreatedAt: now,
			ID:        recentRunSourceProfileID,
			Name:      recentRunSourceProfileName,
			Sources:   cloneDesktopSources(sources),
			UpdatedAt: now,
		})
	}
	if err := saveSourceProfileStore(store); err != nil {
		return "failed"
	}
	if updated {
		return "updated"
	}
	return "created"
}

func githubExportEnabledFromSnapshot(snapshot map[string]any) bool {
	return appcore.GitHubProviderEnabledFromSnapshot(snapshot)
}

func schedulerConfigFromSnapshot(snapshot map[string]any) SchedulerConfig {
	raw := mapValue(snapshot["scheduler"])
	legacySelector := firstNonNil(raw["pipeline_target_selector"], raw["pipelineTargetSelector"])
	return SchedulerConfig{
		Enabled:                    boolValue(raw["enabled"], false),
		IntervalMinutes:            intValue(firstNonNil(raw["interval_minutes"], raw["intervalMinutes"]), 0),
		DailyTimes:                 stringSliceValue(firstNonNil(raw["daily_times"], raw["dailyTimes"])),
		AutoDNSPush:                boolValue(firstNonNil(raw["auto_dns_push"], raw["autoDnsPush"]), true),
		AutoGitHubExport:           boolValue(firstNonNil(raw["auto_github_export"], raw["autoGithubExport"]), true),
		PipelineTemplateID:         strings.TrimSpace(stringValue(firstNonNil(raw["pipeline_template_id"], raw["pipelineTemplateId"]), "")),
		SkipIfActive:               boolValue(firstNonNil(raw["skip_if_active"], raw["skipIfActive"]), true),
		ConfigSource:               stringValue(firstNonNil(raw["config_source"], raw["configSource"]), defaultSchedulerConfigSource),
		PostRunSourceProfileAction: stringValue(firstNonNil(raw["post_run_source_profile_action"], raw["postRunSourceProfileAction"]), defaultSchedulerSourceProfileAction),
		RunMode:                    normalizeSchedulerRunMode(stringValue(firstNonNil(raw["run_mode"], raw["runMode"]), defaultSchedulerRunMode)),
		legacySelectorWarnings:     schedulerPipelineLegacySelectorWarnings(legacySelector),
	}
}

func normalizeSchedulerRunMode(value string) string {
	if strings.EqualFold(strings.TrimSpace(value), "pipeline") {
		return "pipeline"
	}
	return defaultSchedulerRunMode
}

func schedulerPipelineProfilesForRun(cfg SchedulerConfig) ([]PipelineProfile, string, string, []string, error) {
	workspace, warnings, err := loadPipelineWorkspaceOrDefault()
	if err != nil {
		return nil, "", "", warnings, err
	}
	warnings = append(warnings, cfg.legacySelectorWarnings...)
	templateID := strings.TrimSpace(cfg.PipelineTemplateID)
	if templateID == "" {
		templateID = firstNonEmptyString(workspace.ActiveTemplateID, appcore.DefaultPipelineTemplateID)
	}
	templateName := templateID
	templateExists := false
	var templateItem *pipelineTemplateItem
	for index := range workspace.Templates {
		template := &workspace.Templates[index]
		if strings.TrimSpace(template.ID) != templateID {
			continue
		}
		templateExists = true
		templateItem = template
		templateName = firstNonEmptyString(strings.TrimSpace(template.Name), templateID)
		break
	}
	if !templateExists {
		return nil, "", "", warnings, fmt.Errorf("未找到工作流模板 %s", templateID)
	}
	if templateItem == nil || len(templateItem.BoundConfigSnapshot) == 0 {
		return nil, "", "", warnings, fmt.Errorf("工作流模板 %s 尚未绑定执行配置", templateName)
	}
	targetIDs, err := schedulerPipelineTargetIDsForRun(workspace, templateID)
	if err != nil {
		return nil, "", "", warnings, err
	}
	profiles := pipelineProfilesFromWorkspaceSelection(workspace, templateID, targetIDs)
	if len(profiles) == 0 || len(profiles[0].ConfigSnapshot) == 0 {
		return nil, "", "", warnings, fmt.Errorf("工作流模板 %s 尚未绑定执行配置", templateName)
	}
	return profiles, templateID, templateName, warnings, nil
}

func schedulerPipelineTargetIDsForRun(workspace pipelineWorkspace, templateID string) ([]string, error) {
	templateID = strings.TrimSpace(templateID)
	targetIDs := make([]string, 0, len(workspace.Targets))
	for _, target := range workspace.Targets {
		if strings.TrimSpace(target.TemplateID) != templateID || strings.TrimSpace(target.ID) == "" {
			continue
		}
		if !target.Enabled {
			continue
		}
		if len(target.ConfigSnapshot) == 0 {
			continue
		}
		targetIDs = append(targetIDs, strings.TrimSpace(target.ID))
	}
	if len(targetIDs) > 0 {
		return targetIDs, nil
	}
	return nil, fmt.Errorf("工作流模板 %s 尚未绑定执行配置", firstNonEmptyString(templateID, appcore.DefaultPipelineTemplateID))
}

func schedulerPipelineLegacySelectorWarnings(value any) []string {
	if value == nil {
		return nil
	}
	return []string{"已忽略旧版目标选择器，按单绑定配置模式运行。"}
}

func schedulerStatusMessage(message string, warnings []string) string {
	message = strings.TrimSpace(message)
	if len(warnings) == 0 {
		return message
	}
	if message == "" {
		return strings.Join(warnings, " ")
	}
	return fmt.Sprintf("%s %s", message, strings.Join(warnings, " "))
}

func boolPointer(value bool) *bool {
	return &value
}

func schedulerPipelineDNSStatus(profiles []PipelineProfile, result PipelineRunResult, autoDNSPush bool) (string, int) {
	if !autoDNSPush {
		return "skipped", 0
	}
	profileByID := make(map[string]PipelineProfile, len(profiles))
	for _, profile := range profiles {
		profileByID[profile.ID] = profile
	}
	completed := 0
	failed := 0
	for _, item := range result.Results {
		profile, ok := profileByID[item.ProfileID]
		if !ok || !appcore.PipelineDNSPushEnabled(profile.DNSPushPolicy) {
			continue
		}
		if item.DNSResult == nil {
			continue
		}
		if item.Status == "dns_failed" {
			failed++
			continue
		}
		completed++
	}
	switch {
	case failed > 0 && completed > 0:
		return "partial", completed + failed
	case failed > 0:
		return "failed", failed
	case completed > 0:
		return "completed", completed
	default:
		return "skipped", 0
	}
}

func nextSchedulerRun(now time.Time, lastRun time.Time, cfg SchedulerConfig) time.Time {
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
		hour, minute, second, ok := parseDailySchedulerTime(raw)
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

func earlierSchedulerTime(current, candidate time.Time) time.Time {
	if candidate.IsZero() {
		return current
	}
	if current.IsZero() || candidate.Before(current) {
		return candidate
	}
	return current
}

func parseDailySchedulerTime(raw string) (int, int, int, bool) {
	parts := strings.Split(strings.TrimSpace(raw), ":")
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

func parseSchedulerInt(raw string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return fallback
	}
	return value
}

func parseSchedulerTime(raw string) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}
	}
	value, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}
	}
	return value
}

func stringSliceValue(value any) []string {
	switch typed := value.(type) {
	case []string:
		return append([]string(nil), typed...)
	case []any:
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			if text := strings.TrimSpace(stringValue(item, "")); text != "" {
				result = append(result, text)
			}
		}
		return result
	case string:
		fields := strings.FieldsFunc(typed, func(r rune) bool {
			return r == ',' || r == ';' || r == '\n' || r == '\r' || r == '\t' || r == ' '
		})
		result := make([]string, 0, len(fields))
		for _, field := range fields {
			if text := strings.TrimSpace(field); text != "" {
				result = append(result, text)
			}
		}
		return result
	default:
		return nil
	}
}

func probeRowsIPList(rows []ProbeRow) string {
	values := make([]string, 0, len(rows))
	for _, row := range rows {
		if ip := strings.TrimSpace(row.IP); ip != "" {
			values = append(values, ip)
		}
	}
	return strings.Join(values, "\n")
}

func (a *App) hasActiveProbeTask() bool {
	return a.currentProbeRuntimeTaskID() != "" || a.hasActivePipelineTask()
}

func (a *App) currentSchedulerStatus() SchedulerStatus {
	a.schedulerMu.Lock()
	defer a.schedulerMu.Unlock()
	return a.schedulerStatus
}

func (a *App) setSchedulerStatus(update func(*SchedulerStatus)) {
	a.schedulerMu.Lock()
	defer a.schedulerMu.Unlock()
	status := a.schedulerStatus
	update(&status)
	a.schedulerStatus = status
}
