package app

import (
	"os"
	"path/filepath"
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
		Action: appcore.PipelineNodeActionRunProbe,
		Config: map[string]any{
			"download_count":          5,
			"download_enabled":        false,
			"max_loss_rate":           0.05,
			"max_tcp_latency_ms":      120,
			"min_download_mbps":       3.5,
			"port_policy":             "fixed_global",
			"source_colo_filter":      "HKG",
			"source_colo_filter_mode": "deny",
			"source_ip_limit":         12,
			"source_ip_mode":          "mcis",
			"source_mode":             "custom",
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
			"tcp_port": 2053,
		},
	}

	effectiveSnapshot := pipelineProbeSnapshotForNode(runtimeCtx, node)
	probe := mapValue(effectiveSnapshot["probe"])
	thresholds := mapValue(probe["thresholds"])
	stageLimits := mapValue(probe["stage_limits"])
	if got := intValue(probe["tcp_port"], 0); got != 2053 {
		t.Fatalf("tcp_port = %d, want 2053", got)
	}
	if got := stringValue(probe["port_policy"], ""); got != "fixed_global" {
		t.Fatalf("port_policy = %q, want fixed_global", got)
	}
	if got := stringValue(probe["strategy"], ""); got != "fast" {
		t.Fatalf("strategy = %q, want fast when download disabled", got)
	}
	if got := intValue(probe["download_count"], 0); got != 5 {
		t.Fatalf("download_count = %d, want 5", got)
	}
	if got := intValue(stageLimits["stage3"], 0); got != 5 {
		t.Fatalf("stage3 limit = %d, want 5", got)
	}
	if got := intValue(thresholds["max_tcp_latency_ms"], 0); got != 120 {
		t.Fatalf("max_tcp_latency_ms = %d, want 120", got)
	}
	if got := floatValue(thresholds["min_download_mbps"], 0); got != 3.5 {
		t.Fatalf("min_download_mbps = %f, want 3.5", got)
	}
	if got := floatValue(probe["max_loss_rate"], 0); got != 0.05 {
		t.Fatalf("max_loss_rate = %f, want 0.05", got)
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

func TestPipelineProbeNodeEnablesFullStrategyWhenDownloadEnabled(t *testing.T) {
	snapshot := defaultDesktopConfigSnapshot()
	runtimeCtx := &pipelineRuntimeContext{
		ConfigSnapshot: snapshot,
	}
	effectiveSnapshot := pipelineProbeSnapshotForNode(runtimeCtx, appcore.PipelineNode{
		Action: appcore.PipelineNodeActionRunProbe,
		Config: map[string]any{
			"download_enabled": true,
		},
	})
	probe := mapValue(effectiveSnapshot["probe"])
	if got := stringValue(probe["strategy"], ""); got != "full" {
		t.Fatalf("strategy = %q, want full when download is enabled", got)
	}
	if got := boolValue(probe["disable_download"], true); got {
		t.Fatalf("disable_download = true, want false when download is enabled")
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
			"source_ids": []any{"source-b"},
		},
	}
	if _, err := (&App{}).executeSelectSourcesNode(sourceNode, runtimeCtx); err != nil {
		t.Fatalf("executeSelectSourcesNode returned error: %v", err)
	}
	probeSources := pipelineProbeSourcesForNode(runtimeCtx, appcore.PipelineNode{
		Action: appcore.PipelineNodeActionRunProbe,
		Config: map[string]any{"source_mode": "inherit"},
	})
	if got := len(probeSources); got != 1 {
		t.Fatalf("probe sources len = %d, want 1", got)
	}
	if probeSources[0].ID != "source-b" {
		t.Fatalf("probe source id = %q, want source-b", probeSources[0].ID)
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

func TestDeletePipelineTemplateRejectsDefaultTemplate(t *testing.T) {
	isolateStorageForTest(t)
	result := (&App{}).DeletePipelineTemplate(map[string]any{"template_id": appcore.DefaultPipelineTemplateID})
	if result.OK {
		t.Fatalf("DeletePipelineTemplate(default) OK = true, want false")
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
	if got := boolValue(nextCloudflare["proxied"], false); !got {
		t.Fatalf("proxied = %v, want true", got)
	}
	if got := stringValue(nextCloudflare["comment"], ""); got != "pipeline override" {
		t.Fatalf("comment = %q, want pipeline override", got)
	}
	if got := intValue(nextUploadCloudflare["top_n"], 0); got != 2 {
		t.Fatalf("cloudflare top_n = %d, want 2", got)
	}
}
