package app

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type SchedulerConfig struct {
	Enabled                    bool     `json:"enabled"`
	IntervalMinutes            int      `json:"interval_minutes"`
	DailyTimes                 []string `json:"daily_times"`
	AutoDNSPush                bool     `json:"auto_dns_push"`
	AutoGitHubExport           bool     `json:"auto_github_export"`
	SkipIfActive               bool     `json:"skip_if_active"`
	ConfigSource               string   `json:"config_source"`
	PostRunProfileAction       string   `json:"post_run_profile_action"`
	PostRunSourceProfileAction string   `json:"post_run_source_profile_action"`
}

type SchedulerStatus struct {
	Enabled                 bool   `json:"enabled"`
	NextRunAt               string `json:"next_run_at"`
	LastRunAt               string `json:"last_run_at"`
	LastTaskID              string `json:"last_task_id"`
	LastProbeStatus         string `json:"last_probe_status"`
	LastDNSStatus           string `json:"last_dns_status"`
	LastGitHubStatus        string `json:"last_github_status"`
	LastMessage             string `json:"last_message"`
	WorkflowStage           string `json:"workflow_stage"`
	ConfigSource            string `json:"config_source"`
	LastProfileAction       string `json:"last_profile_action"`
	LastSourceProfileAction string `json:"last_source_profile_action"`
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
		if !cfg.Enabled {
			status.LastMessage = "定时任务未启用。"
		}
	})
	if !cfg.Enabled {
		return
	}
	next := nextSchedulerRun(time.Now(), parseSchedulerTime(a.currentSchedulerStatus().LastRunAt), cfg)
	if next.IsZero() {
		a.setSchedulerStatus(func(status *SchedulerStatus) {
			status.Enabled = false
			status.LastMessage = "定时任务已启用，但没有可用的间隔或每日时间规则。"
		})
		return
	}
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
		})
		return
	}
	a.setSchedulerStatus(func(status *SchedulerStatus) {
		status.LastRunAt = now.Format(time.RFC3339)
		status.LastTaskID = taskID
		status.LastProbeStatus = "running"
		status.LastDNSStatus = ""
		status.LastGitHubStatus = ""
		status.LastMessage = "定时工作流开始执行。"
		status.WorkflowStage = "probe"
		status.ConfigSource = configSource
	})
	payload := DesktopProbePayload{
		Config:       snapshot,
		ConfigSource: configSource,
		Sources:      desktopSourcesFromAny(snapshot["sources"]),
		TaskID:       taskID,
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
	profileAction := updateRecentRunProfile(snapshot)
	sourceProfileAction := updateRecentRunSourceProfile(desktopSourcesFromAny(snapshot["sources"]))
	a.setSchedulerStatus(func(status *SchedulerStatus) {
		status.LastRunAt = now.Format(time.RFC3339)
		status.LastTaskID = taskID
		status.LastProbeStatus = "completed"
		status.LastDNSStatus = "skipped"
		status.LastGitHubStatus = "skipped"
		status.LastMessage = fmt.Sprintf("定时测速完成，结果 %d 条。", len(result.Results))
		status.WorkflowStage = "post_run_profiles"
		status.ConfigSource = configSource
		status.LastProfileAction = profileAction
		status.LastSourceProfileAction = sourceProfileAction
	})
	if len(result.Results) == 0 {
		a.setSchedulerStatus(func(status *SchedulerStatus) {
			status.WorkflowStage = "completed"
		})
		return
	}
	if cfg.AutoDNSPush {
		a.setSchedulerStatus(func(status *SchedulerStatus) {
			status.WorkflowStage = "dns"
		})
		dnsResult := a.PushCloudflareDNSRecords(map[string]any{
			"config": snapshot,
			"ipsRaw": probeRowsIPList(result.Results),
		})
		a.setSchedulerStatus(func(status *SchedulerStatus) {
			if dnsResult.OK {
				status.LastDNSStatus = "completed"
			} else {
				status.LastDNSStatus = "failed"
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
		_, err := exportProbeRowsToGitHub(ctx, snapshot, taskID, result.Results, time.Now())
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

func updateRecentRunProfile(snapshot map[string]any) string {
	if len(snapshot) == 0 {
		return "skipped"
	}
	store, err := loadProfileStore()
	if err != nil {
		return "failed"
	}
	now := time.Now().Format(time.RFC3339)
	updated := false
	for index := range store.Items {
		if store.Items[index].ID != recentRunProfileID {
			continue
		}
		store.Items[index].ConfigSnapshot = sanitizeDesktopConfigSnapshot(snapshot)
		store.Items[index].Name = recentRunProfileName
		if store.Items[index].CreatedAt == "" {
			store.Items[index].CreatedAt = now
		}
		store.Items[index].UpdatedAt = now
		updated = true
		break
	}
	if !updated {
		store.Items = append(store.Items, profileItem{
			ConfigSnapshot: sanitizeDesktopConfigSnapshot(snapshot),
			CreatedAt:      now,
			ID:             recentRunProfileID,
			Name:           recentRunProfileName,
			UpdatedAt:      now,
		})
	}
	if err := saveProfileStore(store); err != nil {
		return "failed"
	}
	if updated {
		return "updated"
	}
	return "created"
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
	exportCfg := mapValue(snapshot["export"])
	githubCfg := mapValue(exportCfg["github"])
	if len(githubCfg) == 0 {
		githubCfg = mapValue(snapshot["github"])
	}
	return boolValue(githubCfg["enabled"], false)
}

func schedulerConfigFromSnapshot(snapshot map[string]any) SchedulerConfig {
	raw := mapValue(snapshot["scheduler"])
	return SchedulerConfig{
		Enabled:                    boolValue(raw["enabled"], false),
		IntervalMinutes:            intValue(firstNonNil(raw["interval_minutes"], raw["intervalMinutes"]), 0),
		DailyTimes:                 stringSliceValue(firstNonNil(raw["daily_times"], raw["dailyTimes"])),
		AutoDNSPush:                boolValue(firstNonNil(raw["auto_dns_push"], raw["autoDnsPush"]), true),
		AutoGitHubExport:           boolValue(firstNonNil(raw["auto_github_export"], raw["autoGithubExport"]), true),
		SkipIfActive:               boolValue(firstNonNil(raw["skip_if_active"], raw["skipIfActive"]), true),
		ConfigSource:               stringValue(firstNonNil(raw["config_source"], raw["configSource"]), defaultSchedulerConfigSource),
		PostRunProfileAction:       stringValue(firstNonNil(raw["post_run_profile_action"], raw["postRunProfileAction"]), defaultSchedulerProfileAction),
		PostRunSourceProfileAction: stringValue(firstNonNil(raw["post_run_source_profile_action"], raw["postRunSourceProfileAction"]), defaultSchedulerSourceProfileAction),
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
	var value int
	if _, err := fmt.Sscanf(strings.TrimSpace(raw), "%d", &value); err != nil {
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
	a.ensureProbeControl()
	a.probeControlMu.Lock()
	defer a.probeControlMu.Unlock()
	return a.currentTaskID != "" || a.pausedTaskID != "" || a.pauseRequested
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
