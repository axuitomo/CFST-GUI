package appcore

import (
	"strings"
	"testing"
)

func TestDefaultPipelineTemplateUsesContinuationAfterCheckOutput(t *testing.T) {
	template := DefaultPipelineTemplate("2026-05-31T00:00:00Z")
	if template.EntryNodeID != "advanced-source-group" {
		t.Fatalf("entry_node_id = %q, want advanced-source-group", template.EntryNodeID)
	}
	if got := len(template.Nodes); got != 12 {
		t.Fatalf("nodes len = %d, want 12", got)
	}
	wantNodes := map[string]struct {
		action   string
		nodeType string
	}{
		"advanced-source-group":      {PipelineNodeActionSelectSources, PipelineNodeTypeSource},
		"advanced-source-filter":     {PipelineNodeActionFilterSources, PipelineNodeTypeSource},
		"advanced-probe-tcp":         {PipelineNodeActionProbeTCP, PipelineNodeTypeProbe},
		"advanced-probe-trace":       {PipelineNodeActionProbeTrace, PipelineNodeTypeProbe},
		"advanced-probe-download":    {PipelineNodeActionProbeDownload, PipelineNodeTypeProbe},
		"advanced-filter":            {PipelineNodeActionFilterResults, PipelineNodeTypeFilter},
		"advanced-branch-results":    {PipelineNodeActionBranchHasResults, PipelineNodeTypeBranch},
		"advanced-deliver-dns":       {PipelineNodeActionDeliverDNS, PipelineNodeTypeDeliver},
		"advanced-deliver-github":    {PipelineNodeActionDeliverGitHub, PipelineNodeTypeDeliver},
		"advanced-end-completed":     {PipelineNodeActionEnd, PipelineNodeTypeEnd},
		"advanced-recovery-empty":    {PipelineNodeActionRecoveryMark, PipelineNodeTypeRecovery},
		"advanced-end-manual-review": {PipelineNodeActionEnd, PipelineNodeTypeEnd},
	}
	for _, node := range template.Nodes {
		want, ok := wantNodes[node.ID]
		if !ok {
			t.Fatalf("unexpected node %#v", node)
		}
		if node.Action != want.action || node.NodeType != want.nodeType {
			t.Fatalf("node %s = (%s,%s), want (%s,%s)", node.ID, node.Action, node.NodeType, want.action, want.nodeType)
		}
	}
	branchOutcomes := map[string]string{}
	for _, edge := range template.Edges {
		if edge.SourceNode == "advanced-branch-results" {
			branchOutcomes[edge.Outcome] = edge.TargetNode
		}
	}
	if branchOutcomes["true"] != "advanced-deliver-dns" {
		t.Fatalf("true outcome target = %q, want advanced-deliver-dns", branchOutcomes["true"])
	}
	if branchOutcomes["false"] != "advanced-recovery-empty" {
		t.Fatalf("false outcome target = %q, want advanced-recovery-empty", branchOutcomes["false"])
	}
	if err := ValidatePipelineTemplate(template); err != nil {
		t.Fatalf("default template should validate: %v", err)
	}
}

func TestAdvancedUploadPipelineTemplateProvidesEmptyResultFallback(t *testing.T) {
	template := AdvancedUploadPipelineTemplate("2026-06-02T00:00:00Z")
	if template.ID != AdvancedUploadPipelineTemplateID {
		t.Fatalf("template id = %q, want %s", template.ID, AdvancedUploadPipelineTemplateID)
	}
	if template.EntryNodeID != "advanced-source-group" {
		t.Fatalf("entry_node_id = %q, want advanced-source-group", template.EntryNodeID)
	}
	if got := len(template.Nodes); got != 12 {
		t.Fatalf("nodes len = %d, want 12", got)
	}
	wantNodes := map[string]struct {
		action   string
		nodeType string
	}{
		"advanced-source-group":      {PipelineNodeActionSelectSources, PipelineNodeTypeSource},
		"advanced-source-filter":     {PipelineNodeActionFilterSources, PipelineNodeTypeSource},
		"advanced-probe-tcp":         {PipelineNodeActionProbeTCP, PipelineNodeTypeProbe},
		"advanced-probe-trace":       {PipelineNodeActionProbeTrace, PipelineNodeTypeProbe},
		"advanced-probe-download":    {PipelineNodeActionProbeDownload, PipelineNodeTypeProbe},
		"advanced-filter":            {PipelineNodeActionFilterResults, PipelineNodeTypeFilter},
		"advanced-branch-results":    {PipelineNodeActionBranchHasResults, PipelineNodeTypeBranch},
		"advanced-deliver-dns":       {PipelineNodeActionDeliverDNS, PipelineNodeTypeDeliver},
		"advanced-deliver-github":    {PipelineNodeActionDeliverGitHub, PipelineNodeTypeDeliver},
		"advanced-end-completed":     {PipelineNodeActionEnd, PipelineNodeTypeEnd},
		"advanced-recovery-empty":    {PipelineNodeActionRecoveryMark, PipelineNodeTypeRecovery},
		"advanced-end-manual-review": {PipelineNodeActionEnd, PipelineNodeTypeEnd},
	}
	for _, node := range template.Nodes {
		want, ok := wantNodes[node.ID]
		if !ok {
			t.Fatalf("unexpected node %#v", node)
		}
		if node.Action != want.action || node.NodeType != want.nodeType {
			t.Fatalf("node %s = (%s,%s), want (%s,%s)", node.ID, node.Action, node.NodeType, want.action, want.nodeType)
		}
	}
	branchOutcomes := map[string]string{}
	for _, edge := range template.Edges {
		if edge.SourceNode == "advanced-branch-results" {
			branchOutcomes[edge.Outcome] = edge.TargetNode
		}
	}
	if branchOutcomes["true"] != "advanced-deliver-dns" {
		t.Fatalf("true outcome target = %q, want advanced-deliver-dns", branchOutcomes["true"])
	}
	if branchOutcomes["false"] != "advanced-recovery-empty" {
		t.Fatalf("false outcome target = %q, want advanced-recovery-empty", branchOutcomes["false"])
	}
	if err := ValidatePipelineTemplate(template); err != nil {
		t.Fatalf("advanced upload template should validate: %v", err)
	}
}

func TestDefaultPipelineNodeCatalogExposesDeliverGitHubControls(t *testing.T) {
	var githubItem *PipelineNodeCatalogItem
	items := DefaultPipelineNodeCatalog()
	for index := range items {
		item := &items[index]
		if item.Action == PipelineNodeActionDeliverGitHub {
			githubItem = item
			break
		}
	}
	if githubItem == nil {
		t.Fatal("deliver_github catalog item not found")
	}
	if githubItem.NodeType != PipelineNodeTypeDeliver {
		t.Fatalf("deliver_github node_type = %q, want deliver", githubItem.NodeType)
	}
	if githubItem.DefaultConfig["source"] != "filtered_rows" {
		t.Fatalf("deliver_github default source = %#v, want filtered_rows", githubItem.DefaultConfig["source"])
	}
	fieldKeys := map[string]bool{}
	for _, field := range githubItem.FormSchema {
		fieldKeys[field.Key] = true
	}
	for _, key := range []string{"source", "top_n"} {
		if !fieldKeys[key] {
			t.Fatalf("deliver_github form is missing %s field: %#v", key, githubItem.FormSchema)
		}
	}
}

func TestDefaultPipelineNodeCatalogUsesStageOnlyProbeForms(t *testing.T) {
	items := DefaultPipelineNodeCatalog()
	byAction := map[string]PipelineNodeCatalogItem{}
	for _, item := range items {
		byAction[item.Action] = item
	}
	stageKeys := map[string][]string{
		PipelineNodeActionProbeTCP:      {"tcp_port", "port_policy", "concurrency_stage1", "ping_times", "max_tcp_latency_ms", "min_delay_ms", "max_loss_rate", "timeout_stage1_ms"},
		PipelineNodeActionProbeTrace:    {"trace_url", "trace_colo_mode", "source_colo_filter_phase", "concurrency_stage2", "timeout_stage2_ms", "httping_status_code", "max_trace_latency_ms", "httping_cf_colo", "httping_cf_colo_mode"},
		PipelineNodeActionProbeDownload: {"url", "stage3_limit", "download_count", "print_num", "concurrency_stage3", "download_get_concurrency", "download_time_seconds", "timeout_stage3_ms", "download_warmup_seconds", "download_speed_sample_interval_ms", "download_buffer_kb", "download_http_protocol", "download_speed_metric", "min_download_mbps"},
	}
	for action, requiredKeys := range stageKeys {
		item, ok := byAction[action]
		if !ok {
			t.Fatalf("%s catalog item not found", action)
		}
		if item.NodeType != PipelineNodeTypeProbe {
			t.Fatalf("%s node_type = %q, want probe", action, item.NodeType)
		}
		fieldKeys := map[string]bool{}
		for _, field := range item.FormSchema {
			fieldKeys[field.Key] = true
		}
		for _, key := range requiredKeys {
			if !fieldKeys[key] {
				t.Fatalf("%s form is missing %s field", action, key)
			}
		}
		if len(fieldKeys) != len(requiredKeys) {
			t.Fatalf("%s form keys = %#v, want only %#v", action, fieldKeys, requiredKeys)
		}
		if item.DefaultConfig["strategy"] != "full" {
			t.Fatalf("%s strategy default = %#v, want full", action, item.DefaultConfig["strategy"])
		}
		if item.DefaultConfig["disable_download"] != false {
			t.Fatalf("%s disable_download default = %#v, want false", action, item.DefaultConfig["disable_download"])
		}
		for _, hiddenDefaultKey := range []string{"tcp_port", "trace_url", "url", "download_count"} {
			if _, ok := item.DefaultConfig[hiddenDefaultKey]; !ok {
				t.Fatalf("%s default config missing hidden runtime key %s", action, hiddenDefaultKey)
			}
		}
	}
}

func TestNormalizePipelineWorkspaceForSaveMigratesCheckOutputEndNode(t *testing.T) {
	now := "2026-05-31T00:00:00Z"
	template := PipelineTemplate{
		CreatedAt:   now,
		EntryNodeID: "check-output",
		ID:          "custom-template",
		Name:        "Custom",
		Nodes: []PipelineNode{
			{
				Action:   PipelineNodeActionCheckOutput,
				ID:       "check-output",
				Name:     "结果检查与输出",
				NodeType: PipelineNodeTypeEnd,
			},
		},
		UpdatedAt: now,
		Version:   1,
	}
	workspace := NormalizePipelineWorkspaceForSave(PipelineWorkspace{
		ActiveTemplateID: "custom-template",
		SchemaVersion:    DefaultPipelineWorkspaceSchemaVersion,
		Templates:        []PipelineTemplate{template},
		UpdatedAt:        now,
	}, DefaultPipelineWorkspaceSchemaVersion, now, nil, nil, nil)
	var migrated PipelineTemplate
	for _, item := range workspace.Templates {
		if item.ID == "custom-template" {
			migrated = item
			break
		}
	}
	if len(migrated.Nodes) != 2 {
		t.Fatalf("nodes len = %d, want migrated check_output + end", len(migrated.Nodes))
	}
	if migrated.Nodes[0].NodeType != PipelineNodeTypeDeliver {
		t.Fatalf("check_output node_type = %q, want deliver", migrated.Nodes[0].NodeType)
	}
	if len(migrated.Edges) != 1 || migrated.Edges[0].SourceNode != "check-output" {
		t.Fatalf("edges = %#v, want check-output -> generated end", migrated.Edges)
	}
	if err := ValidatePipelineTemplate(migrated); err != nil {
		t.Fatalf("migrated template should validate: %v", err)
	}
}

func TestNormalizePipelineWorkspaceForSaveMigratesLegacyRunProbeNode(t *testing.T) {
	now := "2026-06-03T00:00:00Z"
	template := PipelineTemplate{
		CreatedAt:   now,
		EntryNodeID: "source-group-main",
		ID:          DefaultPipelineTemplateID,
		Name:        "默认流程",
		Nodes: []PipelineNode{
			{
				Action:   PipelineNodeActionSelectSources,
				ID:       "source-group-main",
				Name:     "输入源组",
				NodeType: PipelineNodeTypeSource,
			},
			{
				Action:   legacyPipelineNodeActionRunProbe,
				ID:       "probe-main",
				Name:     "测速",
				NodeType: PipelineNodeTypeProbe,
				UI:       &PipelineNodeUI{Position: &PipelineCanvasPosition{X: 420, Y: 120}, Width: 320},
			},
			{
				Action:   PipelineNodeActionCheckOutput,
				ID:       "check-output",
				Name:     "结果检查与输出",
				NodeType: PipelineNodeTypeDeliver,
			},
			{
				Action:   PipelineNodeActionEnd,
				ID:       "end-main",
				Name:     "结束",
				NodeType: PipelineNodeTypeEnd,
			},
		},
		Edges: []PipelineEdge{
			{ID: "edge-source-probe", SourceNode: "source-group-main", TargetNode: "probe-main"},
			{ID: "edge-probe-output", SourceNode: "probe-main", TargetNode: "check-output"},
			{ID: "edge-output-end", SourceNode: "check-output", TargetNode: "end-main"},
		},
		UpdatedAt: now,
		Version:   1,
	}
	workspace := NormalizePipelineWorkspaceForSave(PipelineWorkspace{
		ActiveTemplateID: DefaultPipelineTemplateID,
		SchemaVersion:    DefaultPipelineWorkspaceSchemaVersion,
		Templates:        []PipelineTemplate{template},
		UpdatedAt:        now,
	}, DefaultPipelineWorkspaceSchemaVersion, now, nil, nil, nil)
	var migrated PipelineTemplate
	for _, item := range workspace.Templates {
		if item.ID == DefaultPipelineTemplateID {
			migrated = item
			break
		}
	}
	nodes := map[string]PipelineNode{}
	for _, node := range migrated.Nodes {
		nodes[node.ID] = node
	}
	for nodeID, action := range map[string]string{
		"probe-main":          PipelineNodeActionProbeTCP,
		"probe-main-trace":    PipelineNodeActionProbeTrace,
		"probe-main-download": PipelineNodeActionProbeDownload,
	} {
		node, ok := nodes[nodeID]
		if !ok {
			t.Fatalf("migrated node %s not found: %#v", nodeID, migrated.Nodes)
		}
		if node.Action != action || node.NodeType != PipelineNodeTypeProbe {
			t.Fatalf("node %s = (%s,%s), want (%s,probe)", nodeID, node.Action, node.NodeType, action)
		}
		if intValue(node.Config["tcp_port"], 0) != 443 {
			t.Fatalf("node %s config = %#v, want probe defaults", nodeID, node.Config)
		}
	}
	wantEdges := map[string]string{
		"source-group-main":   "probe-main",
		"probe-main":          "probe-main-trace",
		"probe-main-trace":    "probe-main-download",
		"probe-main-download": "check-output",
		"check-output":        "end-main",
	}
	gotEdges := map[string]string{}
	for _, edge := range migrated.Edges {
		gotEdges[edge.SourceNode] = edge.TargetNode
	}
	for source, target := range wantEdges {
		if gotEdges[source] != target {
			t.Fatalf("edge from %s = %q, want %q; edges = %#v", source, gotEdges[source], target, migrated.Edges)
		}
	}
	if err := ValidatePipelineTemplate(migrated); err != nil {
		t.Fatalf("migrated legacy run_probe template should validate: %v", err)
	}
}

func TestNormalizePipelineWorkspaceForSaveKeepsDefaultTemplate(t *testing.T) {
	now := "2026-05-31T00:00:00Z"
	custom := DefaultPipelineTemplate(now)
	custom.ID = "custom-template"
	custom.Name = "Custom"
	workspace := NormalizePipelineWorkspaceForSave(PipelineWorkspace{
		ActiveTemplateID: "custom-template",
		SchemaVersion:    DefaultPipelineWorkspaceSchemaVersion,
		Templates:        []PipelineTemplate{custom},
		UpdatedAt:        now,
	}, DefaultPipelineWorkspaceSchemaVersion, now, nil, nil, nil)
	if len(workspace.Templates) != 3 {
		t.Fatalf("templates len = %d, want default + custom + advanced", len(workspace.Templates))
	}
	if workspace.Templates[0].ID != DefaultPipelineTemplateID {
		t.Fatalf("first template id = %q, want default", workspace.Templates[0].ID)
	}
	if workspace.Templates[2].ID != AdvancedUploadPipelineTemplateID {
		t.Fatalf("third template id = %q, want advanced upload", workspace.Templates[2].ID)
	}
	if workspace.ActiveTemplateID != "custom-template" {
		t.Fatalf("active_template_id = %q, want custom-template", workspace.ActiveTemplateID)
	}
}

func TestNormalizePipelineWorkspaceForSaveCompletesDefaultStageNodeConfig(t *testing.T) {
	now := "2026-05-31T00:00:00Z"
	template := DefaultPipelineTemplate(now)
	for index := range template.Nodes {
		switch template.Nodes[index].ID {
		case "advanced-source-filter", "advanced-probe-tcp", "advanced-probe-trace", "advanced-probe-download":
			template.Nodes[index].Config = map[string]any{}
		}
	}
	workspace := NormalizePipelineWorkspaceForSave(PipelineWorkspace{
		ActiveTemplateID: DefaultPipelineTemplateID,
		SchemaVersion:    DefaultPipelineWorkspaceSchemaVersion,
		Templates:        []PipelineTemplate{template},
		UpdatedAt:        now,
	}, DefaultPipelineWorkspaceSchemaVersion, now, nil, nil, nil)
	nodes := map[string]PipelineNode{}
	for _, node := range workspace.Templates[0].Nodes {
		nodes[node.ID] = node
	}
	if nodes["advanced-source-filter"].Config["source_ip_limit"] != 500 {
		t.Fatalf("advanced-source-filter config = %#v, want source filter defaults", nodes["advanced-source-filter"].Config)
	}
	if nodes["advanced-probe-tcp"].Config["tcp_port"] != 443 {
		t.Fatalf("advanced-probe-tcp config = %#v, want tcp defaults", nodes["advanced-probe-tcp"].Config)
	}
	if nodes["advanced-probe-download"].Config["download_count"] != 10 {
		t.Fatalf("advanced-probe-download config = %#v, want download defaults", nodes["advanced-probe-download"].Config)
	}
}

func TestNormalizePipelineWorkspaceForSaveMigratesLegacyTargetSelectionToBoundConfig(t *testing.T) {
	now := "2026-05-24T10:00:00Z"
	makeTarget := func(id string, enabled bool, domain string) PipelineTarget {
		return PipelineTarget{
			ConfigSnapshot: map[string]any{
				"cloudflare": map[string]any{
					"record_name": domain,
				},
			},
			CreatedAt:     now,
			DNSPushPolicy: PipelineDNSPushPolicyAuto,
			Domain:        domain,
			Enabled:       enabled,
			ID:            id,
			Name:          id,
			Region:        "兼容",
			TemplateID:    "template-a",
			UpdatedAt:     now,
		}
	}

	for _, tc := range []struct {
		activeTargetID string
		name           string
		targets        []PipelineTarget
		wantDomain     string
		wantTargetID   string
	}{
		{
			activeTargetID: "target-b",
			name:           "active target wins",
			targets: []PipelineTarget{
				makeTarget("target-a", true, "a.example.com"),
				makeTarget("target-b", true, "b.example.com"),
			},
			wantDomain:   "b.example.com",
			wantTargetID: "target-b",
		},
		{
			name: "first enabled wins when active target missing",
			targets: []PipelineTarget{
				makeTarget("target-a", false, "a.example.com"),
				makeTarget("target-b", true, "b.example.com"),
			},
			wantDomain:   "b.example.com",
			wantTargetID: "target-b",
		},
		{
			name: "first target wins when none enabled",
			targets: []PipelineTarget{
				makeTarget("target-a", false, "a.example.com"),
				makeTarget("target-b", false, "b.example.com"),
			},
			wantDomain:   "a.example.com",
			wantTargetID: "target-a",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			template := DefaultPipelineTemplate(now)
			template.ID = "template-a"
			template.Name = "Template A"
			template.BoundConfigSnapshot = nil
			workspace := NormalizePipelineWorkspaceForSave(PipelineWorkspace{
				ActiveTargetID:   tc.activeTargetID,
				ActiveTemplateID: "template-a",
				SchemaVersion:    DefaultPipelineWorkspaceSchemaVersion,
				Targets:          tc.targets,
				Templates:        []PipelineTemplate{template},
				UpdatedAt:        now,
			}, DefaultPipelineWorkspaceSchemaVersion, now, nil, nil, nil)

			var normalizedTemplate *PipelineTemplate
			for index := range workspace.Templates {
				if workspace.Templates[index].ID == "template-a" {
					normalizedTemplate = &workspace.Templates[index]
					break
				}
			}
			if normalizedTemplate == nil {
				t.Fatalf("template-a missing from normalized workspace: %#v", workspace.Templates)
			}
			if got := pipelineDomainFromSnapshot(normalizedTemplate.BoundConfigSnapshot); got != tc.wantDomain {
				t.Fatalf("bound_config_snapshot domain = %q, want %q", got, tc.wantDomain)
			}
			if got := workspace.ActiveTargetID; got != tc.wantTargetID {
				t.Fatalf("active_target_id = %q, want %q", got, tc.wantTargetID)
			}
			var normalizedTarget *PipelineTarget
			for index := range workspace.Targets {
				if workspace.Targets[index].ID == tc.wantTargetID {
					normalizedTarget = &workspace.Targets[index]
					break
				}
			}
			if normalizedTarget == nil {
				t.Fatalf("expected target missing from normalized workspace: %#v", workspace.Targets)
			}
			if got := normalizedTarget.ID; got != tc.wantTargetID {
				t.Fatalf("compatibility target id = %q, want %q", got, tc.wantTargetID)
			}
		})
	}
}

func TestNormalizePipelineWorkspaceForSaveKeepsMultipleTargetsPerTemplate(t *testing.T) {
	now := "2026-05-24T10:00:00Z"
	targetSnapshot := func(domain string) map[string]any {
		return map[string]any{"cloudflare": map[string]any{"record_name": domain}}
	}
	template := DefaultPipelineTemplate(now)
	template.ID = "template-a"
	template.Name = "Template A"
	workspace := NormalizePipelineWorkspaceForSave(PipelineWorkspace{
		ActiveTargetID:   "target-b",
		ActiveTemplateID: "template-a",
		SchemaVersion:    DefaultPipelineWorkspaceSchemaVersion,
		Targets: []PipelineTarget{
			{ConfigSnapshot: targetSnapshot("a.example.com"), CreatedAt: now, Enabled: true, ID: "target-a", Name: "A", TemplateID: "template-a", UpdatedAt: now},
			{ConfigSnapshot: targetSnapshot("b.example.com"), CreatedAt: now, Enabled: true, ID: "target-b", Name: "B", TemplateID: "template-a", UpdatedAt: now},
			{ConfigSnapshot: targetSnapshot("c.example.com"), CreatedAt: now, Enabled: false, ID: "target-c", Name: "C", TemplateID: "template-a", UpdatedAt: now},
		},
		Templates: []PipelineTemplate{template},
		UpdatedAt: now,
	}, DefaultPipelineWorkspaceSchemaVersion, now, nil, nil, nil)

	targetIDs := make([]string, 0, len(workspace.Targets))
	for _, target := range workspace.Targets {
		if target.TemplateID == "template-a" {
			targetIDs = append(targetIDs, target.ID)
		}
	}
	if got, want := strings.Join(targetIDs, ","), "target-a,target-b,target-c"; got != want {
		t.Fatalf("template target ids = %q, want %q", got, want)
	}
	if workspace.ActiveTargetID != "target-b" {
		t.Fatalf("active_target_id = %q, want target-b", workspace.ActiveTargetID)
	}
}
