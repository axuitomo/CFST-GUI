package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/axuitomo/CFST-GUI/internal/appcore"
)

func TestPipelineRowsForNodeSourcePrefersExplicitProbeResults(t *testing.T) {
	runtimeCtx := &pipelineRuntimeContext{
		FilteredRows: []ProbeRow{
			{IP: "203.0.113.10"},
		},
		ProbeResult: &ProbeRunResult{
			Results: []ProbeRow{
				{IP: "198.51.100.1"},
				{IP: "198.51.100.2"},
			},
		},
	}

	rows := pipelineRowsForNodeSource(runtimeCtx, "probe_results")
	if got := len(rows); got != 2 {
		t.Fatalf("probe_results len = %d, want 2", got)
	}
	if rows[0].IP != "198.51.100.1" || rows[1].IP != "198.51.100.2" {
		t.Fatalf("probe_results rows = %#v, want original probe results", rows)
	}

	filtered := pipelineRowsForNodeSource(runtimeCtx, "filtered_rows")
	if got := len(filtered); got != 1 {
		t.Fatalf("filtered_rows len = %d, want 1", got)
	}
	if filtered[0].IP != "203.0.113.10" {
		t.Fatalf("filtered_rows = %#v, want filtered row", filtered)
	}
}

func TestPipelineProbeNodeOverridesSnapshotAndSources(t *testing.T) {
	snapshot := defaultDesktopConfigSnapshot()
	snapshot["sources"] = []any{
		map[string]any{
			"content":            "1.1.1.1",
			"enabled":            true,
			"id":                 "source-base",
			"ip_limit":           32,
			"ip_mode":            "traverse",
			"kind":               "inline",
			"name":               "base-source",
			"colo_filter":        "",
			"colo_filter_mode":   "allow",
			"last_fetched_at":    "",
			"last_fetched_count": 0,
			"path":               "",
			"status_text":        "",
			"url":                "",
		},
	}

	runtimeCtx := &pipelineRuntimeContext{
		ConfigSnapshot: snapshot,
	}
	node := appcore.PipelineNode{
		Action: appcore.PipelineNodeActionProbeTCP,
		Config: map[string]any{
			"concurrency_stage1":                123,
			"concurrency_stage2":                12,
			"concurrency_stage3":                2,
			"download_buffer_kb":                512,
			"download_count":                    5,
			"download_get_concurrency":          6,
			"download_http_protocol":            "h2",
			"download_speed_metric":             "max",
			"download_speed_sample_interval_ms": 250,
			"download_time_seconds":             8,
			"download_warmup_seconds":           2,
			"httping_cf_colo":                   "HKG,NRT",
			"httping_cf_colo_mode":              "deny",
			"httping_status_code":               204,
			"max_loss_rate":                     0.05,
			"max_tcp_latency_ms":                120,
			"max_trace_latency_ms":              250,
			"min_delay_ms":                      5,
			"min_download_mbps":                 3.5,
			"ping_times":                        6,
			"port_policy":                       "fixed_global",
			"print_num":                         3,
			"source_colo_filter":                "HKG",
			"source_colo_filter_mode":           "deny",
			"source_colo_filter_phase":          "stage2",
			"source_ip_limit":                   12,
			"source_ip_mode":                    "mcis",
			"source_mode":                       "custom",
			"stage3_limit":                      4,
			"sources": []any{
				map[string]any{
					"content":            "8.8.8.8",
					"enabled":            true,
					"id":                 "source-custom",
					"ip_limit":           4,
					"ip_mode":            "traverse",
					"kind":               "inline",
					"name":               "custom-source",
					"colo_filter":        "",
					"colo_filter_mode":   "allow",
					"last_fetched_at":    "",
					"last_fetched_count": 0,
					"path":               "",
					"status_text":        "",
					"url":                "",
				},
			},
			"tcp_port":          2053,
			"timeout_stage1_ms": 1500,
			"timeout_stage2_ms": 2200,
			"timeout_stage3_ms": 9000,
			"trace_colo_mode":   "trace_url",
			"trace_url":         "https://example.com/cdn-cgi/trace",
			"url":               "https://example.com/file.bin",
		},
	}

	effectiveSnapshot := pipelineProbeSnapshotForNode(runtimeCtx, node)
	probe := mapValue(effectiveSnapshot["probe"])
	concurrency := mapValue(probe["concurrency"])
	thresholds := mapValue(probe["thresholds"])
	stageLimits := mapValue(probe["stage_limits"])
	timeouts := mapValue(probe["timeouts"])
	if got := intValue(probe["tcp_port"], 0); got != 2053 {
		t.Fatalf("tcp_port = %d, want 2053", got)
	}
	if got := stringValue(probe["port_policy"], ""); got != "fixed_global" {
		t.Fatalf("port_policy = %q, want fixed_global", got)
	}
	if got := stringValue(probe["strategy"], ""); got != "full" {
		t.Fatalf("strategy = %q, want full for staged probe nodes", got)
	}
	if got := boolValue(probe["disable_download"], true); got {
		t.Fatalf("disable_download = true, want false for staged probe nodes")
	}
	if got := intValue(concurrency["stage1"], 0); got != 123 {
		t.Fatalf("concurrency stage1 = %d, want 123", got)
	}
	if got := intValue(concurrency["stage2"], 0); got != 12 {
		t.Fatalf("concurrency stage2 = %d, want 12", got)
	}
	if got := intValue(concurrency["stage3"], 0); got != 2 {
		t.Fatalf("concurrency stage3 = %d, want 2", got)
	}
	if got := intValue(probe["ping_times"], 0); got != 6 {
		t.Fatalf("ping_times = %d, want 6", got)
	}
	if got := intValue(probe["min_delay_ms"], 0); got != 5 {
		t.Fatalf("min_delay_ms = %d, want 5", got)
	}
	if got := intValue(timeouts["stage1_ms"], 0); got != 1500 {
		t.Fatalf("timeout stage1 = %d, want 1500", got)
	}
	if got := intValue(timeouts["stage2_ms"], 0); got != 2200 {
		t.Fatalf("timeout stage2 = %d, want 2200", got)
	}
	if got := intValue(timeouts["stage3_ms"], 0); got != 9000 {
		t.Fatalf("timeout stage3 = %d, want 9000", got)
	}
	if got := intValue(probe["download_count"], 0); got != 5 {
		t.Fatalf("download_count = %d, want 5", got)
	}
	if got := intValue(stageLimits["stage3"], 0); got != 4 {
		t.Fatalf("stage3 limit = %d, want 4", got)
	}
	if got := intValue(probe["print_num"], 0); got != 3 {
		t.Fatalf("print_num = %d, want 3", got)
	}
	if got := intValue(probe["download_get_concurrency"], 0); got != 6 {
		t.Fatalf("download_get_concurrency = %d, want 6", got)
	}
	if got := intValue(probe["download_time_seconds"], 0); got != 8 {
		t.Fatalf("download_time_seconds = %d, want 8", got)
	}
	if got := intValue(probe["download_warmup_seconds"], 0); got != 2 {
		t.Fatalf("download_warmup_seconds = %d, want 2", got)
	}
	if got := intValue(probe["download_speed_sample_interval_ms"], 0); got != 250 {
		t.Fatalf("download_speed_sample_interval_ms = %d, want 250", got)
	}
	if got := intValue(probe["download_buffer_kb"], 0); got != 512 {
		t.Fatalf("download_buffer_kb = %d, want 512", got)
	}
	if got := stringValue(probe["download_http_protocol"], ""); got != "h2" {
		t.Fatalf("download_http_protocol = %q, want h2", got)
	}
	if got := stringValue(probe["download_speed_metric"], ""); got != "max" {
		t.Fatalf("download_speed_metric = %q, want max", got)
	}
	if got := stringValue(probe["url"], ""); got != "https://example.com/file.bin" {
		t.Fatalf("url = %q, want file URL", got)
	}
	if got := intValue(thresholds["max_tcp_latency_ms"], 0); got != 120 {
		t.Fatalf("max_tcp_latency_ms = %d, want 120", got)
	}
	if got := intValue(thresholds["max_http_latency_ms"], 0); got != 250 {
		t.Fatalf("max_http_latency_ms = %d, want 250", got)
	}
	if got := floatValue(thresholds["min_download_mbps"], 0); got != 3.5 {
		t.Fatalf("min_download_mbps = %f, want 3.5", got)
	}
	if got := floatValue(probe["max_loss_rate"], 0); got != 0.05 {
		t.Fatalf("max_loss_rate = %f, want 0.05", got)
	}
	if got := stringValue(probe["trace_url"], ""); got != "https://example.com/cdn-cgi/trace" {
		t.Fatalf("trace_url = %q, want trace URL", got)
	}
	if got := stringValue(probe["trace_colo_mode"], ""); got != "trace_url" {
		t.Fatalf("trace_colo_mode = %q, want trace_url", got)
	}
	if got := stringValue(probe["source_colo_filter_phase"], ""); got != "stage2" {
		t.Fatalf("source_colo_filter_phase = %q, want stage2", got)
	}
	if got := intValue(probe["httping_status_code"], 0); got != 204 {
		t.Fatalf("httping_status_code = %d, want 204", got)
	}
	if got := stringValue(probe["httping_cf_colo"], ""); got != "HKG,NRT" {
		t.Fatalf("httping_cf_colo = %q, want HKG,NRT", got)
	}
	if got := stringValue(probe["httping_cf_colo_mode"], ""); got != "deny" {
		t.Fatalf("httping_cf_colo_mode = %q, want deny", got)
	}

	sources := pipelineProbeSourcesForNode(runtimeCtx, node)
	if got := len(sources); got != 1 {
		t.Fatalf("sources len = %d, want 1 custom source", got)
	}
	if sources[0].ID != "source-custom" {
		t.Fatalf("source id = %q, want source-custom", sources[0].ID)
	}
	if sources[0].IPLimit != 12 {
		t.Fatalf("source ip limit = %d, want 12", sources[0].IPLimit)
	}
	if sources[0].IPMode != "mcis" {
		t.Fatalf("source ip mode = %q, want mcis", sources[0].IPMode)
	}
	if sources[0].ColoFilter != "HKG" || sources[0].ColoFilterMode != "deny" {
		t.Fatalf("source colo override = (%q,%q), want (HKG,deny)", sources[0].ColoFilter, sources[0].ColoFilterMode)
	}
}

func TestPipelineProbeStageNodeForcesFullStrategy(t *testing.T) {
	snapshot := defaultDesktopConfigSnapshot()
	runtimeCtx := &pipelineRuntimeContext{
		ConfigSnapshot: snapshot,
	}
	effectiveSnapshot := pipelineProbeSnapshotForNode(runtimeCtx, appcore.PipelineNode{
		Action: appcore.PipelineNodeActionProbeDownload,
		Config: map[string]any{
			"disable_download": true,
			"strategy":         "fast",
		},
	})
	probe := mapValue(effectiveSnapshot["probe"])
	if got := stringValue(probe["strategy"], ""); got != "full" {
		t.Fatalf("strategy = %q, want full for staged probe node", got)
	}
	if got := boolValue(probe["disable_download"], true); got {
		t.Fatalf("disable_download = true, want false for staged probe node")
	}
}

func TestPipelineSourceGroupFiltersSourcesForProbeNode(t *testing.T) {
	snapshot := defaultDesktopConfigSnapshot()
	snapshot["sources"] = []DesktopSource{
		sourceProfileTestSource("source-a", "Source A"),
		sourceProfileTestSource("source-b", "Source B"),
	}
	runtimeCtx := &pipelineRuntimeContext{
		ConfigSnapshot: snapshot,
	}
	sourceNode := appcore.PipelineNode{
		Action: appcore.PipelineNodeActionSelectSources,
		Config: map[string]any{
			"source_ids":       []any{"source-b"},
			"source_selection": appcore.PipelineSourceSelectionCustom,
		},
	}
	if _, err := (&App{}).executeSelectSourcesNode(sourceNode, runtimeCtx); err != nil {
		t.Fatalf("executeSelectSourcesNode returned error: %v", err)
	}
	probeSources := pipelineProbeSourcesForNode(runtimeCtx, appcore.PipelineNode{
		Action: appcore.PipelineNodeActionProbeTCP,
		Config: map[string]any{"source_mode": "inherit"},
	})
	if got := len(probeSources); got != 1 {
		t.Fatalf("probe sources len = %d, want 1", got)
	}
	if probeSources[0].ID != "source-b" {
		t.Fatalf("probe source id = %q, want source-b", probeSources[0].ID)
	}
}

func TestPipelineSourceGroupUsesSourceProfileEnabledSources(t *testing.T) {
	isolateStorageForTest(t)
	sourceA := sourceProfileTestSource("source-a", "Source A")
	sourceB := sourceProfileTestSource("source-b", "Source B")
	sourceB.Enabled = false
	if err := saveSourceProfileStore(sourceProfileStore{
		ActiveProfileID: "profile-a",
		Items: []sourceProfileItem{
			{
				ID:      "profile-a",
				Name:    "输入组 A",
				Sources: []DesktopSource{sourceA, sourceB},
			},
		},
		SchemaVersion: sourceProfilesSchemaVersion,
	}); err != nil {
		t.Fatalf("saveSourceProfileStore: %v", err)
	}
	runtimeCtx := &pipelineRuntimeContext{ConfigSnapshot: defaultDesktopConfigSnapshot()}
	result, err := (&App{}).executeSelectSourcesNode(appcore.PipelineNode{
		Action: appcore.PipelineNodeActionSelectSources,
		Config: map[string]any{
			"source_profile_id": "profile-a",
			"source_selection":  appcore.PipelineSourceSelectionEnabled,
		},
	}, runtimeCtx)
	if err != nil {
		t.Fatalf("executeSelectSourcesNode returned error: %v", err)
	}
	if result.Status != "completed" {
		t.Fatalf("status = %q, want completed", result.Status)
	}
	if got := len(runtimeCtx.SelectedSources); got != 1 {
		t.Fatalf("selected sources len = %d, want 1 enabled source", got)
	}
	if runtimeCtx.SelectedSources[0].ID != "source-a" {
		t.Fatalf("selected source = %#v, want source-a", runtimeCtx.SelectedSources[0])
	}
}

func TestPipelineSourceGroupUsesSourceProfileCustomSourceIDs(t *testing.T) {
	isolateStorageForTest(t)
	sourceA := sourceProfileTestSource("source-a", "Source A")
	sourceB := sourceProfileTestSource("source-b", "Source B")
	sourceB.Enabled = false
	if err := saveSourceProfileStore(sourceProfileStore{
		ActiveProfileID: "profile-a",
		Items: []sourceProfileItem{
			{
				ID:      "profile-a",
				Name:    "输入组 A",
				Sources: []DesktopSource{sourceA, sourceB},
			},
		},
		SchemaVersion: sourceProfilesSchemaVersion,
	}); err != nil {
		t.Fatalf("saveSourceProfileStore: %v", err)
	}
	runtimeCtx := &pipelineRuntimeContext{ConfigSnapshot: defaultDesktopConfigSnapshot()}
	if _, err := (&App{}).executeSelectSourcesNode(appcore.PipelineNode{
		Action: appcore.PipelineNodeActionSelectSources,
		Config: map[string]any{
			"source_ids":        []any{"source-b"},
			"source_profile_id": "profile-a",
			"source_selection":  appcore.PipelineSourceSelectionCustom,
		},
	}, runtimeCtx); err != nil {
		t.Fatalf("executeSelectSourcesNode returned error: %v", err)
	}
	if got := len(runtimeCtx.SelectedSources); got != 1 {
		t.Fatalf("selected sources len = %d, want 1 custom source", got)
	}
	if runtimeCtx.SelectedSources[0].ID != "source-b" {
		t.Fatalf("selected source = %#v, want source-b", runtimeCtx.SelectedSources[0])
	}
}

func TestPipelineSourceGroupMissingSourceProfileFails(t *testing.T) {
	isolateStorageForTest(t)
	runtimeCtx := &pipelineRuntimeContext{ConfigSnapshot: defaultDesktopConfigSnapshot()}
	result, err := (&App{}).executeSelectSourcesNode(appcore.PipelineNode{
		Action: appcore.PipelineNodeActionSelectSources,
		Config: map[string]any{
			"source_profile_id": "missing-profile",
			"source_selection":  appcore.PipelineSourceSelectionEnabled,
		},
	}, runtimeCtx)
	if err == nil {
		t.Fatal("executeSelectSourcesNode returned nil error, want missing profile error")
	}
	if result.Status != "failed" {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if !strings.Contains(err.Error(), "输入组档案 missing-profile 不存在") {
		t.Fatalf("error = %q, want missing profile message", err.Error())
	}
}

func TestPipelineFilterSourcesNodeOverridesSelectedSources(t *testing.T) {
	runtimeCtx := &pipelineRuntimeContext{
		SelectedSources: []DesktopSource{
			sourceProfileTestSource("source-a", "Source A"),
			sourceProfileTestSource("source-b", "Source B"),
		},
	}
	result, err := (&App{}).executeFilterSourcesNode(appcore.PipelineNode{
		Action: appcore.PipelineNodeActionFilterSources,
		Config: map[string]any{
			"source_colo_filter":      "HKG",
			"source_colo_filter_mode": "deny",
			"source_ip_limit":         25,
			"source_ip_mode":          "mcis",
		},
	}, runtimeCtx)
	if err != nil {
		t.Fatalf("executeFilterSourcesNode returned error: %v", err)
	}
	if result.Status != "completed" {
		t.Fatalf("status = %q, want completed", result.Status)
	}
	if got := len(runtimeCtx.SelectedSources); got != 2 {
		t.Fatalf("selected sources len = %d, want 2", got)
	}
	for _, source := range runtimeCtx.SelectedSources {
		if source.IPLimit != 25 || source.IPMode != "mcis" || source.ColoFilter != "HKG" || source.ColoFilterMode != "deny" {
			t.Fatalf("source override = %#v, want filter overrides", source)
		}
	}
}

func TestPipelineCheckOutputExportsMissingCSV(t *testing.T) {
	isolateStorageForTest(t)
	exportDir := t.TempDir()
	snapshot := defaultDesktopConfigSnapshot()
	exportCfg := mapValue(snapshot["export"])
	exportCfg["target_dir"] = exportDir
	snapshot["export"] = exportCfg
	runtimeCtx := &pipelineRuntimeContext{
		ConfigSnapshot: snapshot,
		ProbeResult: &ProbeRunResult{
			Results: []ProbeRow{{IP: "203.0.113.10", DownloadSpeedMB: 12.5}},
		},
		TaskID: "pipeline-check-output-test",
	}
	result, err := (&App{}).executeCheckOutputNode(appcore.PipelineNode{
		Action: appcore.PipelineNodeActionCheckOutput,
		Config: map[string]any{
			"export_if_missing": true,
			"require_csv":       true,
			"source":            "probe_results",
		},
	}, runtimeCtx)
	if err != nil {
		t.Fatalf("executeCheckOutputNode returned error: %v", err)
	}
	if result.Status != "completed" {
		t.Fatalf("status = %q, want completed", result.Status)
	}
	if _, err := os.Stat(filepath.Join(exportDir, "result.csv")); err != nil {
		t.Fatalf("result.csv was not exported: %v", err)
	}
}

func TestPipelineCheckOutputAppliesTopNSelection(t *testing.T) {
	runtimeCtx := &pipelineRuntimeContext{
		ConfigSnapshot: defaultDesktopConfigSnapshot(),
		ProbeResult: &ProbeRunResult{
			Config: ProbeConfig{DownloadSpeedMetric: "average"},
			Results: []ProbeRow{
				{IP: "203.0.113.10", DownloadSpeedMB: 5},
				{IP: "203.0.113.20", DownloadSpeedMB: 20},
				{IP: "203.0.113.30", DownloadSpeedMB: 10},
			},
		},
		TaskID: "pipeline-check-output-top-n-test",
	}
	result, err := (&App{}).executeCheckOutputNode(appcore.PipelineNode{
		ID:     "check-output",
		Action: appcore.PipelineNodeActionCheckOutput,
		Config: map[string]any{
			"require_csv": false,
			"source":      "probe_results",
			"status":      "all",
			"top_n":       1,
		},
	}, runtimeCtx)
	if err != nil {
		t.Fatalf("executeCheckOutputNode returned error: %v", err)
	}
	if result.Status != "completed" {
		t.Fatalf("status = %q, want completed", result.Status)
	}
	if got := len(runtimeCtx.FilteredRows); got != 1 || runtimeCtx.FilteredRows[0].IP != "203.0.113.20" {
		t.Fatalf("filtered rows = %#v, want top speed row", runtimeCtx.FilteredRows)
	}
}

func TestPipelineCheckOutputManualReviewWhenNoResults(t *testing.T) {
	result, err := (&App{}).executeCheckOutputNode(appcore.PipelineNode{
		Action: appcore.PipelineNodeActionCheckOutput,
		Config: map[string]any{"source": "probe_results"},
	}, &pipelineRuntimeContext{})
	if err != nil {
		t.Fatalf("executeCheckOutputNode returned error: %v", err)
	}
	if result.Status != "manual_review" {
		t.Fatalf("status = %q, want manual_review", result.Status)
	}
}

func TestPipelineCheckOutputStatusSurvivesTrailingEndNode(t *testing.T) {
	template := appcore.PipelineTemplate{
		EntryNodeID: "check-output",
		ID:          "template-check-output",
		Nodes: []appcore.PipelineNode{
			{
				Action:   appcore.PipelineNodeActionCheckOutput,
				Config:   map[string]any{"source": "probe_results"},
				ID:       "check-output",
				Name:     "结果检查与输出",
				NodeType: appcore.PipelineNodeTypeDeliver,
			},
			{
				Action: appcore.PipelineNodeActionEnd,
				Config: map[string]any{
					"message": "流程已结束。",
					"status":  "completed",
				},
				ID:       "end-main",
				Name:     "结束",
				NodeType: appcore.PipelineNodeTypeEnd,
			},
		},
		Edges: []appcore.PipelineEdge{
			{ID: "edge-output-end", SourceNode: "check-output", TargetNode: "end-main"},
		},
	}
	result, err := (&App{}).executeTemplateDAG(PipelineProfile{ID: "profile-1", Enabled: true}, template, &pipelineRuntimeContext{TaskID: "pipeline-check-output-status-test"}, "pipeline-check-output-status-test", nil)
	if err != nil {
		t.Fatalf("executeTemplateDAG returned error: %v", err)
	}
	if result.Status != "manual_review" {
		t.Fatalf("status = %q, want manual_review", result.Status)
	}
	if got := len(result.NodeResults); got != 2 {
		t.Fatalf("node results len = %d, want 2", got)
	}
}

func TestDeletePipelineTemplateRejectsDefaultTemplate(t *testing.T) {
	isolateStorageForTest(t)
	result := (&App{}).DeletePipelineTemplate(map[string]any{"template_id": appcore.DefaultPipelineTemplateID})
	if result.OK {
		t.Fatalf("DeletePipelineTemplate(default) OK = true, want false")
	}
}

func TestDeletePipelineTemplateRejectsAdvancedBuiltInTemplate(t *testing.T) {
	isolateStorageForTest(t)
	result := (&App{}).DeletePipelineTemplate(map[string]any{"template_id": appcore.AdvancedUploadPipelineTemplateID})
	if result.OK {
		t.Fatalf("DeletePipelineTemplate(advanced built-in) OK = true, want false")
	}
}

func TestDeletePipelineTemplateAllowsCustomTemplate(t *testing.T) {
	isolateStorageForTest(t)
	now := "2026-05-31T00:00:00Z"
	custom := appcore.DefaultPipelineTemplate(now)
	custom.ID = "custom-template"
	custom.Name = "Custom"
	workspace := appcore.NormalizePipelineWorkspaceForSave(appcore.PipelineWorkspace{
		ActiveTemplateID: "custom-template",
		SchemaVersion:    pipelineWorkspaceSchemaVersion,
		Templates:        []appcore.PipelineTemplate{custom},
		UpdatedAt:        now,
	}, pipelineWorkspaceSchemaVersion, now, sanitizeDesktopConfigSnapshot, nil, nil)
	if err := savePipelineWorkspace(workspace); err != nil {
		t.Fatalf("savePipelineWorkspace: %v", err)
	}
	result := (&App{}).DeletePipelineTemplate(map[string]any{"template_id": "custom-template"})
	if !result.OK {
		t.Fatalf("DeletePipelineTemplate(custom) failed: %#v", result)
	}
	next := pipelineWorkspaceFromAny(result.Data)
	for _, template := range next.Templates {
		if template.ID == "custom-template" {
			t.Fatalf("custom template still present after delete")
		}
	}
}

func TestPipelineEnsureUploadSelectionAppliesNodeOverridesAndTopN(t *testing.T) {
	runtimeCtx := &pipelineRuntimeContext{
		ConfigSnapshot: defaultDesktopConfigSnapshot(),
		NodeOutputs:    map[string]any{},
		ProbeResult: &ProbeRunResult{
			Config: ProbeConfig{DownloadSpeedMetric: "average"},
			Results: []ProbeRow{
				{IP: "203.0.113.10", Colo: "HKG", DownloadSpeedMB: 5},
				{IP: "203.0.113.20", Colo: "HKG", DownloadSpeedMB: 15},
				{IP: "2001:db8::1", Colo: "HKG", DownloadSpeedMB: 30},
				{IP: "198.51.100.30", Colo: "LAX", DownloadSpeedMB: 25},
			},
		},
	}
	node := appcore.PipelineNode{
		ID:     "filter-node",
		Action: appcore.PipelineNodeActionFilterResults,
		Config: map[string]any{
			"colo_allow": "HKG",
			"ip_version": "ipv4",
			"source":     "probe_results",
			"top_n":      1,
		},
	}

	selection, err := pipelineEnsureUploadSelection(runtimeCtx, node)
	if err != nil {
		t.Fatalf("pipelineEnsureUploadSelection returned error: %v", err)
	}
	if got := len(selection.FilteredRows); got != 1 {
		t.Fatalf("filtered len = %d, want 1", got)
	}
	if selection.FilteredRows[0].IP != "203.0.113.20" {
		t.Fatalf("filtered row = %#v, want top HKG IPv4 result", selection.FilteredRows[0])
	}
	if got := len(runtimeCtx.FilteredRows); got != 1 || runtimeCtx.FilteredRows[0].IP != "203.0.113.20" {
		t.Fatalf("runtime filtered rows = %#v, want cached top result", runtimeCtx.FilteredRows)
	}
}

func TestPipelineDNSSnapshotForNodeAppliesNodeOverrides(t *testing.T) {
	snapshot := defaultDesktopConfigSnapshot()
	cloudflare := mapValue(snapshot["cloudflare"])
	cloudflare["record_name"] = "old.example.com"
	snapshot["cloudflare"] = cloudflare

	runtimeCtx := &pipelineRuntimeContext{
		ConfigSnapshot: snapshot,
	}
	node := appcore.PipelineNode{
		Action: appcore.PipelineNodeActionDeliverDNS,
		Config: map[string]any{
			"comment":     "pipeline override",
			"proxied":     true,
			"record_name": "new.example.com",
			"record_type": "AAAA",
			"top_n":       2,
			"ttl":         120,
		},
	}

	effectiveSnapshot := pipelineDNSSnapshotForNode(runtimeCtx, node)
	nextCloudflare := mapValue(effectiveSnapshot["cloudflare"])
	nextUpload := mapValue(effectiveSnapshot["upload"])
	nextUploadCloudflare := mapValue(nextUpload["cloudflare"])

	if got := stringValue(nextCloudflare["record_name"], ""); got != "new.example.com" {
		t.Fatalf("record_name = %q, want new.example.com", got)
	}
	if got := stringValue(nextCloudflare["record_type"], ""); got != "AAAA" {
		t.Fatalf("record_type = %q, want AAAA", got)
	}
	if got := intValue(nextCloudflare["ttl"], 0); got != 120 {
		t.Fatalf("ttl = %d, want 120", got)
	}
	if got := boolValue(nextCloudflare["proxied"], true); got {
		t.Fatalf("proxied = %v, want false", got)
	}
	if got := stringValue(nextCloudflare["comment"], ""); got != "pipeline override" {
		t.Fatalf("comment = %q, want pipeline override", got)
	}
	if got := intValue(nextUploadCloudflare["top_n"], 0); got != 2 {
		t.Fatalf("cloudflare top_n = %d, want 2", got)
	}
}

func TestPipelineGitHubSelectionSnapshotAppliesNodeTopN(t *testing.T) {
	runtimeCtx := &pipelineRuntimeContext{
		ConfigSnapshot: defaultDesktopConfigSnapshot(),
	}
	node := appcore.PipelineNode{
		Action: appcore.PipelineNodeActionDeliverGitHub,
		Config: map[string]any{
			"source": "filtered_rows",
			"top_n":  3,
		},
	}

	effectiveSnapshot := pipelineSelectionSnapshotForNode(runtimeCtx, node)
	upload := mapValue(effectiveSnapshot["upload"])
	github := mapValue(upload["github"])
	cloudflare := mapValue(upload["cloudflare"])

	if got := intValue(github["top_n"], 0); got != 3 {
		t.Fatalf("github top_n = %d, want 3", got)
	}
	if got := intValue(cloudflare["top_n"], 0); got == 3 {
		t.Fatalf("cloudflare top_n = %d, want GitHub override to be target-specific", got)
	}
}

func TestPipelineRowsForNodeSourceUsesLastUploadSelectionDeterministically(t *testing.T) {
	runtimeCtx := &pipelineRuntimeContext{
		ConfigSnapshot: defaultDesktopConfigSnapshot(),
		NodeOutputs: map[string]any{
			"older": UploadSelectionResult{FilteredRows: []ProbeRow{{IP: "198.51.100.10"}}},
		},
	}

	first := UploadSelectionResult{FilteredRows: []ProbeRow{{IP: "203.0.113.10"}}}
	second := UploadSelectionResult{FilteredRows: []ProbeRow{{IP: "203.0.113.20"}}}
	runtimeCtx.NodeOutputs["first"] = first
	runtimeCtx.LastUploadSelection = &second

	rows := pipelineRowsForNodeSource(runtimeCtx, "filtered_rows")
	if got := len(rows); got != 1 {
		t.Fatalf("rows len = %d, want 1", got)
	}
	if rows[0].IP != "203.0.113.20" {
		t.Fatalf("filtered_rows = %#v, want last explicit upload selection", rows)
	}
}
