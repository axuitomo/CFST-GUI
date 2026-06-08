package app

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/appcore"
	"github.com/axuitomo/CFST-GUI/internal/utils"
)

func TestNextSchedulerRun(t *testing.T) {
	location := time.FixedZone("test", 8*60*60)
	now := time.Date(2026, 5, 9, 10, 30, 0, 0, location)

	tests := []struct {
		name    string
		cfg     SchedulerConfig
		lastRun time.Time
		want    time.Time
	}{
		{
			name: "disabled",
			cfg:  SchedulerConfig{Enabled: false, IntervalMinutes: 30},
		},
		{
			name: "no rules",
			cfg:  SchedulerConfig{Enabled: true},
		},
		{
			name: "interval without last run",
			cfg:  SchedulerConfig{Enabled: true, IntervalMinutes: 30},
			want: now.Add(30 * time.Minute),
		},
		{
			name:    "interval advances from last run",
			cfg:     SchedulerConfig{Enabled: true, IntervalMinutes: 30},
			lastRun: now.Add(-75 * time.Minute),
			want:    time.Date(2026, 5, 9, 10, 45, 0, 0, location),
		},
		{
			name: "daily future today",
			cfg:  SchedulerConfig{Enabled: true, DailyTimes: []string{"11:15"}},
			want: time.Date(2026, 5, 9, 11, 15, 0, 0, location),
		},
		{
			name: "daily rolls to next day",
			cfg:  SchedulerConfig{Enabled: true, DailyTimes: []string{"09:00"}},
			want: time.Date(2026, 5, 10, 9, 0, 0, 0, location),
		},
		{
			name: "earliest interval or daily",
			cfg:  SchedulerConfig{Enabled: true, IntervalMinutes: 120, DailyTimes: []string{"10:45"}},
			want: time.Date(2026, 5, 9, 10, 45, 0, 0, location),
		},
		{
			name: "invalid daily time ignored",
			cfg:  SchedulerConfig{Enabled: true, DailyTimes: []string{"bad", "25:00", "10:31:05"}},
			want: time.Date(2026, 5, 9, 10, 31, 5, 0, location),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := nextSchedulerRun(now, tc.lastRun, tc.cfg)
			if !got.Equal(tc.want) {
				t.Fatalf("nextSchedulerRun() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestSchedulerPipelineTargetIDsForRun(t *testing.T) {
	workspace := pipelineWorkspace{
		ActiveTemplateID: "template-a",
		Templates: []pipelineTemplateItem{
			{ID: "template-a", Name: "Template A", BoundConfigSnapshot: map[string]any{"cloudflare": map[string]any{"record_name": "a.example.com"}}},
			{ID: "template-b", Name: "Template B", BoundConfigSnapshot: map[string]any{"cloudflare": map[string]any{"record_name": "b.example.com"}}},
		},
		Targets: []pipelineTargetItem{
			{Enabled: true, ID: "target-a", Name: "A", TemplateID: "template-a", ConfigSnapshot: map[string]any{"cloudflare": map[string]any{"record_name": "a.example.com"}}},
			{Enabled: true, ID: "target-c", Name: "C", TemplateID: "template-a", ConfigSnapshot: map[string]any{"cloudflare": map[string]any{"record_name": "c.example.com"}}},
			{Enabled: false, ID: "target-disabled", Name: "Disabled", TemplateID: "template-a", ConfigSnapshot: map[string]any{"cloudflare": map[string]any{"record_name": "disabled.example.com"}}},
			{Enabled: true, ID: "target-empty", Name: "Empty", TemplateID: "template-a"},
			{Enabled: true, ID: "target-b", Name: "B", TemplateID: "template-b", ConfigSnapshot: map[string]any{"cloudflare": map[string]any{"record_name": "b.example.com"}}},
		},
	}

	targetIDs, err := schedulerPipelineTargetIDsForRun(workspace, "template-a")
	if err != nil {
		t.Fatalf("schedulerPipelineTargetIDsForRun(default) error = %v", err)
	}
	if got, want := strings.Join(targetIDs, ","), "target-a,target-c"; got != want {
		t.Fatalf("schedulerPipelineTargetIDsForRun(default) ids = %q, want %q", got, want)
	}
}

func TestSchedulerConfigFromSnapshotIgnoresLegacySelector(t *testing.T) {
	cfg := schedulerConfigFromSnapshot(map[string]any{
		"scheduler": map[string]any{
			"enabled": true,
			"pipeline_target_selector": map[string]any{
				"mode":                "tags_any",
				"explicit_target_ids": []string{"target-a"},
				"tags_any":            []string{"night"},
			},
			"pipeline_template_id": "template-a",
			"run_mode":             "pipeline",
		},
	})
	if len(cfg.legacySelectorWarnings) != 1 || !strings.Contains(cfg.legacySelectorWarnings[0], "忽略旧版目标选择器") {
		t.Fatalf("legacySelectorWarnings = %#v, want legacy selector warning", cfg.legacySelectorWarnings)
	}
}

func TestSaveDesktopConfigReloadsPipelineSchedulerStatus(t *testing.T) {
	isolateStorageForTest(t)
	app := NewApp()
	snapshot := defaultDesktopConfigSnapshot()
	snapshot["scheduler"] = map[string]any{
		"enabled":              true,
		"interval_minutes":     60,
		"pipeline_template_id": "template-a",
		"run_mode":             "pipeline",
	}

	if result := app.SaveDesktopConfig(map[string]any{"config_snapshot": snapshot}); !result.OK {
		t.Fatalf("SaveDesktopConfig failed: %#v", result)
	}

	status := app.currentSchedulerStatus()
	if !status.Enabled {
		t.Fatalf("scheduler status enabled = false, want true: %#v", status)
	}
	if status.NextRunAt == "" {
		t.Fatalf("scheduler NextRunAt is empty, want scheduled time: %#v", status)
	}
	if status.LastMessage != "定时任务已启用。" {
		t.Fatalf("LastMessage = %q, want enabled message", status.LastMessage)
	}
}

func TestRunScheduledProbeSkipsWhenActive(t *testing.T) {
	app := NewApp()
	if ok, _ := app.setCurrentProbeTask("manual-task", nil); !ok {
		t.Fatal("setCurrentProbeTask returned false")
	}
	defer app.clearCurrentProbeTask("manual-task")

	app.runScheduledProbe(context.Background(), SchedulerConfig{
		Enabled:      true,
		SkipIfActive: true,
	})

	status := app.currentSchedulerStatus()
	if status.LastProbeStatus != "skipped" {
		t.Fatalf("LastProbeStatus = %q, want skipped", status.LastProbeStatus)
	}
	if status.LastTaskID == "" || status.LastRunAt == "" {
		t.Fatalf("scheduler status missing task/run metadata: %#v", status)
	}
	if status.LastDNSStatus != "" || status.LastGitHubStatus != "" {
		t.Fatalf("downstream statuses = (%q,%q), want empty", status.LastDNSStatus, status.LastGitHubStatus)
	}
}

func TestSchedulerSnapshotForRunPrefersNewerDraft(t *testing.T) {
	isolateStorageForTest(t)
	app := NewApp()
	savedAt := time.Date(2026, 5, 9, 10, 0, 0, 0, time.FixedZone("test", 8*60*60))
	savedSnapshot := defaultDesktopConfigSnapshot()
	mapValue(savedSnapshot["cloudflare"])["record_name"] = "saved.example.com"
	if result := app.SaveDesktopConfig(map[string]any{"config_snapshot": savedSnapshot}); !result.OK {
		t.Fatalf("SaveDesktopConfig failed: %#v", result)
	}
	rewriteSavedAtForTest(t, desktopConfigFilePath(), savedAt)

	draftSnapshot := defaultDesktopConfigSnapshot()
	mapValue(draftSnapshot["cloudflare"])["record_name"] = "draft.example.com"
	if result := app.SaveDesktopDraft(map[string]any{"config_snapshot": draftSnapshot}); !result.OK {
		t.Fatalf("SaveDesktopDraft failed: %#v", result)
	}
	rewriteSavedAtForTest(t, desktopDraftFilePath(), savedAt.Add(time.Second))

	got, source, err := schedulerSnapshotForRun(SchedulerConfig{ConfigSource: defaultSchedulerConfigSource})
	if err != nil {
		t.Fatalf("schedulerSnapshotForRun returned error: %v", err)
	}
	if source != "draft" {
		t.Fatalf("source = %q, want draft", source)
	}
	if gotName := stringValue(mapValue(got["cloudflare"])["record_name"], ""); gotName != "draft.example.com" {
		t.Fatalf("record_name = %q, want draft snapshot", gotName)
	}
}

func TestSchedulerSnapshotForRunFallsBackToSavedConfig(t *testing.T) {
	isolateStorageForTest(t)
	app := NewApp()
	savedSnapshot := defaultDesktopConfigSnapshot()
	mapValue(savedSnapshot["cloudflare"])["record_name"] = "saved-only.example.com"
	if result := app.SaveDesktopConfig(map[string]any{"config_snapshot": savedSnapshot}); !result.OK {
		t.Fatalf("SaveDesktopConfig failed: %#v", result)
	}

	got, source, err := schedulerSnapshotForRun(SchedulerConfig{ConfigSource: defaultSchedulerConfigSource})
	if err != nil {
		t.Fatalf("schedulerSnapshotForRun returned error: %v", err)
	}
	if source != "saved" {
		t.Fatalf("source = %q, want saved", source)
	}
	if gotName := stringValue(mapValue(got["cloudflare"])["record_name"], ""); gotName != "saved-only.example.com" {
		t.Fatalf("record_name = %q, want saved snapshot", gotName)
	}
}

func TestRunScheduledProbePassesConfigSourceToTaskContext(t *testing.T) {
	oldTCP := desktopTCPProbeRunner
	oldTrace := desktopTraceProbeRunner
	oldDownload := desktopDownloadProbeRunner
	t.Cleanup(func() {
		desktopTCPProbeRunner = oldTCP
		desktopTraceProbeRunner = oldTrace
		desktopDownloadProbeRunner = oldDownload
	})
	desktopTCPProbeRunner = func() (utils.PingDelaySet, error) {
		return utils.PingDelaySet{{
			PingData: &utils.PingData{
				IP:       parseTestIP("1.1.1.1"),
				Sended:   3,
				Received: 3,
				Delay:    10 * time.Millisecond,
			},
		}}, nil
	}
	desktopTraceProbeRunner = func(input utils.PingDelaySet) utils.PingDelaySet {
		return input
	}
	desktopDownloadProbeRunner = func(input utils.PingDelaySet) utils.DownloadSpeedSet {
		return utils.DownloadSpeedSet(input)
	}

	for _, tc := range []struct {
		name       string
		withDraft  bool
		wantSource string
	}{
		{name: "saved fallback", wantSource: "saved"},
		{name: "newer draft", withDraft: true, wantSource: "draft"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			isolateStorageForTest(t)
			app := NewApp()
			savedSnapshot := defaultDesktopConfigSnapshot()
			mapValue(savedSnapshot["probe"])["disable_download"] = true
			savedSnapshot["sources"] = []any{
				map[string]any{
					"content": "1.1.1.1",
					"enabled": true,
					"id":      "scheduled-source",
					"kind":    "inline",
					"name":    "Scheduled Source",
				},
			}
			if result := app.SaveDesktopConfig(map[string]any{"config_snapshot": savedSnapshot}); !result.OK {
				t.Fatalf("SaveDesktopConfig failed: %#v", result)
			}
			savedAt := time.Date(2026, 5, 9, 10, 0, 0, 0, time.FixedZone("test", 8*60*60))
			rewriteSavedAtForTest(t, desktopConfigFilePath(), savedAt)
			if tc.withDraft {
				draftSnapshot := defaultDesktopConfigSnapshot()
				mapValue(draftSnapshot["probe"])["disable_download"] = true
				draftSnapshot["sources"] = savedSnapshot["sources"]
				mapValue(draftSnapshot["cloudflare"])["record_name"] = "draft.example.com"
				if result := app.SaveDesktopDraft(map[string]any{"config_snapshot": draftSnapshot}); !result.OK {
					t.Fatalf("SaveDesktopDraft failed: %#v", result)
				}
				rewriteSavedAtForTest(t, desktopDraftFilePath(), savedAt.Add(time.Second))
			}

			app.runScheduledProbe(context.Background(), SchedulerConfig{
				AutoDNSPush:      false,
				AutoGitHubExport: false,
				ConfigSource:     defaultSchedulerConfigSource,
			})
			status := app.currentSchedulerStatus()
			if status.LastProbeStatus != "completed" {
				t.Fatalf("LastProbeStatus = %q, want completed; message=%s", status.LastProbeStatus, status.LastMessage)
			}
			snapshot, _, err := schedulerSnapshotForRun(SchedulerConfig{ConfigSource: defaultSchedulerConfigSource})
			if err != nil {
				t.Fatalf("schedulerSnapshotForRun returned error: %v", err)
			}
			probeResult, err := app.RunDesktopProbe(DesktopProbePayload{
				Config:       snapshot,
				ConfigSource: status.ConfigSource,
				Sources:      desktopSourcesFromAny(snapshot["sources"]),
				TaskID:       "direct-" + tc.name,
			})
			if err != nil {
				t.Fatalf("RunDesktopProbe returned error: %v", err)
			}
			if got := probeResult.TaskContext.ConfigSource; got != tc.wantSource {
				t.Fatalf("task_context.config_source = %q, want %q; result=%#v", got, tc.wantSource, probeResult.TaskContext)
			}
			if strings.TrimSpace(status.ConfigSource) != tc.wantSource {
				t.Fatalf("scheduler status config source = %q, want %q", status.ConfigSource, tc.wantSource)
			}
		})
	}
}

func TestRunScheduledProbePipelineModeRunsEnabledProfiles(t *testing.T) {
	isolateStorageForTest(t)
	oldTCP := desktopTCPProbeRunner
	oldTrace := desktopTraceProbeRunner
	oldDownload := desktopDownloadProbeRunner
	t.Cleanup(func() {
		desktopTCPProbeRunner = oldTCP
		desktopTraceProbeRunner = oldTrace
		desktopDownloadProbeRunner = oldDownload
	})
	desktopTCPProbeRunner = func() (utils.PingDelaySet, error) {
		return utils.PingDelaySet{{
			PingData: &utils.PingData{
				IP:       parseTestIP("1.1.1.1"),
				Sended:   3,
				Received: 3,
				Delay:    10 * time.Millisecond,
			},
		}}, nil
	}
	desktopTraceProbeRunner = func(input utils.PingDelaySet) utils.PingDelaySet {
		return input
	}
	desktopDownloadProbeRunner = func(input utils.PingDelaySet) utils.DownloadSpeedSet {
		return utils.DownloadSpeedSet(input)
	}

	app := NewApp()
	snapshot := defaultDesktopConfigSnapshot()
	mapValue(snapshot["probe"])["disable_download"] = true
	snapshot["sources"] = []any{
		map[string]any{
			"content": "1.1.1.1",
			"enabled": true,
			"id":      "scheduled-pipeline-source",
			"kind":    "inline",
			"name":    "Scheduled Pipeline Source",
		},
	}
	now := time.Now().Format(time.RFC3339)
	workspace := normalizePipelineWorkspaceForSave(pipelineWorkspace{
		ActiveTemplateID: appcore.DefaultPipelineTemplateID,
		SchemaVersion:    pipelineWorkspaceSchemaVersion,
		Templates:        []pipelineTemplateItem{appcore.DefaultPipelineTemplate(now)},
		Targets: []pipelineTargetItem{
			{
				ConfigSnapshot: sanitizeDesktopConfigSnapshot(snapshot),
				CreatedAt:      now,
				DNSPushPolicy:  appcore.PipelineDNSPushPolicyAuto,
				Domain:         "jp.example.com",
				Enabled:        true,
				ID:             "pipeline-jp",
				Name:           "日本",
				Region:         "JP",
				TemplateID:     appcore.DefaultPipelineTemplateID,
				UpdatedAt:      now,
			},
			{
				ConfigSnapshot: sanitizeDesktopConfigSnapshot(snapshot),
				CreatedAt:      now,
				DNSPushPolicy:  appcore.PipelineDNSPushPolicyAuto,
				Domain:         "sg.example.com",
				Enabled:        true,
				ID:             "pipeline-sg",
				Name:           "新加坡",
				Region:         "SG",
				TemplateID:     appcore.DefaultPipelineTemplateID,
				UpdatedAt:      now,
			},
			{
				ConfigSnapshot: sanitizeDesktopConfigSnapshot(snapshot),
				CreatedAt:      now,
				DNSPushPolicy:  appcore.PipelineDNSPushPolicyAuto,
				Domain:         "us.example.com",
				Enabled:        false,
				ID:             "pipeline-us",
				Name:           "美国",
				Region:         "US",
				TemplateID:     appcore.DefaultPipelineTemplateID,
				UpdatedAt:      now,
			},
		},
	})
	if err := savePipelineWorkspace(workspace); err != nil {
		t.Fatalf("savePipelineWorkspace failed: %v", err)
	}

	app.runScheduledProbe(context.Background(), SchedulerConfig{
		AutoDNSPush:      false,
		AutoGitHubExport: false,
		RunMode:          "pipeline",
	})

	status := app.currentSchedulerStatus()
	if status.LastProbeStatus != "completed" {
		t.Fatalf("LastProbeStatus = %q, want completed; status=%#v", status.LastProbeStatus, status)
	}
	if status.ConfigSource != "pipeline_workspace" {
		t.Fatalf("ConfigSource = %q, want pipeline_workspace", status.ConfigSource)
	}
	if status.LastDNSStatus != "skipped" {
		t.Fatalf("LastDNSStatus = %q, want skipped", status.LastDNSStatus)
	}
	if status.LastTaskID == "" {
		t.Fatalf("LastTaskID should be set: %#v", status)
	}

	snapshotResult := app.GetPipelineSnapshot(map[string]any{"pipeline_id": status.LastTaskID})
	if !snapshotResult.OK {
		t.Fatalf("GetPipelineSnapshot failed: %#v", snapshotResult)
	}
	result, ok := snapshotResult.Data.(PipelineRunResult)
	if !ok {
		t.Fatalf("snapshot data type = %T, want PipelineRunResult", snapshotResult.Data)
	}
	if result.Total != 2 {
		t.Fatalf("pipeline total = %d, want 2 bound profiles", result.Total)
	}
	if len(result.Results) != 2 {
		t.Fatalf("pipeline results = %#v, want exactly two bound profiles", result.Results)
	}
	resultIDs := []string{result.Results[0].ProfileID, result.Results[1].ProfileID}
	if got, want := strings.Join(resultIDs, ","), "pipeline-jp,pipeline-sg"; got != want {
		t.Fatalf("pipeline profile ids = %q, want %q", got, want)
	}
}

func TestRunScheduledPipelineRecoversFromPanic(t *testing.T) {
	isolateStorageForTest(t)
	oldTCP := desktopTCPProbeRunner
	oldTrace := desktopTraceProbeRunner
	t.Cleanup(func() {
		desktopTCPProbeRunner = oldTCP
		desktopTraceProbeRunner = oldTrace
	})
	desktopTCPProbeRunner = func() (utils.PingDelaySet, error) {
		panic("scheduled pipeline tcp boom")
	}
	desktopTraceProbeRunner = func(input utils.PingDelaySet) utils.PingDelaySet {
		t.Fatal("trace runner should not run after TCP panic")
		return nil
	}

	app := NewApp()
	snapshot := defaultDesktopConfigSnapshot()
	mapValue(snapshot["probe"])["disable_download"] = true
	snapshot["sources"] = []any{
		map[string]any{
			"content": "1.1.1.1",
			"enabled": true,
			"id":      "scheduled-pipeline-panic-source",
			"kind":    "inline",
			"name":    "Scheduled Pipeline Panic Source",
		},
	}
	now := time.Now().Format(time.RFC3339)
	workspace := normalizePipelineWorkspaceForSave(pipelineWorkspace{
		ActiveTemplateID: appcore.DefaultPipelineTemplateID,
		SchemaVersion:    pipelineWorkspaceSchemaVersion,
		Templates:        []pipelineTemplateItem{appcore.DefaultPipelineTemplate(now)},
		Targets: []pipelineTargetItem{
			{
				ConfigSnapshot: sanitizeDesktopConfigSnapshot(snapshot),
				CreatedAt:      now,
				DNSPushPolicy:  appcore.PipelineDNSPushPolicySkip,
				Enabled:        true,
				ID:             "pipeline-panic",
				Name:           "Panic",
				TemplateID:     appcore.DefaultPipelineTemplateID,
				UpdatedAt:      now,
			},
		},
	})
	if err := savePipelineWorkspace(workspace); err != nil {
		t.Fatalf("savePipelineWorkspace failed: %v", err)
	}

	app.runScheduledProbe(context.Background(), SchedulerConfig{
		AutoDNSPush:      false,
		AutoGitHubExport: false,
		RunMode:          "pipeline",
	})

	status := app.currentSchedulerStatus()
	if status.LastProbeStatus != "failed" {
		t.Fatalf("LastProbeStatus = %q, want failed; status=%#v", status.LastProbeStatus, status)
	}
	if !strings.Contains(status.LastMessage, "scheduled pipeline tcp boom") {
		t.Fatalf("LastMessage = %q, want panic detail", status.LastMessage)
	}
	if current := app.activePipelineID(); current != "" {
		t.Fatalf("active pipeline = %q, want cleared", current)
	}
	snapshotResult := app.GetPipelineSnapshot(map[string]any{"pipeline_id": status.LastTaskID})
	if !snapshotResult.OK {
		t.Fatalf("GetPipelineSnapshot failed: %#v", snapshotResult)
	}
	result, ok := snapshotResult.Data.(PipelineRunResult)
	if !ok {
		t.Fatalf("snapshot data type = %T, want PipelineRunResult", snapshotResult.Data)
	}
	if result.Status != "failed" {
		t.Fatalf("pipeline status = %q, want failed", result.Status)
	}
}

func TestRunPipelineReturnsDesktopCommandResult(t *testing.T) {
	isolateStorageForTest(t)
	oldTCP := desktopTCPProbeRunner
	oldTrace := desktopTraceProbeRunner
	oldDownload := desktopDownloadProbeRunner
	t.Cleanup(func() {
		desktopTCPProbeRunner = oldTCP
		desktopTraceProbeRunner = oldTrace
		desktopDownloadProbeRunner = oldDownload
	})
	desktopTCPProbeRunner = func() (utils.PingDelaySet, error) {
		return utils.PingDelaySet{{
			PingData: &utils.PingData{
				IP:       parseTestIP("1.1.1.1"),
				Sended:   3,
				Received: 3,
				Delay:    10 * time.Millisecond,
			},
		}}, nil
	}
	desktopTraceProbeRunner = func(input utils.PingDelaySet) utils.PingDelaySet {
		return input
	}
	desktopDownloadProbeRunner = func(input utils.PingDelaySet) utils.DownloadSpeedSet {
		return utils.DownloadSpeedSet(input)
	}

	app := NewApp()
	snapshot := defaultDesktopConfigSnapshot()
	mapValue(snapshot["probe"])["disable_download"] = true
	snapshot["sources"] = []any{
		map[string]any{
			"content": "1.1.1.1",
			"enabled": true,
			"id":      "pipeline-command-source",
			"kind":    "inline",
			"name":    "Pipeline Command Source",
		},
	}
	now := time.Now().Format(time.RFC3339)
	command := app.RunPipeline(PipelineRunPayload{
		PipelineID: "pipeline-command",
		Profiles: []PipelineProfile{
			{
				ConfigSnapshot: sanitizeDesktopConfigSnapshot(snapshot),
				CreatedAt:      now,
				DNSPushPolicy:  appcore.PipelineDNSPushPolicySkip,
				Domain:         "jp.example.com",
				Enabled:        true,
				ID:             "pipeline-command-profile",
				Name:           "日本",
				Region:         "JP",
				UpdatedAt:      now,
			},
		},
		SchedulerOverrides: appcore.PipelineRuntimeOverrides{
			AllowGitHubExport: boolPointer(false),
		},
		TaskID: "pipeline-command",
	})
	if !command.OK || command.Code != "PIPELINE_COMPLETED" {
		t.Fatalf("RunPipeline command = %#v, want PIPELINE_COMPLETED ok", command)
	}
	result, ok := command.Data.(PipelineRunResult)
	if !ok {
		t.Fatalf("RunPipeline data type = %T, want PipelineRunResult", command.Data)
	}
	if result.PipelineID != "pipeline-command" || result.Total != 1 {
		t.Fatalf("RunPipeline data = %#v, want command pipeline result", result)
	}

	if ok, _ := app.claimPipeline("active-pipeline"); !ok {
		t.Fatal("claimPipeline returned false")
	}
	blocked := app.RunPipeline(PipelineRunPayload{PipelineID: "blocked-pipeline"})
	app.clearPipeline("active-pipeline")
	if blocked.OK || blocked.Code != "PIPELINE_ALREADY_RUNNING" || blocked.Data != nil {
		t.Fatalf("blocked RunPipeline command = %#v, want PIPELINE_ALREADY_RUNNING without data", blocked)
	}
}

func TestPipelineResultsKeepOnlyMostRecentRun(t *testing.T) {
	app := NewApp()
	app.rememberPipelineResult(PipelineRunResult{
		PipelineID: "pipeline-old",
		Status:     "completed",
		TaskID:     "pipeline-old",
	})
	app.rememberPipelineResult(PipelineRunResult{
		PipelineID: "pipeline-new",
		Status:     "failed",
		TaskID:     "pipeline-new",
	})

	if len(app.pipelineResults) != 1 {
		t.Fatalf("pipelineResults len = %d, want 1 recent result", len(app.pipelineResults))
	}
	if _, ok := app.pipelineResults["pipeline-new"]; !ok {
		t.Fatalf("recent pipeline result missing: %#v", app.pipelineResults)
	}
	if _, ok := app.pipelineResults["pipeline-old"]; ok {
		t.Fatalf("old pipeline result should be replaced: %#v", app.pipelineResults)
	}

	listResult := app.ListPipelineResults(map[string]any{})
	if !listResult.OK {
		t.Fatalf("ListPipelineResults failed: %#v", listResult)
	}
	results, ok := listResult.Data.([]PipelineRunResult)
	if !ok {
		t.Fatalf("ListPipelineResults data type = %T, want []PipelineRunResult", listResult.Data)
	}
	if len(results) != 1 || results[0].PipelineID != "pipeline-new" {
		t.Fatalf("ListPipelineResults data = %#v, want only recent pipeline-new result", results)
	}

	oldSnapshot := app.GetPipelineSnapshot(map[string]any{"pipeline_id": "pipeline-old"})
	if oldSnapshot.OK {
		t.Fatalf("GetPipelineSnapshot(old) = %#v, want not found", oldSnapshot)
	}
	latestSnapshot := app.GetPipelineSnapshot(map[string]any{})
	if !latestSnapshot.OK {
		t.Fatalf("GetPipelineSnapshot(latest) failed: %#v", latestSnapshot)
	}
	latest, ok := latestSnapshot.Data.(PipelineRunResult)
	if !ok {
		t.Fatalf("GetPipelineSnapshot(latest) data type = %T, want PipelineRunResult", latestSnapshot.Data)
	}
	if latest.PipelineID != "pipeline-new" {
		t.Fatalf("GetPipelineSnapshot(latest) = %#v, want pipeline-new", latest)
	}

	if ok, _ := app.claimPipeline("pipeline-current"); !ok {
		t.Fatal("claimPipeline returned false")
	}
	if len(app.pipelineResults) != 0 {
		t.Fatalf("pipelineResults should be cleared on new run claim: %#v", app.pipelineResults)
	}
	app.clearPipeline("pipeline-current")
}

func TestStartPipelineEmitsFailureWhenAsyncWorkerPanics(t *testing.T) {
	isolateStorageForTest(t)
	app := NewApp()
	app.eventHub = newWebUIEventHub()
	events, unsubscribe := app.eventHub.subscribe()
	t.Cleanup(func() {
		unsubscribe()
	})

	oldTCP := desktopTCPProbeRunner
	t.Cleanup(func() {
		desktopTCPProbeRunner = oldTCP
	})
	desktopTCPProbeRunner = func() (utils.PingDelaySet, error) {
		panic("boom")
	}

	snapshot := defaultDesktopConfigSnapshot()
	mapValue(snapshot["probe"])["disable_download"] = true
	snapshot["sources"] = []any{
		map[string]any{
			"content": "1.1.1.1",
			"enabled": true,
			"id":      "async-panic-source",
			"kind":    "inline",
			"name":    "Async Panic Source",
		},
	}
	now := time.Now().Format(time.RFC3339)
	template := appcore.DefaultPipelineTemplate(now)

	result := app.StartPipeline(PipelineRunPayload{
		PipelineID: "async-panic-pipeline",
		Profiles: []PipelineProfile{
			{
				ConfigSnapshot: sanitizeDesktopConfigSnapshot(snapshot),
				CreatedAt:      now,
				DNSPushPolicy:  appcore.PipelineDNSPushPolicySkip,
				Domain:         "panic.example.com",
				Enabled:        true,
				ID:             "panic-profile",
				Name:           "Panic Profile",
				Region:         "panic",
				UpdatedAt:      now,
			},
		},
		TaskID:     "async-panic-pipeline",
		TemplateID: template.ID,
		Workspace: pipelineWorkspace{
			ActiveTemplateID: template.ID,
			Templates:        []pipelineTemplateItem{template},
		},
	})
	if !result.OK || result.Code != "PIPELINE_ACCEPTED" {
		t.Fatalf("StartPipeline result = %#v, want PIPELINE_ACCEPTED", result)
	}

	failed := waitForProbeEvent(t, events, "async-panic-pipeline", "pipeline.failed")
	if got := stringValue(failed.Payload["pipeline_id"], ""); got != "async-panic-pipeline" {
		t.Fatalf("pipeline_id = %q, want async-panic-pipeline", got)
	}
	if got := stringValue(failed.Payload["message"], ""); !strings.Contains(got, "异常退出") {
		t.Fatalf("message = %q, want async panic failure message", got)
	}

	time.Sleep(50 * time.Millisecond)
	pipelineSnapshot := app.GetPipelineSnapshot(map[string]any{"pipeline_id": "async-panic-pipeline"})
	if !pipelineSnapshot.OK {
		t.Fatalf("GetPipelineSnapshot = %#v, want ready failed snapshot", pipelineSnapshot)
	}
	run, ok := pipelineSnapshot.Data.(PipelineRunResult)
	if !ok {
		t.Fatalf("snapshot data type = %T, want PipelineRunResult", pipelineSnapshot.Data)
	}
	if run.Status != "failed" {
		t.Fatalf("snapshot status = %q, want failed", run.Status)
	}
}

func TestSchedulerRecentRunSourceProfileUsesFixedID(t *testing.T) {
	isolateStorageForTest(t)
	sources := []DesktopSource{sourceProfileTestSource("recent-source", "Recent Source")}

	if action := updateRecentRunSourceProfile(sources); action != "created" {
		t.Fatalf("source profile action = %q, want created", action)
	}

	sourceProfiles, err := loadSourceProfileStore()
	if err != nil {
		t.Fatalf("load source profiles: %v", err)
	}
	if len(sourceProfiles.Items) != 1 || sourceProfiles.Items[0].ID != recentRunSourceProfileID {
		t.Fatalf("source profiles = %#v, want fixed recent-run source profile", sourceProfiles.Items)
	}
	if len(sourceProfiles.Items[0].Sources) != 1 || sourceProfiles.Items[0].Sources[0].Name != "Recent Source" {
		t.Fatalf("recent source profile sources = %#v, want saved sources", sourceProfiles.Items[0].Sources)
	}

	if action := updateRecentRunSourceProfile([]DesktopSource{sourceProfileTestSource("recent-source-2", "Recent Source 2")}); action != "updated" {
		t.Fatalf("source profile action = %q, want updated", action)
	}
}

func TestGitHubExportEnabledFromSnapshot(t *testing.T) {
	if githubExportEnabledFromSnapshot(map[string]any{}) {
		t.Fatal("empty snapshot should not enable GitHub export")
	}
	if !githubExportEnabledFromSnapshot(map[string]any{
		"export": map[string]any{
			"github": map[string]any{
				"enabled": true,
			},
		},
	}) {
		t.Fatal("export.github.enabled=true should enable GitHub export")
	}
	if !githubExportEnabledFromSnapshot(map[string]any{
		"github": map[string]any{
			"enabled": "true",
		},
	}) {
		t.Fatal("legacy github.enabled=true should enable GitHub export")
	}
}
