package appcore

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

const (
	DefaultPipelineProfilesSchemaVersion  = "cfst-gui-pipeline-profiles-v1"
	DefaultPipelineWorkspaceSchemaVersion = "cfst-gui-pipeline-workspace-v1"
	DefaultPipelineProfileID              = "pipeline-profile-default"
	DefaultPipelineTemplateID             = "pipeline-template-default"
	DefaultPipelineTargetID               = "pipeline-target-default"
	PipelineDNSPushPolicyAuto             = "auto"
	PipelineDNSPushPolicySkip             = "skip"
	PipelineNodeTypeSource                = "source"
	PipelineNodeTypeProbe                 = "probe"
	PipelineNodeTypeFilter                = "filter"
	PipelineNodeTypeBranch                = "branch"
	PipelineNodeTypeDeliver               = "deliver"
	PipelineNodeTypeRecovery              = "recovery"
	PipelineNodeTypeEnd                   = "end"
	PipelineNodeActionSelectSources       = "select_sources"
	PipelineNodeActionRunProbe            = "run_probe"
	PipelineNodeActionFilterResults       = "filter_results"
	PipelineNodeActionBranchHasResults    = "branch_has_results"
	PipelineNodeActionDeliverDNS          = "deliver_dns"
	PipelineNodeActionDeliverGitHub       = "deliver_github"
	PipelineNodeActionRecoveryMark        = "recovery_mark"
	PipelineNodeActionCheckOutput         = "check_output"
	PipelineNodeActionEnd                 = "end"
)

type PipelineProfile struct {
	ConfigSnapshot map[string]any `json:"config_snapshot"`
	CreatedAt      string         `json:"created_at"`
	DNSPushPolicy  string         `json:"dns_push_policy"`
	Domain         string         `json:"domain"`
	Enabled        bool           `json:"enabled"`
	ID             string         `json:"id"`
	Name           string         `json:"name"`
	Region         string         `json:"region"`
	UpdatedAt      string         `json:"updated_at"`
}

type PipelineCanvasPosition struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type PipelineViewport struct {
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
	Zoom float64 `json:"zoom"`
}

type PipelineNodeUI struct {
	Collapsed bool                    `json:"collapsed,omitempty"`
	Position  *PipelineCanvasPosition `json:"position,omitempty"`
	Width     float64                 `json:"width,omitempty"`
}

type PipelineTemplateUI struct {
	Viewport *PipelineViewport `json:"viewport,omitempty"`
}

type PipelineNode struct {
	Action    string          `json:"action"`
	Config    map[string]any  `json:"config,omitempty"`
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	NodeType  string          `json:"node_type"`
	UI        *PipelineNodeUI `json:"ui,omitempty"`
	UpdatedAt string          `json:"updated_at,omitempty"`
}

type PipelineEdge struct {
	ID         string `json:"id"`
	Label      string `json:"label,omitempty"`
	Outcome    string `json:"outcome,omitempty"`
	SourceNode string `json:"source_node_id"`
	TargetNode string `json:"target_node_id"`
}

type PipelineTemplate struct {
	BoundConfigSnapshot map[string]any      `json:"bound_config_snapshot,omitempty"`
	CreatedAt           string              `json:"created_at"`
	Description         string              `json:"description"`
	Enabled             bool                `json:"enabled"`
	EntryNodeID         string              `json:"entry_node_id"`
	Edges               []PipelineEdge      `json:"edges"`
	ID                  string              `json:"id"`
	Name                string              `json:"name"`
	Nodes               []PipelineNode      `json:"nodes"`
	UI                  *PipelineTemplateUI `json:"ui,omitempty"`
	UpdatedAt           string              `json:"updated_at"`
	Version             int                 `json:"version"`
}

type PipelineTarget struct {
	ConfigSnapshot map[string]any `json:"config_snapshot"`
	CreatedAt      string         `json:"created_at"`
	DNSPushPolicy  string         `json:"dns_push_policy"`
	Domain         string         `json:"domain"`
	Enabled        bool           `json:"enabled"`
	ID             string         `json:"id"`
	Name           string         `json:"name"`
	Region         string         `json:"region"`
	Tags           []string       `json:"tags,omitempty"`
	TemplateID     string         `json:"template_id"`
	UpdatedAt      string         `json:"updated_at"`
}

type PipelineWorkspace struct {
	ActiveTargetID   string             `json:"active_target_id"`
	ActiveTemplateID string             `json:"active_template_id"`
	SchemaVersion    string             `json:"schema_version"`
	Targets          []PipelineTarget   `json:"targets"`
	Templates        []PipelineTemplate `json:"templates"`
	UpdatedAt        string             `json:"updated_at"`
}

type PipelineProfileStore struct {
	ActiveProfileID string            `json:"active_profile_id"`
	Items           []PipelineProfile `json:"items"`
	SchemaVersion   string            `json:"schema_version"`
	UpdatedAt       string            `json:"updated_at"`
}

type PipelineRunPayload struct {
	ConfigSource       string                   `json:"config_source"`
	PipelineID         string                   `json:"pipeline_id"`
	ProfileIDs         []string                 `json:"profile_ids"`
	Profiles           []PipelineProfile        `json:"profiles"`
	SchedulerOverrides PipelineRuntimeOverrides `json:"scheduler_overrides,omitempty"`
	TargetIDs          []string                 `json:"target_ids"`
	TaskID             string                   `json:"task_id"`
	TemplateID         string                   `json:"template_id"`
	Workspace          PipelineWorkspace        `json:"workspace"`
}

type PipelineNodeRunResult struct {
	Action        string         `json:"action"`
	BranchTaken   string         `json:"branch_taken,omitempty"`
	CompletedAt   string         `json:"completed_at"`
	Message       string         `json:"message"`
	Metrics       map[string]any `json:"metrics,omitempty"`
	NodeID        string         `json:"node_id"`
	NodeName      string         `json:"node_name"`
	NodeType      string         `json:"node_type"`
	Outcome       string         `json:"outcome,omitempty"`
	OutputSummary string         `json:"output_summary,omitempty"`
	StartedAt     string         `json:"started_at"`
	Status        string         `json:"status"`
}

type PipelineRuntimeOverrides struct {
	AllowDNSPush *bool `json:"allow_dns_push,omitempty"`
}

type PipelineNodeCatalogFieldOption struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type PipelineNodeCatalogFieldVisibleWhen struct {
	Equals    any    `json:"equals,omitempty"`
	Field     string `json:"field"`
	NotEquals any    `json:"not_equals,omitempty"`
}

type PipelineNodeCatalogField struct {
	DefaultValue any                                  `json:"default_value,omitempty"`
	Description  string                               `json:"description,omitempty"`
	FieldType    string                               `json:"field_type"`
	Group        string                               `json:"group,omitempty"`
	HelpText     string                               `json:"help_text,omitempty"`
	Key          string                               `json:"key"`
	Label        string                               `json:"label"`
	Max          *float64                             `json:"max,omitempty"`
	Min          *float64                             `json:"min,omitempty"`
	Options      []PipelineNodeCatalogFieldOption     `json:"options,omitempty"`
	Placeholder  string                               `json:"placeholder,omitempty"`
	Required     bool                                 `json:"required,omitempty"`
	Rows         int                                  `json:"rows,omitempty"`
	Step         *float64                             `json:"step,omitempty"`
	VisibleWhen  *PipelineNodeCatalogFieldVisibleWhen `json:"visible_when,omitempty"`
}

type PipelineNodeCatalogOutcome struct {
	Description string `json:"description,omitempty"`
	Label       string `json:"label"`
	Value       string `json:"value"`
}

type PipelineNodeCatalogItem struct {
	Action        string                       `json:"action"`
	DefaultConfig map[string]any               `json:"default_config"`
	Description   string                       `json:"description,omitempty"`
	DisplayName   string                       `json:"display_name"`
	FormSchema    []PipelineNodeCatalogField   `json:"form_schema,omitempty"`
	NodeType      string                       `json:"node_type"`
	Outcomes      []PipelineNodeCatalogOutcome `json:"outcomes,omitempty"`
}

type PipelineProfileRunResult struct {
	DNSResult   any                     `json:"dns_result,omitempty"`
	Domain      string                  `json:"domain"`
	Message     string                  `json:"message"`
	NodeResults []PipelineNodeRunResult `json:"node_results,omitempty"`
	ProfileID   string                  `json:"profile_id"`
	ProfileName string                  `json:"profile_name"`
	ProbeResult *ProbeRunResult         `json:"probe_result,omitempty"`
	Region      string                  `json:"region"`
	Status      string                  `json:"status"`
	TaskID      string                  `json:"task_id"`
	TargetID    string                  `json:"target_id,omitempty"`
	TargetName  string                  `json:"target_name,omitempty"`
	Warnings    []string                `json:"warnings,omitempty"`
}

type PipelineRunResult struct {
	CompletedAt   string                     `json:"completed_at"`
	DurationMS    int64                      `json:"duration_ms"`
	Failed        int                        `json:"failed"`
	PipelineID    string                     `json:"pipeline_id"`
	Results       []PipelineProfileRunResult `json:"results"`
	Skipped       int                        `json:"skipped"`
	StartedAt     string                     `json:"started_at"`
	Status        string                     `json:"status"`
	Succeeded     int                        `json:"succeeded"`
	TaskID        string                     `json:"task_id"`
	TargetIDs     []string                   `json:"target_ids,omitempty"`
	TargetResults []PipelineProfileRunResult `json:"target_results,omitempty"`
	TemplateID    string                     `json:"template_id,omitempty"`
	Total         int                        `json:"total"`
	Warnings      []string                   `json:"warnings,omitempty"`
}

func floatPtr(value float64) *float64 {
	return &value
}

func LoadPipelineProfileStore(path string, schemaVersion string, sanitize func(map[string]any) map[string]any) (PipelineProfileStore, error) {
	store := PipelineProfileStore{
		Items:         []PipelineProfile{},
		SchemaVersion: schemaVersion,
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return store, nil
		}
		return store, err
	}
	if _, err := UnmarshalJSONCompat(raw, &store); err != nil {
		return store, err
	}
	return NormalizePipelineProfileStoreForSave(store, schemaVersion, time.Now().Format(time.RFC3339), sanitize, nil), nil
}

func SavePipelineProfileStore(path string, store PipelineProfileStore, schemaVersion string, sanitize func(map[string]any) map[string]any) error {
	store = NormalizePipelineProfileStoreForSave(store, schemaVersion, time.Now().Format(time.RFC3339), sanitize, nil)
	raw, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	return WriteFileAtomic(path, raw, 0o600)
}

func LoadPipelineWorkspace(path string, legacyPath string, schemaVersion string, now string, sanitize func(map[string]any) map[string]any) (PipelineWorkspace, bool, error) {
	workspace := PipelineWorkspace{
		SchemaVersion: schemaVersion,
		Targets:       []PipelineTarget{},
		Templates:     []PipelineTemplate{},
	}
	raw, err := os.ReadFile(path)
	if err == nil {
		if _, err := UnmarshalJSONCompat(raw, &workspace); err != nil {
			return workspace, false, err
		}
		return NormalizePipelineWorkspaceForSave(workspace, schemaVersion, now, sanitize, nil, nil), false, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return workspace, false, err
	}
	if strings.TrimSpace(legacyPath) == "" {
		return workspace, false, nil
	}
	legacyStore, err := LoadPipelineProfileStore(legacyPath, DefaultPipelineProfilesSchemaVersion, sanitize)
	if err != nil {
		return workspace, false, err
	}
	if len(legacyStore.Items) == 0 {
		return workspace, false, nil
	}
	return PipelineWorkspaceFromProfileStore(legacyStore, schemaVersion, now, sanitize), true, nil
}

func SavePipelineWorkspace(path string, workspace PipelineWorkspace, schemaVersion string, now string, sanitize func(map[string]any) map[string]any) error {
	workspace = NormalizePipelineWorkspaceForSave(workspace, schemaVersion, now, sanitize, nil, nil)
	raw, err := json.MarshalIndent(workspace, "", "  ")
	if err != nil {
		return err
	}
	return WriteFileAtomic(path, raw, 0o600)
}

func DefaultPipelineProfileStoreFromSnapshot(snapshot map[string]any, schemaVersion string, now string, sanitize func(map[string]any) map[string]any) PipelineProfileStore {
	now = strings.TrimSpace(now)
	if now == "" {
		now = time.Now().Format(time.RFC3339)
	}
	if sanitize != nil {
		snapshot = sanitize(snapshot)
	}
	domain := ""
	if cloudflare, ok := snapshot["cloudflare"].(map[string]any); ok {
		domain = strings.TrimSpace(stringValue(firstNonNil(cloudflare["record_name"], cloudflare["recordName"]), ""))
	}
	return PipelineProfileStore{
		ActiveProfileID: DefaultPipelineProfileID,
		Items: []PipelineProfile{
			{
				ConfigSnapshot: clonePipelineSnapshot(snapshot),
				CreatedAt:      now,
				DNSPushPolicy:  PipelineDNSPushPolicyAuto,
				Domain:         domain,
				Enabled:        true,
				ID:             DefaultPipelineProfileID,
				Name:           "默认策略",
				Region:         "默认地域",
				UpdatedAt:      now,
			},
		},
		SchemaVersion: schemaVersion,
		UpdatedAt:     now,
	}
}

func DefaultPipelineNodeCatalog() []PipelineNodeCatalogItem {
	return []PipelineNodeCatalogItem{
		{
			Action: PipelineNodeActionSelectSources,
			DefaultConfig: map[string]any{
				"source_ids": []any{},
			},
			Description: "从当前绑定配置里选择一个或多个输入源，作为后续测速的输入组。",
			DisplayName: "输入源组",
			FormSchema: []PipelineNodeCatalogField{
				{
					DefaultValue: []any{},
					FieldType:    "json",
					Group:        "输入组",
					HelpText:     "留空表示使用当前绑定配置中的全部启用输入源；桌面画布会提供勾选操作。",
					Key:          "source_ids",
					Label:        "输入源 ID",
					Rows:         4,
				},
			},
			NodeType: PipelineNodeTypeSource,
		},
		{
			Action: PipelineNodeActionRunProbe,
			DefaultConfig: map[string]any{
				"download_enabled": true,
				"source_mode":      "inherit",
				"strategy":         "full",
			},
			Description: "依次执行 TCP 延迟测速、追踪测试和下载测速，产出 probe_result 供后续节点消费。",
			DisplayName: "测速",
			FormSchema: []PipelineNodeCatalogField{
				{
					DefaultValue: "inherit",
					FieldType:    "select",
					Group:        "输入源",
					HelpText:     "默认继承当前目标绑定的输入源；需要在节点内单独配置时再切到自定义。",
					Key:          "source_mode",
					Label:        "输入源模式",
					Options: []PipelineNodeCatalogFieldOption{
						{Label: "继承绑定配置", Value: "inherit"},
						{Label: "使用自定义输入源", Value: "custom"},
					},
				},
				{
					DefaultValue: []any{},
					FieldType:    "json",
					Group:        "输入源",
					HelpText:     "填写 DesktopSourceConfig 数组，结构与“输入源”页面保存的内容一致。",
					Key:          "sources",
					Label:        "自定义输入源",
					Rows:         8,
					VisibleWhen: &PipelineNodeCatalogFieldVisibleWhen{
						Equals: "custom",
						Field:  "source_mode",
					},
				},
				{
					DefaultValue: 500,
					FieldType:    "number",
					Group:        "输入源",
					HelpText:     "覆盖当前节点里每个输入源的单源候选上限。",
					Key:          "source_ip_limit",
					Label:        "单源 IP 上限",
					Min:          floatPtr(1),
					Step:         floatPtr(1),
				},
				{
					DefaultValue: "traverse",
					FieldType:    "select",
					Group:        "输入源",
					HelpText:     "遍历模式直接读取候选；MCIS 模式会先做搜索。",
					Key:          "source_ip_mode",
					Label:        "输入源模式",
					Options: []PipelineNodeCatalogFieldOption{
						{Label: "遍历", Value: "traverse"},
						{Label: "MCIS 搜索", Value: "mcis"},
					},
				},
				{
					DefaultValue: "",
					FieldType:    "textarea",
					Group:        "输入源",
					HelpText:     "为当前节点所有输入源统一附加 colo 筛选词。",
					Key:          "source_colo_filter",
					Label:        "源级 Colo 筛选",
					Rows:         3,
				},
				{
					DefaultValue: "allow",
					FieldType:    "select",
					Group:        "输入源",
					Key:          "source_colo_filter_mode",
					Label:        "Colo 筛选方式",
					Options: []PipelineNodeCatalogFieldOption{
						{Label: "仅允许", Value: "allow"},
						{Label: "排除", Value: "deny"},
					},
				},
				{
					DefaultValue: 443,
					FieldType:    "number",
					Group:        "测速阶段",
					Key:          "tcp_port",
					Label:        "全局测速端口",
					Max:          floatPtr(65535),
					Min:          floatPtr(1),
					Step:         floatPtr(1),
				},
				{
					DefaultValue: "source_override_global",
					FieldType:    "select",
					Group:        "测速阶段",
					HelpText:     "决定输入源里自带端口时，是沿用源端口还是固定用全局端口。",
					Key:          "port_policy",
					Label:        "端口策略",
					Options: []PipelineNodeCatalogFieldOption{
						{Label: "输入源端口优先", Value: "source_override_global"},
						{Label: "固定全局端口", Value: "fixed_global"},
					},
				},
				{
					DefaultValue: "full",
					FieldType:    "select",
					Group:        "测速阶段",
					HelpText:     "快速模式会跳过下载测速，仅保留延迟/追踪阶段。",
					Key:          "strategy",
					Label:        "测速策略",
					Options: []PipelineNodeCatalogFieldOption{
						{Label: "完整测速", Value: "full"},
						{Label: "快速模式", Value: "fast"},
					},
				},
				{
					DefaultValue: true,
					FieldType:    "checkbox",
					Group:        "测速阶段",
					HelpText:     "关闭后会自动切到快速模式，跳过下载测速阶段。",
					Key:          "download_enabled",
					Label:        "启用下载测速",
				},
				{
					DefaultValue: "average",
					FieldType:    "select",
					Group:        "测速阶段",
					Key:          "download_speed_metric",
					Label:        "结果排序指标",
					Options: []PipelineNodeCatalogFieldOption{
						{Label: "平均速度", Value: "average"},
						{Label: "峰值速度", Value: "max"},
					},
				},
				{
					DefaultValue: 10,
					FieldType:    "number",
					Group:        "测速阶段",
					Key:          "download_count",
					Label:        "下载测速数量",
					Min:          floatPtr(1),
					Step:         floatPtr(1),
				},
				{
					DefaultValue: 0.15,
					FieldType:    "number",
					Group:        "阈值",
					Key:          "max_loss_rate",
					Label:        "最大丢包率",
					Max:          floatPtr(1),
					Min:          floatPtr(0),
					Step:         floatPtr(0.01),
				},
				{
					FieldType: "number",
					Group:     "阈值",
					Key:       "max_tcp_latency_ms",
					Label:     "最大 TCP 延迟(ms)",
					Min:       floatPtr(1),
					Step:      floatPtr(1),
				},
				{
					DefaultValue: 0,
					FieldType:    "number",
					Group:        "阈值",
					Key:          "min_download_mbps",
					Label:        "最小下载速度(MB/s)",
					Min:          floatPtr(0),
					Step:         floatPtr(0.1),
				},
			},
			NodeType: PipelineNodeTypeProbe,
		},
		{
			Action: PipelineNodeActionFilterResults,
			DefaultConfig: map[string]any{
				"source": "probe_results",
				"status": "passed",
			},
			Description: "按共享上传规则筛选 probe 结果，产出 filtered_rows。",
			DisplayName: "结果筛选",
			FormSchema: []PipelineNodeCatalogField{
				{
					DefaultValue: "probe_results",
					FieldType:    "select",
					Group:        "数据来源",
					HelpText:     "通常保持默认。只有想继续处理上一步筛选后的结果时，才改成“已筛选结果”。",
					Key:          "source",
					Label:        "筛选输入",
					Options: []PipelineNodeCatalogFieldOption{
						{Label: "测速结果", Value: "probe_results"},
						{Label: "已筛选结果", Value: "filtered_rows"},
					},
				},
				{
					DefaultValue: "passed",
					FieldType:    "select",
					Group:        "筛选条件",
					Key:          "status",
					Label:        "结果状态",
					Options: []PipelineNodeCatalogFieldOption{
						{Label: "仅成功结果", Value: "passed"},
						{Label: "全部结果", Value: "all"},
					},
				},
				{
					DefaultValue: "any",
					FieldType:    "select",
					Group:        "筛选条件",
					Key:          "ip_version",
					Label:        "IP 版本",
					Options: []PipelineNodeCatalogFieldOption{
						{Label: "全部", Value: "any"},
						{Label: "仅 IPv4", Value: "ipv4"},
						{Label: "仅 IPv6", Value: "ipv6"},
					},
				},
				{
					FieldType: "number",
					Group:     "筛选条件",
					Key:       "max_loss_rate",
					Label:     "最大丢包率",
					Max:       floatPtr(1),
					Min:       floatPtr(0),
					Step:      floatPtr(0.01),
				},
				{
					FieldType: "number",
					Group:     "筛选条件",
					Key:       "max_tcp_latency_ms",
					Label:     "最大 TCP 延迟(ms)",
					Min:       floatPtr(1),
					Step:      floatPtr(1),
				},
				{
					FieldType: "number",
					Group:     "筛选条件",
					Key:       "max_trace_latency_ms",
					Label:     "最大追踪延迟(ms)",
					Min:       floatPtr(1),
					Step:      floatPtr(1),
				},
				{
					DefaultValue: 0,
					FieldType:    "number",
					Group:        "筛选条件",
					Key:          "min_download_mbps",
					Label:        "最小下载速度(MB/s)",
					Min:          floatPtr(0),
					Step:         floatPtr(0.1),
				},
				{
					DefaultValue: "",
					FieldType:    "textarea",
					Group:        "筛选条件",
					Key:          "colo_allow",
					Label:        "仅允许的 Colo",
					Placeholder:  "例如 HKG,SJC",
					Rows:         3,
				},
				{
					DefaultValue: "",
					FieldType:    "textarea",
					Group:        "筛选条件",
					Key:          "colo_deny",
					Label:        "排除的 Colo",
					Placeholder:  "例如 LAX,NRT",
					Rows:         3,
				},
				{
					DefaultValue: 0,
					FieldType:    "number",
					Group:        "筛选条件",
					HelpText:     "大于 0 时，只保留排序后的前 N 条结果继续向下游传递。",
					Key:          "top_n",
					Label:        "保留前 N 条",
					Min:          floatPtr(0),
					Step:         floatPtr(1),
				},
			},
			NodeType: PipelineNodeTypeFilter,
		},
		{
			Action: PipelineNodeActionBranchHasResults,
			DefaultConfig: map[string]any{
				"source": "filtered_rows",
			},
			Description: "检查当前结果集是否还有可继续投递的结果，并按 outcome 选择下一条边。",
			DisplayName: "结果检查",
			FormSchema: []PipelineNodeCatalogField{
				{
					DefaultValue: "filtered_rows",
					FieldType:    "select",
					Group:        "数据来源",
					HelpText:     "一般选“筛选结果”。如果你想直接拿测速结果做判断，再切到“测速结果”。",
					Key:          "source",
					Label:        "检查输入",
					Options: []PipelineNodeCatalogFieldOption{
						{Label: "筛选结果", Value: "filtered_rows"},
						{Label: "测速结果", Value: "probe_results"},
					},
				},
			},
			NodeType: PipelineNodeTypeBranch,
			Outcomes: []PipelineNodeCatalogOutcome{
				{Description: "存在可继续处理的结果。", Label: "有结果", Value: "true"},
				{Description: "当前没有可继续处理的结果。", Label: "无结果", Value: "false"},
			},
		},
		{
			Action: PipelineNodeActionDeliverDNS,
			DefaultConfig: map[string]any{
				"source": "filtered_rows",
			},
			Description: "把当前筛选结果推送到 Cloudflare DNS。",
			DisplayName: "DNS 推送",
			FormSchema: []PipelineNodeCatalogField{
				{
					DefaultValue: "filtered_rows",
					FieldType:    "select",
					Group:        "数据来源",
					Key:          "source",
					Label:        "推送输入",
					Options: []PipelineNodeCatalogFieldOption{
						{Label: "筛选结果", Value: "filtered_rows"},
						{Label: "测速结果", Value: "probe_results"},
					},
				},
				{
					DefaultValue: 0,
					FieldType:    "number",
					Group:        "推送行为",
					Key:          "top_n",
					Label:        "推送前 N 条",
					Min:          floatPtr(0),
					Step:         floatPtr(1),
				},
				{
					FieldType:   "text",
					Group:       "DNS 记录",
					HelpText:    "留空时继承工作流绑定配置里的记录名。",
					Key:         "record_name",
					Label:       "记录名",
					Placeholder: "sub.example.com",
				},
				{
					DefaultValue: "A",
					FieldType:    "select",
					Group:        "DNS 记录",
					Key:          "record_type",
					Label:        "记录类型",
					Options: []PipelineNodeCatalogFieldOption{
						{Label: "A (IPv4)", Value: "A"},
						{Label: "AAAA (IPv6)", Value: "AAAA"},
					},
				},
				{
					DefaultValue: 300,
					FieldType:    "number",
					Group:        "DNS 记录",
					Key:          "ttl",
					Label:        "TTL",
					Min:          floatPtr(1),
					Step:         floatPtr(1),
				},
				{
					DefaultValue: false,
					FieldType:    "checkbox",
					Group:        "DNS 记录",
					Key:          "proxied",
					Label:        "启用代理",
				},
				{
					FieldType:   "text",
					Group:       "DNS 记录",
					Key:         "comment",
					Label:       "注释",
					Placeholder: "可选，留空则沿用绑定配置。",
				},
			},
			NodeType: PipelineNodeTypeDeliver,
		},
		{
			Action:        PipelineNodeActionDeliverGitHub,
			DefaultConfig: map[string]any{},
			Description:   "把当前筛选结果导出到 GitHub。",
			DisplayName:   "GitHub 导出",
			NodeType:      PipelineNodeTypeDeliver,
		},
		{
			Action: PipelineNodeActionRecoveryMark,
			DefaultConfig: map[string]any{
				"message": "需要人工复核。",
				"status":  "manual_review",
			},
			Description: "记录恢复/回退原因，为后续 end 节点提供上下文。",
			DisplayName: "人工复核标记",
			FormSchema: []PipelineNodeCatalogField{
				{
					DefaultValue: "manual_review",
					FieldType:    "select",
					Group:        "结果状态",
					HelpText:     "这里决定这一步对外显示成什么状态。",
					Key:          "status",
					Label:        "标记状态",
					Options: []PipelineNodeCatalogFieldOption{
						{Label: "人工复核", Value: "manual_review"},
						{Label: "已跳过", Value: "skipped"},
						{Label: "失败", Value: "failed"},
					},
				},
				{
					DefaultValue: "需要人工复核。",
					FieldType:    "textarea",
					Group:        "说明",
					Key:          "message",
					Label:        "说明",
					Placeholder:  "说明为什么需要人工复核。",
					Rows:         4,
				},
			},
			NodeType: PipelineNodeTypeRecovery,
		},
		{
			Action: PipelineNodeActionEnd,
			DefaultConfig: map[string]any{
				"message": "流程已结束。",
				"status":  "completed",
			},
			Description: "声明当前路径的最终状态和说明。",
			DisplayName: "结束",
			FormSchema: []PipelineNodeCatalogField{
				{
					DefaultValue: "completed",
					FieldType:    "select",
					Group:        "结果状态",
					HelpText:     "这里决定流程最后在运行记录里显示成完成、失败还是需要手动处理。",
					Key:          "status",
					Label:        "最终状态",
					Options: []PipelineNodeCatalogFieldOption{
						{Label: "完成", Value: "completed"},
						{Label: "人工复核", Value: "manual_review"},
						{Label: "已跳过", Value: "skipped"},
						{Label: "失败", Value: "failed"},
						{Label: "部分完成", Value: "partial"},
					},
				},
				{
					DefaultValue: "流程已结束。",
					FieldType:    "textarea",
					Group:        "说明",
					Key:          "message",
					Label:        "结束说明",
					Placeholder:  "展示给运行结果区的说明。",
					Rows:         4,
				},
			},
			NodeType: PipelineNodeTypeEnd,
		},
		{
			Action: PipelineNodeActionCheckOutput,
			DefaultConfig: map[string]any{
				"export_if_missing": true,
				"require_csv":       true,
				"source":            "probe_results",
			},
			Description: "检查测速结果与 CSV 写入状态，必要时按当前导出配置补写结果。",
			DisplayName: "结果检查与输出",
			FormSchema: []PipelineNodeCatalogField{
				{
					DefaultValue: "probe_results",
					FieldType:    "select",
					Group:        "结果检查",
					Key:          "source",
					Label:        "检查输入",
					Options: []PipelineNodeCatalogFieldOption{
						{Label: "测速结果", Value: "probe_results"},
						{Label: "已筛选结果", Value: "filtered_rows"},
					},
				},
				{
					DefaultValue: true,
					FieldType:    "checkbox",
					Group:        "CSV 输出",
					Key:          "require_csv",
					Label:        "要求 CSV 写入",
				},
				{
					DefaultValue: true,
					FieldType:    "checkbox",
					Group:        "CSV 输出",
					HelpText:     "CSV 缺失且存在结果时，按工作流绑定配置里的导出路径补写。",
					Key:          "export_if_missing",
					Label:        "缺失时补写 CSV",
				},
			},
			NodeType: PipelineNodeTypeEnd,
		},
	}
}

func pipelineNodeCatalogByAction() map[string]PipelineNodeCatalogItem {
	items := DefaultPipelineNodeCatalog()
	index := make(map[string]PipelineNodeCatalogItem, len(items))
	for _, item := range items {
		index[item.Action] = item
	}
	return index
}

func DefaultPipelineTemplate(now string) PipelineTemplate {
	now = strings.TrimSpace(now)
	if now == "" {
		now = time.Now().Format(time.RFC3339)
	}
	nodes := []PipelineNode{
		{
			Action: PipelineNodeActionSelectSources,
			Config: map[string]any{
				"source_ids": []any{},
			},
			ID:        "source-group-main",
			Name:      "输入源组",
			NodeType:  PipelineNodeTypeSource,
			UI:        &PipelineNodeUI{Position: &PipelineCanvasPosition{X: 60, Y: 120}, Width: 320},
			UpdatedAt: now,
		},
		{
			Action: PipelineNodeActionRunProbe,
			Config: map[string]any{
				"download_enabled": true,
				"source_mode":      "inherit",
				"strategy":         "full",
			},
			ID:        "probe-main",
			Name:      "测速",
			NodeType:  PipelineNodeTypeProbe,
			UI:        &PipelineNodeUI{Position: &PipelineCanvasPosition{X: 420, Y: 120}, Width: 320},
			UpdatedAt: now,
		},
		{
			Action: PipelineNodeActionCheckOutput,
			Config: map[string]any{
				"export_if_missing": true,
				"require_csv":       true,
				"source":            "probe_results",
			},
			ID:        "check-output",
			Name:      "结果检查与输出",
			NodeType:  PipelineNodeTypeEnd,
			UI:        &PipelineNodeUI{Position: &PipelineCanvasPosition{X: 780, Y: 120}, Width: 320},
			UpdatedAt: now,
		},
	}
	edges := []PipelineEdge{
		{ID: "edge-source-probe", SourceNode: "source-group-main", TargetNode: "probe-main"},
		{ID: "edge-probe-output", SourceNode: "probe-main", TargetNode: "check-output"},
	}
	return PipelineTemplate{
		BoundConfigSnapshot: map[string]any{},
		CreatedAt:           now,
		Description:         "默认流程：输入源组 -> 测速 -> 结果检查与输出",
		Enabled:             true,
		EntryNodeID:         "source-group-main",
		Edges:               edges,
		ID:                  DefaultPipelineTemplateID,
		Name:                "默认流程",
		Nodes:               nodes,
		UI:                  &PipelineTemplateUI{Viewport: &PipelineViewport{X: 0, Y: 0, Zoom: 0.95}},
		UpdatedAt:           now,
		Version:             1,
	}
}

func DefaultPipelineWorkspaceFromSnapshot(snapshot map[string]any, schemaVersion string, now string, sanitize func(map[string]any) map[string]any) PipelineWorkspace {
	now = strings.TrimSpace(now)
	if now == "" {
		now = time.Now().Format(time.RFC3339)
	}
	template := DefaultPipelineTemplate(now)
	if sanitize != nil {
		template.BoundConfigSnapshot = sanitize(snapshot)
	} else {
		template.BoundConfigSnapshot = clonePipelineSnapshot(snapshot)
	}
	workspace := PipelineWorkspace{
		ActiveTemplateID: DefaultPipelineTemplateID,
		SchemaVersion:    schemaVersion,
		Templates:        []PipelineTemplate{template},
		UpdatedAt:        now,
	}
	return NormalizePipelineWorkspaceForSave(workspace, DefaultPipelineWorkspaceSchemaVersion, now, sanitize, nil, nil)
}

func PipelineWorkspaceFromProfileStore(store PipelineProfileStore, schemaVersion string, now string, sanitize func(map[string]any) map[string]any) PipelineWorkspace {
	now = strings.TrimSpace(now)
	if now == "" {
		now = time.Now().Format(time.RFC3339)
	}
	template := DefaultPipelineTemplate(now)
	if profile := preferredPipelineProfile(store); profile != nil {
		template.BoundConfigSnapshot = clonePipelineSnapshot(profile.ConfigSnapshot)
		if sanitize != nil {
			template.BoundConfigSnapshot = sanitize(template.BoundConfigSnapshot)
		}
	}
	workspace := PipelineWorkspace{
		ActiveTemplateID: DefaultPipelineTemplateID,
		SchemaVersion:    schemaVersion,
		Templates:        []PipelineTemplate{template},
		UpdatedAt:        now,
	}
	return NormalizePipelineWorkspaceForSave(workspace, schemaVersion, now, sanitize, nil, nil)
}

func ensureDefaultPipelineTemplate(templates []PipelineTemplate, now string) []PipelineTemplate {
	defaultTemplate := DefaultPipelineTemplate(now)
	for index := range templates {
		if strings.TrimSpace(templates[index].ID) != DefaultPipelineTemplateID {
			continue
		}
		if !isLegacyDefaultPipelineTemplate(templates[index]) {
			templates[index] = ensureDefaultPipelineTemplateDefaults(templates[index])
			return templates
		}
		defaultTemplate.BoundConfigSnapshot = clonePipelineSnapshot(templates[index].BoundConfigSnapshot)
		defaultTemplate.CreatedAt = firstNonEmptyString(templates[index].CreatedAt, defaultTemplate.CreatedAt)
		defaultTemplate.UpdatedAt = now
		templates[index] = defaultTemplate
		return templates
	}
	return append([]PipelineTemplate{defaultTemplate}, templates...)
}

func ensureDefaultPipelineTemplateDefaults(template PipelineTemplate) PipelineTemplate {
	if strings.TrimSpace(template.ID) != DefaultPipelineTemplateID {
		return template
	}
	for index := range template.Nodes {
		node := &template.Nodes[index]
		if strings.TrimSpace(node.ID) != "probe-main" || normalizePipelineNodeAction(node.Action) != PipelineNodeActionRunProbe {
			continue
		}
		if node.Config == nil {
			node.Config = map[string]any{}
		}
		if _, ok := node.Config["source_mode"]; !ok {
			node.Config["source_mode"] = "inherit"
		}
		if _, ok := node.Config["download_enabled"]; !ok {
			node.Config["download_enabled"] = true
		}
		if _, ok := node.Config["strategy"]; !ok {
			node.Config["strategy"] = "full"
		}
	}
	return template
}

func isLegacyDefaultPipelineTemplate(template PipelineTemplate) bool {
	if strings.TrimSpace(template.ID) != DefaultPipelineTemplateID {
		return false
	}
	if strings.TrimSpace(template.EntryNodeID) != "probe-main" {
		return false
	}
	nodeIDs := make(map[string]struct{}, len(template.Nodes))
	for _, node := range template.Nodes {
		nodeIDs[strings.TrimSpace(node.ID)] = struct{}{}
	}
	_, hasLegacyBranch := nodeIDs["branch-results"]
	_, hasLegacyDNS := nodeIDs["deliver-dns"]
	_, hasSourceGroup := nodeIDs["source-group-main"]
	_, hasCheckOutput := nodeIDs["check-output"]
	return hasLegacyBranch && hasLegacyDNS && !hasSourceGroup && !hasCheckOutput
}

func LegacyPipelineProfileStoreFromWorkspace(workspace PipelineWorkspace, schemaVersion string, now string, sanitize func(map[string]any) map[string]any) PipelineProfileStore {
	workspace = NormalizePipelineWorkspaceForSave(workspace, DefaultPipelineWorkspaceSchemaVersion, now, sanitize, nil, nil)
	items := make([]PipelineProfile, 0, len(workspace.Targets))
	for index, target := range workspace.Targets {
		id := strings.TrimSpace(target.ID)
		if id == "" {
			id = fmt.Sprintf("pipeline-profile-%d", index+1)
		}
		items = append(items, PipelineProfile{
			ConfigSnapshot: clonePipelineSnapshot(target.ConfigSnapshot),
			CreatedAt:      target.CreatedAt,
			DNSPushPolicy:  NormalizePipelineDNSPushPolicy(target.DNSPushPolicy),
			Domain:         target.Domain,
			Enabled:        target.Enabled,
			ID:             id,
			Name:           target.Name,
			Region:         target.Region,
			UpdatedAt:      target.UpdatedAt,
		})
	}
	activeProfileID := strings.TrimSpace(workspace.ActiveTargetID)
	if activeProfileID == "" && len(items) > 0 {
		activeProfileID = items[0].ID
	}
	return NormalizePipelineProfileStoreForSave(PipelineProfileStore{
		ActiveProfileID: activeProfileID,
		Items:           items,
		SchemaVersion:   schemaVersion,
		UpdatedAt:       workspace.UpdatedAt,
	}, schemaVersion, now, sanitize, nil)
}

func NormalizePipelineProfileStoreForSave(store PipelineProfileStore, schemaVersion string, now string, sanitize func(map[string]any) map[string]any, newProfileID func(index int) string) PipelineProfileStore {
	now = strings.TrimSpace(now)
	if now == "" {
		now = time.Now().Format(time.RFC3339)
	}
	if strings.TrimSpace(store.SchemaVersion) == "" {
		store.SchemaVersion = schemaVersion
	}
	store.UpdatedAt = firstNonEmptyString(store.UpdatedAt, now)
	if store.Items == nil {
		store.Items = []PipelineProfile{}
	}
	for index := range store.Items {
		item := &store.Items[index]
		if strings.TrimSpace(item.ID) == "" {
			if newProfileID != nil {
				item.ID = strings.TrimSpace(newProfileID(index))
			}
			if strings.TrimSpace(item.ID) == "" {
				item.ID = fmt.Sprintf("pipeline-profile-%d", time.Now().UnixNano()+int64(index))
			}
		}
		if strings.TrimSpace(item.Name) == "" {
			item.Name = fmt.Sprintf("策略 %d", index+1)
		}
		if strings.TrimSpace(item.Region) == "" {
			item.Region = "未分组"
		}
		item.DNSPushPolicy = NormalizePipelineDNSPushPolicy(item.DNSPushPolicy)
		if item.ConfigSnapshot == nil {
			item.ConfigSnapshot = map[string]any{}
		}
		if sanitize != nil {
			item.ConfigSnapshot = sanitize(item.ConfigSnapshot)
		} else {
			item.ConfigSnapshot = clonePipelineSnapshot(item.ConfigSnapshot)
		}
		if strings.TrimSpace(item.CreatedAt) == "" {
			item.CreatedAt = now
		}
		if strings.TrimSpace(item.UpdatedAt) == "" {
			item.UpdatedAt = now
		}
	}
	if strings.TrimSpace(store.ActiveProfileID) == "" && len(store.Items) > 0 {
		store.ActiveProfileID = store.Items[0].ID
	}
	if len(store.Items) > 0 {
		found := false
		for _, item := range store.Items {
			if item.ID == store.ActiveProfileID {
				found = true
				break
			}
		}
		if !found {
			store.ActiveProfileID = store.Items[0].ID
		}
	}
	return store
}

func NormalizePipelineWorkspaceForSave(workspace PipelineWorkspace, schemaVersion string, now string, sanitize func(map[string]any) map[string]any, newTemplateID func(index int) string, newTargetID func(index int) string) PipelineWorkspace {
	now = strings.TrimSpace(now)
	if now == "" {
		now = time.Now().Format(time.RFC3339)
	}
	if strings.TrimSpace(workspace.SchemaVersion) == "" {
		workspace.SchemaVersion = schemaVersion
	}
	workspace.UpdatedAt = firstNonEmptyString(workspace.UpdatedAt, now)
	if workspace.Templates == nil {
		workspace.Templates = []PipelineTemplate{}
	}
	if workspace.Targets == nil {
		workspace.Targets = []PipelineTarget{}
	}
	legacyTargets := make([]PipelineTarget, len(workspace.Targets))
	copy(legacyTargets, workspace.Targets)
	legacyActiveTargetID := strings.TrimSpace(workspace.ActiveTargetID)
	for index := range workspace.Templates {
		item := &workspace.Templates[index]
		if strings.TrimSpace(item.ID) == "" {
			if newTemplateID != nil {
				item.ID = strings.TrimSpace(newTemplateID(index))
			}
			if strings.TrimSpace(item.ID) == "" {
				item.ID = fmt.Sprintf("pipeline-template-%d", time.Now().UnixNano()+int64(index))
			}
		}
		if strings.TrimSpace(item.Name) == "" {
			item.Name = fmt.Sprintf("流程 %d", index+1)
		}
		if strings.TrimSpace(item.EntryNodeID) == "" && len(item.Nodes) > 0 {
			item.EntryNodeID = item.Nodes[0].ID
		}
		if strings.TrimSpace(item.CreatedAt) == "" {
			item.CreatedAt = now
		}
		if strings.TrimSpace(item.UpdatedAt) == "" {
			item.UpdatedAt = now
		}
		if item.Version <= 0 {
			item.Version = 1
		}
		if item.UI == nil {
			item.UI = &PipelineTemplateUI{}
		}
		if item.UI.Viewport == nil {
			item.UI.Viewport = &PipelineViewport{X: 0, Y: 0, Zoom: 1}
		}
		if item.UI.Viewport.Zoom <= 0 {
			item.UI.Viewport.Zoom = 1
		}
		item.BoundConfigSnapshot = pipelineTemplateBoundSnapshot(item, legacyTargets, legacyActiveTargetID, sanitize)
		if item.Nodes == nil {
			item.Nodes = []PipelineNode{}
		}
		if item.Edges == nil {
			item.Edges = []PipelineEdge{}
		}
		for nodeIndex := range item.Nodes {
			node := &item.Nodes[nodeIndex]
			rawNodeType := strings.TrimSpace(node.NodeType)
			rawAction := strings.TrimSpace(node.Action)
			if strings.TrimSpace(node.ID) == "" {
				node.ID = fmt.Sprintf("%s-node-%d", item.ID, nodeIndex+1)
			}
			if strings.TrimSpace(node.Name) == "" {
				node.Name = fmt.Sprintf("步骤 %d", nodeIndex+1)
			}
			node.NodeType = normalizePipelineNodeType(node.NodeType)
			node.Action = normalizePipelineNodeAction(node.Action)
			if rawNodeType == "" {
				if actionNodeType, ok := pipelineNodeActionNodeType(node.Action); ok {
					node.NodeType = actionNodeType
				}
			}
			if strings.TrimSpace(node.Action) == "" {
				node.Action = defaultPipelineNodeAction(node.NodeType)
			}
			node.Config = normalizePipelineNodeConfig(rawAction, node.Action, node.Config)
			if node.UI == nil {
				node.UI = &PipelineNodeUI{}
			}
			if node.UI.Width <= 0 {
				node.UI.Width = 320
			}
			node.UpdatedAt = firstNonEmptyString(node.UpdatedAt, now)
		}
		for edgeIndex := range item.Edges {
			edge := &item.Edges[edgeIndex]
			if strings.TrimSpace(edge.ID) == "" {
				edge.ID = fmt.Sprintf("%s-edge-%d", item.ID, edgeIndex+1)
			}
		}
	}
	workspace.Templates = ensureDefaultPipelineTemplate(workspace.Templates, now)
	if strings.TrimSpace(workspace.ActiveTemplateID) == "" && len(workspace.Templates) > 0 {
		workspace.ActiveTemplateID = workspace.Templates[0].ID
	}
	workspace.Targets = pipelineWorkspaceCompatibilityTargets(workspace.Templates, legacyTargets, legacyActiveTargetID, now, sanitize, newTargetID)
	if len(workspace.Targets) > 0 {
		activeTargetID := compatibilityTargetIDForTemplate(workspace.Templates, workspace.Targets, workspace.ActiveTemplateID)
		workspace.ActiveTargetID = firstNonEmptyString(activeTargetID, workspace.Targets[0].ID)
	} else {
		workspace.ActiveTargetID = ""
	}
	return workspace
}

func ValidatePipelineWorkspace(workspace PipelineWorkspace) error {
	for _, template := range workspace.Templates {
		if err := ValidatePipelineTemplate(template); err != nil {
			return fmt.Errorf("template %s: %w", firstNonEmptyString(strings.TrimSpace(template.Name), strings.TrimSpace(template.ID), "unknown"), err)
		}
	}
	templateIDs := make(map[string]struct{}, len(workspace.Templates))
	for _, template := range workspace.Templates {
		templateIDs[strings.TrimSpace(template.ID)] = struct{}{}
	}
	for _, target := range workspace.Targets {
		templateID := strings.TrimSpace(target.TemplateID)
		if templateID == "" {
			continue
		}
		if _, ok := templateIDs[templateID]; !ok {
			return fmt.Errorf("target %s references unknown template_id %s", firstNonEmptyString(strings.TrimSpace(target.Name), strings.TrimSpace(target.ID), "unknown"), templateID)
		}
	}
	return nil
}

func ValidatePipelineTemplate(template PipelineTemplate) error {
	if len(template.Nodes) == 0 {
		return errors.New("missing nodes")
	}
	entryID := strings.TrimSpace(template.EntryNodeID)
	if entryID == "" {
		return errors.New("missing entry_node_id")
	}
	nodeByID := make(map[string]PipelineNode, len(template.Nodes))
	inDegree := make(map[string]int, len(template.Nodes))
	outDegree := make(map[string]int, len(template.Nodes))
	adjacency := make(map[string][]string, len(template.Nodes))
	seenNodeIDs := make(map[string]struct{}, len(template.Nodes))
	branchOutcomes := make(map[string]map[string]struct{}, len(template.Nodes))
	endCount := 0
	for _, node := range template.Nodes {
		nodeID := strings.TrimSpace(node.ID)
		if nodeID == "" {
			return errors.New("node id cannot be empty")
		}
		if _, ok := seenNodeIDs[nodeID]; ok {
			return fmt.Errorf("duplicate node id %s", nodeID)
		}
		seenNodeIDs[nodeID] = struct{}{}
		action := normalizePipelineNodeAction(node.Action)
		catalogItem, ok := pipelineNodeCatalogByAction()[action]
		if !ok {
			return fmt.Errorf("node %s uses unknown action %s", nodeID, strings.TrimSpace(node.Action))
		}
		nodeType := normalizePipelineNodeType(node.NodeType)
		if nodeType != catalogItem.NodeType {
			return fmt.Errorf("node %s action %s requires node_type %s", nodeID, action, catalogItem.NodeType)
		}
		nodeByID[nodeID] = node
		inDegree[nodeID] = 0
		outDegree[nodeID] = 0
		adjacency[nodeID] = []string{}
		if nodeType == PipelineNodeTypeBranch {
			branchOutcomes[nodeID] = make(map[string]struct{})
		}
		if nodeType == PipelineNodeTypeEnd {
			endCount++
		}
	}
	if _, ok := nodeByID[entryID]; !ok {
		return fmt.Errorf("entry node %s not found", entryID)
	}
	if endCount == 0 {
		return errors.New("missing end node")
	}
	seenEdgeIDs := make(map[string]struct{}, len(template.Edges))
	for _, edge := range template.Edges {
		edgeID := strings.TrimSpace(edge.ID)
		if edgeID != "" {
			if _, ok := seenEdgeIDs[edgeID]; ok {
				return fmt.Errorf("duplicate edge id %s", edgeID)
			}
			seenEdgeIDs[edgeID] = struct{}{}
		}
		sourceID := strings.TrimSpace(edge.SourceNode)
		targetID := strings.TrimSpace(edge.TargetNode)
		if sourceID == "" || targetID == "" {
			return errors.New("edge source_node_id and target_node_id are required")
		}
		sourceNode, ok := nodeByID[sourceID]
		if !ok {
			return fmt.Errorf("edge source %s not found", sourceID)
		}
		if _, ok := nodeByID[targetID]; !ok {
			return fmt.Errorf("edge target %s not found", targetID)
		}
		sourceNodeType := normalizePipelineNodeType(sourceNode.NodeType)
		if sourceNodeType == PipelineNodeTypeEnd {
			return fmt.Errorf("end node %s cannot have outgoing edges", sourceID)
		}
		outcome := strings.TrimSpace(edge.Outcome)
		if sourceNodeType == PipelineNodeTypeBranch {
			if outcome == "" {
				return fmt.Errorf("branch node %s edge %s is missing outcome", sourceID, edgeID)
			}
			if _, ok := branchOutcomes[sourceID][outcome]; ok {
				return fmt.Errorf("branch node %s has duplicate outcome %s", sourceID, outcome)
			}
			branchOutcomes[sourceID][outcome] = struct{}{}
			if !pipelineNodeCatalogOutcomeAllowed(normalizePipelineNodeAction(sourceNode.Action), outcome) {
				return fmt.Errorf("branch node %s uses unsupported outcome %s", sourceID, outcome)
			}
		} else if outcome != "" {
			return fmt.Errorf("node %s cannot declare outcome on outgoing edge", sourceID)
		}
		adjacency[sourceID] = append(adjacency[sourceID], targetID)
		inDegree[targetID]++
		outDegree[sourceID]++
	}
	for nodeID, node := range nodeByID {
		nodeType := normalizePipelineNodeType(node.NodeType)
		if nodeType == PipelineNodeTypeBranch && outDegree[nodeID] == 0 {
			return fmt.Errorf("branch node %s has no outgoing edge", nodeID)
		}
		if nodeType != PipelineNodeTypeBranch && nodeType != PipelineNodeTypeEnd && outDegree[nodeID] == 0 {
			return fmt.Errorf("node %s has no outgoing edge", nodeID)
		}
		if nodeType != PipelineNodeTypeBranch && outDegree[nodeID] > 1 {
			return fmt.Errorf("node %s has more than 1 outgoing edge", nodeID)
		}
	}
	reachable := map[string]bool{}
	queue := []string{entryID}
	reachable[entryID] = true
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, next := range adjacency[current] {
			if reachable[next] {
				continue
			}
			reachable[next] = true
			queue = append(queue, next)
		}
	}
	reachableEnd := 0
	for nodeID, node := range nodeByID {
		if !reachable[nodeID] {
			return fmt.Errorf("node %s is unreachable from entry", nodeID)
		}
		if normalizePipelineNodeType(node.NodeType) == PipelineNodeTypeEnd {
			reachableEnd++
		}
	}
	if reachableEnd == 0 {
		return errors.New("entry path does not reach any end node")
	}
	pending := make([]string, 0, len(inDegree))
	inDegreeCopy := make(map[string]int, len(inDegree))
	for nodeID, degree := range inDegree {
		inDegreeCopy[nodeID] = degree
		if degree == 0 {
			pending = append(pending, nodeID)
		}
	}
	visited := 0
	for len(pending) > 0 {
		current := pending[0]
		pending = pending[1:]
		visited++
		for _, next := range adjacency[current] {
			inDegreeCopy[next]--
			if inDegreeCopy[next] == 0 {
				pending = append(pending, next)
			}
		}
	}
	if visited != len(nodeByID) {
		return errors.New("graph contains a cycle")
	}
	return nil
}

func PipelineProfileStoreFromAny(value any) PipelineProfileStore {
	if value == nil {
		return PipelineProfileStore{}
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return PipelineProfileStore{}
	}
	var store PipelineProfileStore
	if err := json.Unmarshal(raw, &store); err != nil {
		return PipelineProfileStore{}
	}
	return store
}

func PipelineProfilesFromAny(value any) []PipelineProfile {
	if value == nil {
		return nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	var profiles []PipelineProfile
	if err := json.Unmarshal(raw, &profiles); err != nil {
		return nil
	}
	return profiles
}

func PipelineWorkspaceFromAny(value any) PipelineWorkspace {
	if value == nil {
		return PipelineWorkspace{}
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return PipelineWorkspace{}
	}
	var workspace PipelineWorkspace
	if err := json.Unmarshal(raw, &workspace); err != nil {
		return PipelineWorkspace{}
	}
	return workspace
}

func PipelineTemplatesFromAny(value any) []PipelineTemplate {
	if value == nil {
		return nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	var templates []PipelineTemplate
	if err := json.Unmarshal(raw, &templates); err != nil {
		return nil
	}
	return templates
}

func PipelineTargetsFromAny(value any) []PipelineTarget {
	if value == nil {
		return nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	var targets []PipelineTarget
	if err := json.Unmarshal(raw, &targets); err != nil {
		return nil
	}
	return targets
}

func NormalizePipelineDNSPushPolicy(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case PipelineDNSPushPolicySkip, "manual", "disabled", "none":
		return PipelineDNSPushPolicySkip
	default:
		return PipelineDNSPushPolicyAuto
	}
}

func PipelineDNSPushEnabled(policy string) bool {
	return NormalizePipelineDNSPushPolicy(policy) == PipelineDNSPushPolicyAuto
}

func normalizePipelineNodeType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case PipelineNodeTypeSource:
		return PipelineNodeTypeSource
	case PipelineNodeTypeFilter:
		return PipelineNodeTypeFilter
	case PipelineNodeTypeBranch:
		return PipelineNodeTypeBranch
	case PipelineNodeTypeDeliver:
		return PipelineNodeTypeDeliver
	case PipelineNodeTypeRecovery:
		return PipelineNodeTypeRecovery
	case PipelineNodeTypeEnd:
		return PipelineNodeTypeEnd
	default:
		return PipelineNodeTypeProbe
	}
}

func defaultPipelineNodeAction(nodeType string) string {
	switch normalizePipelineNodeType(nodeType) {
	case PipelineNodeTypeSource:
		return PipelineNodeActionSelectSources
	case PipelineNodeTypeFilter:
		return PipelineNodeActionFilterResults
	case PipelineNodeTypeBranch:
		return PipelineNodeActionBranchHasResults
	case PipelineNodeTypeDeliver:
		return PipelineNodeActionDeliverDNS
	case PipelineNodeTypeRecovery:
		return PipelineNodeActionRecoveryMark
	case PipelineNodeTypeEnd:
		return PipelineNodeActionEnd
	default:
		return PipelineNodeActionRunProbe
	}
}

func normalizePipelineNodeAction(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch normalized {
	case "":
		return ""
	case "filter_candidates", PipelineNodeActionFilterResults:
		return PipelineNodeActionFilterResults
	case "has_results", PipelineNodeActionBranchHasResults:
		return PipelineNodeActionBranchHasResults
	case "dns_push", PipelineNodeActionDeliverDNS:
		return PipelineNodeActionDeliverDNS
	case "github_export", PipelineNodeActionDeliverGitHub:
		return PipelineNodeActionDeliverGitHub
	case "mark_manual_review", PipelineNodeActionRecoveryMark:
		return PipelineNodeActionRecoveryMark
	case "completed", "manual_review", PipelineNodeActionEnd:
		return PipelineNodeActionEnd
	case "source_group", "select_source", PipelineNodeActionSelectSources:
		return PipelineNodeActionSelectSources
	case PipelineNodeActionCheckOutput:
		return PipelineNodeActionCheckOutput
	case PipelineNodeActionRunProbe:
		return PipelineNodeActionRunProbe
	default:
		return normalized
	}
}

func pipelineNodeActionNodeType(action string) (string, bool) {
	catalogItem, ok := pipelineNodeCatalogByAction()[normalizePipelineNodeAction(action)]
	if !ok {
		return "", false
	}
	return catalogItem.NodeType, true
}

func normalizePipelineNodeConfig(rawAction string, action string, config map[string]any) map[string]any {
	normalized := clonePipelineSnapshot(config)
	catalogItem, ok := pipelineNodeCatalogByAction()[normalizePipelineNodeAction(action)]
	if ok {
		for key, value := range catalogItem.DefaultConfig {
			if _, exists := normalized[key]; !exists {
				normalized[key] = value
			}
		}
	}
	switch strings.ToLower(strings.TrimSpace(rawAction)) {
	case "manual_review":
		if _, ok := normalized["status"]; !ok {
			normalized["status"] = "manual_review"
		}
		if _, ok := normalized["message"]; !ok {
			normalized["message"] = "需要人工复核。"
		}
	case "completed":
		if _, ok := normalized["status"]; !ok {
			normalized["status"] = "completed"
		}
	}
	if normalized == nil {
		return map[string]any{}
	}
	return normalized
}

func pipelineNodeCatalogOutcomeAllowed(action string, outcome string) bool {
	action = normalizePipelineNodeAction(action)
	outcome = strings.TrimSpace(outcome)
	if outcome == "" {
		return false
	}
	catalogItem, ok := pipelineNodeCatalogByAction()[action]
	if !ok || len(catalogItem.Outcomes) == 0 {
		return true
	}
	for _, item := range catalogItem.Outcomes {
		if strings.TrimSpace(item.Value) == outcome {
			return true
		}
	}
	return false
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func clonePipelineSnapshot(snapshot map[string]any) map[string]any {
	if snapshot == nil {
		return map[string]any{}
	}
	raw, err := json.Marshal(snapshot)
	if err != nil {
		return map[string]any{}
	}
	var cloned map[string]any
	if err := json.Unmarshal(raw, &cloned); err != nil {
		return map[string]any{}
	}
	return cloned
}

func preferredPipelineProfile(store PipelineProfileStore) *PipelineProfile {
	if len(store.Items) == 0 {
		return nil
	}
	activeProfileID := strings.TrimSpace(store.ActiveProfileID)
	if activeProfileID != "" {
		for index := range store.Items {
			if strings.TrimSpace(store.Items[index].ID) == activeProfileID {
				return &store.Items[index]
			}
		}
	}
	for index := range store.Items {
		if store.Items[index].Enabled {
			return &store.Items[index]
		}
	}
	return &store.Items[0]
}

func pipelineTemplateBoundSnapshot(template *PipelineTemplate, legacyTargets []PipelineTarget, activeTargetID string, sanitize func(map[string]any) map[string]any) map[string]any {
	snapshot := clonePipelineSnapshot(template.BoundConfigSnapshot)
	if len(snapshot) == 0 {
		if target := preferredTargetForTemplate(strings.TrimSpace(template.ID), legacyTargets, activeTargetID); target != nil {
			snapshot = clonePipelineSnapshot(target.ConfigSnapshot)
		}
	}
	if sanitize != nil {
		return sanitize(snapshot)
	}
	return snapshot
}

func preferredTargetForTemplate(templateID string, targets []PipelineTarget, activeTargetID string) *PipelineTarget {
	templateID = strings.TrimSpace(templateID)
	activeTargetID = strings.TrimSpace(activeTargetID)
	var firstEnabled *PipelineTarget
	var first *PipelineTarget
	for index := range targets {
		target := &targets[index]
		if templateID != "" && strings.TrimSpace(target.TemplateID) != templateID {
			continue
		}
		if activeTargetID != "" && strings.TrimSpace(target.ID) == activeTargetID {
			return target
		}
		if first == nil {
			first = target
		}
		if firstEnabled == nil && target.Enabled {
			firstEnabled = target
		}
	}
	if firstEnabled != nil {
		return firstEnabled
	}
	return first
}

func pipelineWorkspaceCompatibilityTargets(templates []PipelineTemplate, legacyTargets []PipelineTarget, activeTargetID string, now string, sanitize func(map[string]any) map[string]any, newTargetID func(index int) string) []PipelineTarget {
	if len(templates) == 0 {
		return []PipelineTarget{}
	}
	targets := make([]PipelineTarget, 0, len(templates))
	for index := range templates {
		template := templates[index]
		legacy := preferredTargetForTemplate(strings.TrimSpace(template.ID), legacyTargets, activeTargetID)
		targetID := ""
		if legacy != nil {
			targetID = strings.TrimSpace(legacy.ID)
		}
		if targetID == "" && newTargetID != nil {
			targetID = strings.TrimSpace(newTargetID(index))
		}
		if targetID == "" {
			templateID := strings.TrimSpace(template.ID)
			if templateID == "" {
				templateID = fmt.Sprintf("template-%d", index+1)
			}
			targetID = fmt.Sprintf("%s-target", templateID)
		}
		snapshot := clonePipelineSnapshot(template.BoundConfigSnapshot)
		if sanitize != nil {
			snapshot = sanitize(snapshot)
		}
		domain := pipelineDomainFromSnapshot(snapshot)
		target := PipelineTarget{
			ConfigSnapshot: snapshot,
			CreatedAt: firstNonEmptyString(func() string {
				if legacy != nil {
					return legacy.CreatedAt
				}
				return ""
			}(), template.CreatedAt, now),
			DNSPushPolicy: NormalizePipelineDNSPushPolicy(func() string {
				if legacy != nil {
					return legacy.DNSPushPolicy
				}
				return ""
			}()),
			Domain: firstNonEmptyString(func() string {
				if legacy != nil && strings.TrimSpace(legacy.Domain) != "" {
					return legacy.Domain
				}
				return ""
			}(), domain),
			Enabled: true,
			ID:      targetID,
			Name: firstNonEmptyString(func() string {
				if legacy != nil {
					return legacy.Name
				}
				return ""
			}(), strings.TrimSpace(template.Name), fmt.Sprintf("工作流 %d", index+1)),
			Region: firstNonEmptyString(func() string {
				if legacy != nil {
					return legacy.Region
				}
				return ""
			}(), "当前配置"),
			Tags:       []string{},
			TemplateID: strings.TrimSpace(template.ID),
			UpdatedAt: firstNonEmptyString(template.UpdatedAt, func() string {
				if legacy != nil {
					return legacy.UpdatedAt
				}
				return ""
			}(), now),
		}
		if legacy != nil {
			target.Tags = normalizeStringSlice(append([]string{}, legacy.Tags...))
		}
		targets = append(targets, target)
	}
	return targets
}

func compatibilityTargetIDForTemplate(templates []PipelineTemplate, targets []PipelineTarget, activeTemplateID string) string {
	activeTemplateID = strings.TrimSpace(activeTemplateID)
	if activeTemplateID == "" && len(templates) > 0 {
		activeTemplateID = strings.TrimSpace(templates[0].ID)
	}
	for _, target := range targets {
		if strings.TrimSpace(target.TemplateID) == activeTemplateID {
			return strings.TrimSpace(target.ID)
		}
	}
	return ""
}

func pipelineDomainFromSnapshot(snapshot map[string]any) string {
	cloudflare, ok := snapshot["cloudflare"].(map[string]any)
	if !ok {
		return ""
	}
	return strings.TrimSpace(stringValue(firstNonNil(cloudflare["record_name"], cloudflare["recordName"]), ""))
}

func normalizeStringSlice(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	if len(result) == 0 {
		return nil
	}
	return result
}
