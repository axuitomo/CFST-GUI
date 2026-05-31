package appcore

import "testing"

func TestDefaultPipelineTemplateUsesThreeStageWorkflow(t *testing.T) {
	template := DefaultPipelineTemplate("2026-05-31T00:00:00Z")
	if template.EntryNodeID != "source-group-main" {
		t.Fatalf("entry_node_id = %q, want source-group-main", template.EntryNodeID)
	}
	if got := len(template.Nodes); got != 3 {
		t.Fatalf("nodes len = %d, want 3", got)
	}
	wantNodes := map[string]struct {
		action   string
		nodeType string
	}{
		"source-group-main": {PipelineNodeActionSelectSources, PipelineNodeTypeSource},
		"probe-main":        {PipelineNodeActionRunProbe, PipelineNodeTypeProbe},
		"check-output":      {PipelineNodeActionCheckOutput, PipelineNodeTypeEnd},
	}
	for _, node := range template.Nodes {
		want, ok := wantNodes[node.ID]
		if !ok {
			t.Fatalf("unexpected node %#v", node)
		}
		if node.Action != want.action || node.NodeType != want.nodeType {
			t.Fatalf("node %s = (%s,%s), want (%s,%s)", node.ID, node.Action, node.NodeType, want.action, want.nodeType)
		}
		if node.ID == "probe-main" {
			if node.Config["strategy"] != "full" {
				t.Fatalf("probe-main strategy = %#v, want full", node.Config["strategy"])
			}
			if node.Config["download_enabled"] != true {
				t.Fatalf("probe-main download_enabled = %#v, want true", node.Config["download_enabled"])
			}
		}
	}
	if got := len(template.Edges); got != 2 {
		t.Fatalf("edges len = %d, want 2", got)
	}
	if err := ValidatePipelineTemplate(template); err != nil {
		t.Fatalf("default template should validate: %v", err)
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
	if len(workspace.Templates) != 2 {
		t.Fatalf("templates len = %d, want default + custom", len(workspace.Templates))
	}
	if workspace.Templates[0].ID != DefaultPipelineTemplateID {
		t.Fatalf("first template id = %q, want default", workspace.Templates[0].ID)
	}
	if workspace.ActiveTemplateID != "custom-template" {
		t.Fatalf("active_template_id = %q, want custom-template", workspace.ActiveTemplateID)
	}
}

func TestNormalizePipelineWorkspaceForSaveCompletesDefaultProbeConfig(t *testing.T) {
	now := "2026-05-31T00:00:00Z"
	template := DefaultPipelineTemplate(now)
	for index := range template.Nodes {
		if template.Nodes[index].ID == "probe-main" {
			template.Nodes[index].Config = map[string]any{}
		}
	}
	workspace := NormalizePipelineWorkspaceForSave(PipelineWorkspace{
		ActiveTemplateID: DefaultPipelineTemplateID,
		SchemaVersion:    DefaultPipelineWorkspaceSchemaVersion,
		Templates:        []PipelineTemplate{template},
		UpdatedAt:        now,
	}, DefaultPipelineWorkspaceSchemaVersion, now, nil, nil, nil)
	for _, node := range workspace.Templates[0].Nodes {
		if node.ID != "probe-main" {
			continue
		}
		if node.Config["strategy"] != "full" || node.Config["download_enabled"] != true || node.Config["source_mode"] != "inherit" {
			t.Fatalf("probe-main config = %#v, want full download defaults", node.Config)
		}
		return
	}
	t.Fatal("probe-main not found")
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
				if workspace.Targets[index].TemplateID == "template-a" {
					normalizedTarget = &workspace.Targets[index]
					break
				}
			}
			if normalizedTarget == nil {
				t.Fatalf("template-a target missing from normalized workspace: %#v", workspace.Targets)
			}
			if got := normalizedTarget.ID; got != tc.wantTargetID {
				t.Fatalf("compatibility target id = %q, want %q", got, tc.wantTargetID)
			}
			if !normalizedTarget.Enabled {
				t.Fatalf("compatibility target should stay enabled in single-target mode")
			}
		})
	}
}
