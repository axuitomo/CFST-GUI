package appcore

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"
)

const (
	DefaultPipelineProfilesSchemaVersion  = "cfst-gui-pipeline-profiles-v1"
	DefaultPipelineWorkspaceSchemaVersion = "cfst-gui-pipeline-workspace-v1"
	DefaultPipelineProfileID              = "pipeline-profile-default"
	DefaultPipelineTemplateID             = "pipeline-template-default"
	AdvancedUploadPipelineTemplateID      = "pipeline-template-advanced-upload"
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
	PipelineNodeActionFilterSources       = "filter_sources"
	PipelineNodeActionProbeTCP            = "probe_tcp"
	PipelineNodeActionProbeTrace          = "probe_trace"
	PipelineNodeActionProbeDownload       = "probe_download"
	PipelineNodeActionFilterResults       = "filter_results"
	PipelineNodeActionBranchHasResults    = "branch_has_results"
	PipelineNodeActionDeliverDNS          = "deliver_dns"
	PipelineNodeActionDeliverGitHub       = "deliver_github"
	PipelineNodeActionRecoveryMark        = "recovery_mark"
	PipelineNodeActionCheckOutput         = "check_output"
	PipelineNodeActionEnd                 = "end"
	legacyPipelineNodeActionRunProbe      = "run_probe"
	PipelineSourceSelectionEnabled        = "enabled"
	PipelineSourceSelectionCustom         = "custom"
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
	AllowDNSPush         *bool `json:"allow_dns_push,omitempty"`
	AllowGitHubExport    *bool `json:"allow_github_export,omitempty"`
	DisablePostProbePush bool  `json:"disable_post_probe_push,omitempty"`
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

func pipelineProbeFullModeDefaultConfig() map[string]any {
	return map[string]any{
		"concurrency_stage1":                200,
		"concurrency_stage2":                30,
		"concurrency_stage3":                1,
		"disable_download":                  false,
		"download_buffer_kb":                256,
		"download_count":                    10,
		"download_get_concurrency":          4,
		"download_http_protocol":            "auto",
		"download_speed_metric":             "average",
		"download_speed_sample_interval_ms": 500,
		"download_time_seconds":             4,
		"download_warmup_seconds":           1,
		"httping_cf_colo":                   "",
		"httping_cf_colo_mode":              "allow",
		"httping_status_code":               0,
		"max_loss_rate":                     0.15,
		"max_tcp_latency_ms":                nil,
		"max_trace_latency_ms":              nil,
		"min_delay_ms":                      0,
		"min_download_mbps":                 0,
		"ping_times":                        4,
		"port_policy":                       "source_override_global",
		"print_num":                         0,
		"source_colo_filter_phase":          "precheck",
		"stage3_limit":                      10,
		"strategy":                          "full",
		"tcp_port":                          443,
		"timeout_stage1_ms":                 1000,
		"timeout_stage2_ms":                 1000,
		"timeout_stage3_ms":                 10000,
		"trace_colo_mode":                   "standard",
		"trace_url":                         "",
		"url":                               "https://speedtest.xyz9923.dpdns.org/500m",
	}
}

func pipelineProbeFullModeFormSchema(primaryStage string) []PipelineNodeCatalogField {
	tcpFields := []PipelineNodeCatalogField{
		{
			DefaultValue: 443,
			FieldType:    "number",
			Group:        "第一阶段 TCP",
			Key:          "tcp_port",
			Label:        "全局测速端口",
			Max:          floatPtr(65535),
			Min:          floatPtr(1),
			Step:         floatPtr(1),
		},
		{
			DefaultValue: "source_override_global",
			FieldType:    "select",
			Group:        "第一阶段 TCP",
			HelpText:     "输入源声明端口时优先使用，否则回退到固定端口。",
			Key:          "port_policy",
			Label:        "端口策略",
			Options: []PipelineNodeCatalogFieldOption{
				{Label: "输入源端口优先", Value: "source_override_global"},
				{Label: "固定全局端口", Value: "fixed_global"},
			},
		},
		{
			DefaultValue: 200,
			FieldType:    "number",
			Group:        "第一阶段 TCP",
			Key:          "concurrency_stage1",
			Label:        "TCP 并发线程",
			Max:          floatPtr(1000),
			Min:          floatPtr(1),
			Step:         floatPtr(1),
		},
		{
			DefaultValue: 4,
			FieldType:    "number",
			Group:        "第一阶段 TCP",
			Key:          "ping_times",
			Label:        "TCP 发包次数",
			Min:          floatPtr(2),
			Step:         floatPtr(1),
		},
		{
			FieldType: "number",
			Group:     "第一阶段 TCP",
			Key:       "max_tcp_latency_ms",
			Label:     "TCP 延迟上限(ms)",
			Min:       floatPtr(1),
			Step:      floatPtr(1),
		},
		{
			DefaultValue: 0,
			FieldType:    "number",
			Group:        "第一阶段 TCP",
			Key:          "min_delay_ms",
			Label:        "TCP 延迟下限(ms)",
			Min:          floatPtr(0),
			Step:         floatPtr(1),
		},
		{
			DefaultValue: 0.15,
			FieldType:    "number",
			Group:        "第一阶段 TCP",
			Key:          "max_loss_rate",
			Label:        "TCP 丢包率上限",
			Max:          floatPtr(1),
			Min:          floatPtr(0),
			Step:         floatPtr(0.01),
		},
		{
			DefaultValue: 1000,
			FieldType:    "number",
			Group:        "第一阶段 TCP",
			Key:          "timeout_stage1_ms",
			Label:        "阶段 1 TCP 超时(ms)",
			Min:          floatPtr(1),
			Step:         floatPtr(1),
		},
	}
	traceFields := []PipelineNodeCatalogField{
		{
			DefaultValue: "",
			FieldType:    "text",
			Group:        "第二阶段 追踪/COLO",
			HelpText:     "留空时从文件测速 URL 派生 /cdn-cgi/trace。",
			Key:          "trace_url",
			Label:        "追踪 URL",
			Placeholder:  "https://speed.cloudflare.com/cdn-cgi/trace",
		},
		{
			DefaultValue: "standard",
			FieldType:    "select",
			Group:        "第二阶段 追踪/COLO",
			Key:          "trace_colo_mode",
			Label:        "第二阶段 COLO 获取模式",
			Options: []PipelineNodeCatalogFieldOption{
				{Label: "标准", Value: "standard"},
				{Label: "追踪 URL", Value: "trace_url"},
			},
		},
		{
			DefaultValue: "precheck",
			FieldType:    "select",
			Group:        "第二阶段 追踪/COLO",
			HelpText:     "国家/COLO 筛选词复用 Cloudflare COLO 字典派生链路。",
			Key:          "source_colo_filter_phase",
			Label:        "输入源 COLO 筛选阶段",
			Options: []PipelineNodeCatalogFieldOption{
				{Label: "cloudflare-colos", Value: "precheck"},
				{Label: "第二阶段起效", Value: "stage2"},
			},
		},
		{
			DefaultValue: 30,
			FieldType:    "number",
			Group:        "第二阶段 追踪/COLO",
			Key:          "concurrency_stage2",
			Label:        "追踪并发线程",
			Max:          floatPtr(30),
			Min:          floatPtr(1),
			Step:         floatPtr(1),
		},
		{
			DefaultValue: 1000,
			FieldType:    "number",
			Group:        "第二阶段 追踪/COLO",
			Key:          "timeout_stage2_ms",
			Label:        "追踪超时(ms)",
			Min:          floatPtr(1),
			Step:         floatPtr(1),
		},
		{
			DefaultValue: 0,
			FieldType:    "number",
			Group:        "第二阶段 追踪/COLO",
			HelpText:     "0 表示不限制；100-599 表示启用状态码筛选。",
			Key:          "httping_status_code",
			Label:        "追踪有效状态码",
			Max:          floatPtr(599),
			Min:          floatPtr(0),
			Step:         floatPtr(1),
		},
		{
			FieldType: "number",
			Group:     "第二阶段 追踪/COLO",
			Key:       "max_trace_latency_ms",
			Label:     "追踪延迟上限(ms)",
			Min:       floatPtr(1),
			Step:      floatPtr(1),
		},
		{
			DefaultValue: "",
			FieldType:    "text",
			Group:        "第二阶段 追踪/COLO",
			HelpText:     "空列表不限制；可填写 HKG,NRT,LAX 等 COLO。",
			Key:          "httping_cf_colo",
			Label:        "最终国家/COLO 筛选词",
			Placeholder:  "HKG,NRT,LAX",
		},
		{
			DefaultValue: "allow",
			FieldType:    "select",
			Group:        "第二阶段 追踪/COLO",
			Key:          "httping_cf_colo_mode",
			Label:        "最终筛选方式",
			Options: []PipelineNodeCatalogFieldOption{
				{Label: "白名单", Value: "allow"},
				{Label: "黑名单", Value: "deny"},
			},
		},
	}
	downloadFields := []PipelineNodeCatalogField{
		{
			DefaultValue: "https://speedtest.xyz9923.dpdns.org/500m",
			FieldType:    "text",
			Group:        "第三阶段 下载",
			HelpText:     "文件测速阶段只访问该文件 URL；不要填写 /cdn-cgi/trace。",
			Key:          "url",
			Label:        "文件测速 URL",
		},
		{
			DefaultValue: 10,
			FieldType:    "number",
			Group:        "第三阶段 下载",
			HelpText:     "限制完整模式进入文件测速的候选数。",
			Key:          "stage3_limit",
			Label:        "测速上限",
			Min:          floatPtr(1),
			Step:         floatPtr(1),
		},
		{
			DefaultValue: 10,
			FieldType:    "number",
			Group:        "第三阶段 下载",
			Key:          "download_count",
			Label:        "下载测速数量",
			Min:          floatPtr(1),
			Step:         floatPtr(1),
		},
		{
			DefaultValue: 0,
			FieldType:    "number",
			Group:        "第三阶段 下载",
			HelpText:     "0 不限制；正数按速度指标输出前 N 条。",
			Key:          "print_num",
			Label:        "结果显示数量",
			Min:          floatPtr(0),
			Step:         floatPtr(1),
		},
		{
			DefaultValue: 1,
			FieldType:    "number",
			Group:        "第三阶段 下载",
			HelpText:     "文件测速阶段保持串行时维持 1。",
			Key:          "concurrency_stage3",
			Label:        "下载阶段并发",
			Min:          floatPtr(1),
			Step:         floatPtr(1),
		},
		{
			DefaultValue: 4,
			FieldType:    "number",
			Group:        "第三阶段 下载",
			Key:          "download_get_concurrency",
			Label:        "单 IP GET 分片并发",
			Max:          floatPtr(32),
			Min:          floatPtr(1),
			Step:         floatPtr(1),
		},
		{
			DefaultValue: 4,
			FieldType:    "number",
			Group:        "第三阶段 下载",
			Key:          "download_time_seconds",
			Label:        "单 IP 下载测速时间(秒)",
			Min:          floatPtr(1),
			Step:         floatPtr(1),
		},
		{
			DefaultValue: 10000,
			FieldType:    "number",
			Group:        "第三阶段 下载",
			Key:          "timeout_stage3_ms",
			Label:        "阶段 3 下载超时(ms)",
			Min:          floatPtr(1),
			Step:         floatPtr(1),
		},
		{
			DefaultValue: 1,
			FieldType:    "number",
			Group:        "第三阶段 下载",
			Key:          "download_warmup_seconds",
			Label:        "下载预热时间(秒)",
			Min:          floatPtr(0),
			Step:         floatPtr(1),
		},
		{
			DefaultValue: 500,
			FieldType:    "number",
			Group:        "第三阶段 下载",
			Key:          "download_speed_sample_interval_ms",
			Label:        "下载测速采样间隔(ms)",
			Min:          floatPtr(1),
			Step:         floatPtr(100),
		},
		{
			DefaultValue: 256,
			FieldType:    "number",
			Group:        "第三阶段 下载",
			Key:          "download_buffer_kb",
			Label:        "下载缓冲(KiB)",
			Max:          floatPtr(4096),
			Min:          floatPtr(64),
			Step:         floatPtr(64),
		},
		{
			DefaultValue: "auto",
			FieldType:    "select",
			Group:        "第三阶段 下载",
			Key:          "download_http_protocol",
			Label:        "下载 HTTP 协议",
			Options: []PipelineNodeCatalogFieldOption{
				{Label: "Auto", Value: "auto"},
				{Label: "H1.1", Value: "h1"},
				{Label: "H2", Value: "h2"},
				{Label: "H3", Value: "h3"},
			},
		},
		{
			DefaultValue: "average",
			FieldType:    "select",
			Group:        "第三阶段 下载",
			Key:          "download_speed_metric",
			Label:        "下载速率依据",
			Options: []PipelineNodeCatalogFieldOption{
				{Label: "平均速率", Value: "average"},
				{Label: "最高速率", Value: "max"},
			},
		},
		{
			DefaultValue: 0,
			FieldType:    "number",
			Group:        "第三阶段 下载",
			Key:          "min_download_mbps",
			Label:        "最低下载速度(MB/s)",
			Min:          floatPtr(0),
			Step:         floatPtr(0.1),
		},
	}
	switch primaryStage {
	case PipelineNodeActionProbeTrace:
		return traceFields
	case PipelineNodeActionProbeDownload:
		return downloadFields
	default:
		return tcpFields
	}
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
				"source_ids":        []any{},
				"source_profile_id": "",
				"source_selection":  PipelineSourceSelectionEnabled,
			},
			Description: "从当前绑定配置或指定输入组档案中选择输入源，作为后续测速的输入组。",
			DisplayName: "输入源组",
			FormSchema: []PipelineNodeCatalogField{
				{
					DefaultValue: "",
					FieldType:    "select",
					Group:        "输入组",
					HelpText:     "留空使用当前工作流绑定配置；前端会把已有输入组档案作为选项注入。",
					Key:          "source_profile_id",
					Label:        "输入组档案",
					Options: []PipelineNodeCatalogFieldOption{
						{Label: "当前绑定配置", Value: ""},
					},
				},
				{
					DefaultValue: PipelineSourceSelectionEnabled,
					FieldType:    "select",
					Group:        "输入组",
					HelpText:     "全部启用表示使用所选输入组中 enabled=true 的输入源；自定义勾选只使用 source_ids。",
					Key:          "source_selection",
					Label:        "选择方式",
					Options: []PipelineNodeCatalogFieldOption{
						{Label: "全部启用输入源", Value: PipelineSourceSelectionEnabled},
						{Label: "自定义勾选", Value: PipelineSourceSelectionCustom},
					},
				},
			},
			NodeType: PipelineNodeTypeSource,
		},
		{
			Action: PipelineNodeActionFilterSources,
			DefaultConfig: map[string]any{
				"source_colo_filter":      "",
				"source_colo_filter_mode": "allow",
				"source_ip_limit":         500,
				"source_ip_mode":          "traverse",
			},
			Description: "对上游输入源组批量覆盖 IP 上限、抽样模式和国家/COLO 筛选词，再输出新的输入源组。",
			DisplayName: "输入源筛选",
			FormSchema: []PipelineNodeCatalogField{
				{
					DefaultValue: 500,
					FieldType:    "number",
					Group:        "输入源筛选",
					HelpText:     "批量覆盖每个输入源的候选 IP 上限；实际语义沿用输入源处理链路。",
					Key:          "source_ip_limit",
					Label:        "总测试 IP 上限",
					Min:          floatPtr(1),
					Step:         floatPtr(1),
				},
				{
					DefaultValue: "traverse",
					FieldType:    "select",
					Group:        "输入源筛选",
					HelpText:     "遍历直接读取候选；MCIS 抽样复用现有输入源 MCIS 处理。",
					Key:          "source_ip_mode",
					Label:        "IP 获取模式",
					Options: []PipelineNodeCatalogFieldOption{
						{Label: "遍历", Value: "traverse"},
						{Label: "MCIS 抽样", Value: "mcis"},
					},
				},
				{
					DefaultValue: "",
					FieldType:    "textarea",
					Group:        "国家/COLO 筛选",
					HelpText:     "复用现有 COLO 词典筛选链路；国家筛选需依赖 Cloudflare COLO 字典派生。",
					Key:          "source_colo_filter",
					Label:        "国家/COLO 筛选词",
					Placeholder:  "例如 HKG,SJC 或可由 COLO 字典派生的国家词",
					Rows:         3,
				},
				{
					DefaultValue: "allow",
					FieldType:    "select",
					Group:        "国家/COLO 筛选",
					Key:          "source_colo_filter_mode",
					Label:        "筛选方式",
					Options: []PipelineNodeCatalogFieldOption{
						{Label: "白名单", Value: "allow"},
						{Label: "黑名单", Value: "deny"},
					},
				},
			},
			NodeType: PipelineNodeTypeSource,
		},
		{
			Action:        PipelineNodeActionProbeTCP,
			DefaultConfig: pipelineProbeFullModeDefaultConfig(),
			Description:   "第一阶段：执行 TCP 延迟测速，输出可继续追踪的候选节点。",
			DisplayName:   "TCP 延迟测速",
			FormSchema:    pipelineProbeFullModeFormSchema(PipelineNodeActionProbeTCP),
			NodeType:      PipelineNodeTypeProbe,
		},
		{
			Action:        PipelineNodeActionProbeTrace,
			DefaultConfig: pipelineProbeFullModeDefaultConfig(),
			Description:   "第二阶段：复用现有追踪/COLO 检查链路，输出可下载测速的候选节点。",
			DisplayName:   "追踪测试",
			FormSchema:    pipelineProbeFullModeFormSchema(PipelineNodeActionProbeTrace),
			NodeType:      PipelineNodeTypeProbe,
		},
		{
			Action:        PipelineNodeActionProbeDownload,
			DefaultConfig: pipelineProbeFullModeDefaultConfig(),
			Description:   "第三阶段：执行下载测速，按速度指标排序并产出最终 probe_results。",
			DisplayName:   "下载测速",
			FormSchema:    pipelineProbeFullModeFormSchema(PipelineNodeActionProbeDownload),
			NodeType:      PipelineNodeTypeProbe,
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
					HelpText:     "大于 0 时，只保留排序后的前 N 条结果继续向下游传递，并影响所有后续投递节点。",
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
					HelpText:     "只限制本 DNS 推送节点；留 0 时沿用上传配置或上游筛选结果。",
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
						{Label: "ALL (A + AAAA)", Value: "ALL"},
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
			Action: PipelineNodeActionDeliverGitHub,
			DefaultConfig: map[string]any{
				"source": "filtered_rows",
			},
			Description: "把当前筛选结果导出到 GitHub。",
			DisplayName: "GitHub 导出",
			FormSchema: []PipelineNodeCatalogField{
				{
					DefaultValue: "filtered_rows",
					FieldType:    "select",
					Group:        "数据来源",
					Key:          "source",
					Label:        "导出输入",
					Options: []PipelineNodeCatalogFieldOption{
						{Label: "筛选结果", Value: "filtered_rows"},
						{Label: "测速结果", Value: "probe_results"},
					},
				},
				{
					DefaultValue: 0,
					FieldType:    "number",
					Group:        "导出行为",
					HelpText:     "只限制本 GitHub 导出节点；留 0 时沿用上传配置或上游筛选结果。",
					Key:          "top_n",
					Label:        "导出前 N 条",
					Min:          floatPtr(0),
					Step:         floatPtr(1),
				},
			},
			NodeType: PipelineNodeTypeDeliver,
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
				"status":            "passed",
				"top_n":             0,
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
					DefaultValue: "passed",
					FieldType:    "select",
					Group:        "结果检查",
					Key:          "status",
					Label:        "结果状态",
					Options: []PipelineNodeCatalogFieldOption{
						{Label: "仅成功结果", Value: "passed"},
						{Label: "全部结果", Value: "all"},
					},
				},
				{
					DefaultValue: 0,
					FieldType:    "number",
					Group:        "结果检查",
					HelpText:     "大于 0 时仅检查排序后的前 N 条，并影响补写 CSV 的输入。",
					Key:          "top_n",
					Label:        "检查前 N 条",
					Min:          floatPtr(0),
					Step:         floatPtr(1),
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
			NodeType: PipelineNodeTypeDeliver,
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
	template := AdvancedUploadPipelineTemplate(now)
	template.ID = DefaultPipelineTemplateID
	template.Name = "默认流程"
	template.Description = "默认流程：筛选结果后，有结果自动 DNS 推送并导出 GitHub；无结果进入人工复核。"
	return template
}

func AdvancedUploadPipelineTemplate(now string) PipelineTemplate {
	now = strings.TrimSpace(now)
	if now == "" {
		now = time.Now().Format(time.RFC3339)
	}
	nodes := []PipelineNode{
		{
			Action: PipelineNodeActionSelectSources,
			Config: map[string]any{
				"source_ids":        []any{},
				"source_profile_id": "",
				"source_selection":  PipelineSourceSelectionEnabled,
			},
			ID:        "advanced-source-group",
			Name:      "输入源组",
			NodeType:  PipelineNodeTypeSource,
			UI:        &PipelineNodeUI{Position: &PipelineCanvasPosition{X: 60, Y: 160}, Width: 320},
			UpdatedAt: now,
		},
		{
			Action: PipelineNodeActionFilterSources,
			Config: map[string]any{
				"source_colo_filter":      "",
				"source_colo_filter_mode": "allow",
				"source_ip_limit":         500,
				"source_ip_mode":          "traverse",
			},
			ID:        "advanced-source-filter",
			Name:      "输入源筛选",
			NodeType:  PipelineNodeTypeSource,
			UI:        &PipelineNodeUI{Position: &PipelineCanvasPosition{X: 420, Y: 160}, Width: 320},
			UpdatedAt: now,
		},
		{
			Action:    PipelineNodeActionProbeTCP,
			Config:    pipelineProbeFullModeDefaultConfig(),
			ID:        "advanced-probe-tcp",
			Name:      "TCP 延迟测速",
			NodeType:  PipelineNodeTypeProbe,
			UI:        &PipelineNodeUI{Position: &PipelineCanvasPosition{X: 780, Y: 160}, Width: 320},
			UpdatedAt: now,
		},
		{
			Action:    PipelineNodeActionProbeTrace,
			Config:    pipelineProbeFullModeDefaultConfig(),
			ID:        "advanced-probe-trace",
			Name:      "追踪测试",
			NodeType:  PipelineNodeTypeProbe,
			UI:        &PipelineNodeUI{Position: &PipelineCanvasPosition{X: 1140, Y: 160}, Width: 320},
			UpdatedAt: now,
		},
		{
			Action:    PipelineNodeActionProbeDownload,
			Config:    pipelineProbeFullModeDefaultConfig(),
			ID:        "advanced-probe-download",
			Name:      "下载测速",
			NodeType:  PipelineNodeTypeProbe,
			UI:        &PipelineNodeUI{Position: &PipelineCanvasPosition{X: 1500, Y: 160}, Width: 320},
			UpdatedAt: now,
		},
		{
			Action: PipelineNodeActionFilterResults,
			Config: map[string]any{
				"source": "probe_results",
				"status": "passed",
			},
			ID:        "advanced-filter",
			Name:      "结果筛选",
			NodeType:  PipelineNodeTypeFilter,
			UI:        &PipelineNodeUI{Position: &PipelineCanvasPosition{X: 1860, Y: 160}, Width: 320},
			UpdatedAt: now,
		},
		{
			Action: PipelineNodeActionBranchHasResults,
			Config: map[string]any{
				"source": "filtered_rows",
			},
			ID:        "advanced-branch-results",
			Name:      "结果检查",
			NodeType:  PipelineNodeTypeBranch,
			UI:        &PipelineNodeUI{Position: &PipelineCanvasPosition{X: 2220, Y: 160}, Width: 320},
			UpdatedAt: now,
		},
		{
			Action: PipelineNodeActionDeliverDNS,
			Config: map[string]any{
				"source": "filtered_rows",
			},
			ID:        "advanced-deliver-dns",
			Name:      "DNS 推送",
			NodeType:  PipelineNodeTypeDeliver,
			UI:        &PipelineNodeUI{Position: &PipelineCanvasPosition{X: 2580, Y: 60}, Width: 320},
			UpdatedAt: now,
		},
		{
			Action: PipelineNodeActionDeliverGitHub,
			Config: map[string]any{
				"source": "filtered_rows",
			},
			ID:        "advanced-deliver-github",
			Name:      "GitHub 导出",
			NodeType:  PipelineNodeTypeDeliver,
			UI:        &PipelineNodeUI{Position: &PipelineCanvasPosition{X: 2940, Y: 60}, Width: 320},
			UpdatedAt: now,
		},
		{
			Action: PipelineNodeActionEnd,
			Config: map[string]any{
				"message": "上传流程已完成。",
				"status":  "completed",
			},
			ID:        "advanced-end-completed",
			Name:      "结束",
			NodeType:  PipelineNodeTypeEnd,
			UI:        &PipelineNodeUI{Position: &PipelineCanvasPosition{X: 3300, Y: 60}, Width: 320},
			UpdatedAt: now,
		},
		{
			Action: PipelineNodeActionRecoveryMark,
			Config: map[string]any{
				"message": "筛选后没有可投递结果，需要人工复核。",
				"status":  "manual_review",
			},
			ID:        "advanced-recovery-empty",
			Name:      "人工复核标记",
			NodeType:  PipelineNodeTypeRecovery,
			UI:        &PipelineNodeUI{Position: &PipelineCanvasPosition{X: 2580, Y: 300}, Width: 320},
			UpdatedAt: now,
		},
		{
			Action: PipelineNodeActionEnd,
			Config: map[string]any{
				"message": "筛选后没有可投递结果，已进入人工复核。",
				"status":  "manual_review",
			},
			ID:        "advanced-end-manual-review",
			Name:      "结束（人工复核）",
			NodeType:  PipelineNodeTypeEnd,
			UI:        &PipelineNodeUI{Position: &PipelineCanvasPosition{X: 2940, Y: 300}, Width: 320},
			UpdatedAt: now,
		},
	}
	edges := []PipelineEdge{
		{ID: "advanced-edge-source-filter", SourceNode: "advanced-source-group", TargetNode: "advanced-source-filter"},
		{ID: "advanced-edge-filter-tcp", SourceNode: "advanced-source-filter", TargetNode: "advanced-probe-tcp"},
		{ID: "advanced-edge-tcp-trace", SourceNode: "advanced-probe-tcp", TargetNode: "advanced-probe-trace"},
		{ID: "advanced-edge-trace-download", SourceNode: "advanced-probe-trace", TargetNode: "advanced-probe-download"},
		{ID: "advanced-edge-download-filter", SourceNode: "advanced-probe-download", TargetNode: "advanced-filter"},
		{ID: "advanced-edge-filter-branch", SourceNode: "advanced-filter", TargetNode: "advanced-branch-results"},
		{ID: "advanced-edge-branch-dns", SourceNode: "advanced-branch-results", TargetNode: "advanced-deliver-dns", Outcome: "true", Label: "有结果"},
		{ID: "advanced-edge-dns-github", SourceNode: "advanced-deliver-dns", TargetNode: "advanced-deliver-github"},
		{ID: "advanced-edge-github-end", SourceNode: "advanced-deliver-github", TargetNode: "advanced-end-completed"},
		{ID: "advanced-edge-branch-recovery", SourceNode: "advanced-branch-results", TargetNode: "advanced-recovery-empty", Outcome: "false", Label: "无结果"},
		{ID: "advanced-edge-recovery-end", SourceNode: "advanced-recovery-empty", TargetNode: "advanced-end-manual-review"},
	}
	return PipelineTemplate{
		BoundConfigSnapshot: map[string]any{},
		CreatedAt:           now,
		Description:         "高级上传流程：筛选结果后，有结果自动 DNS 推送并导出 GitHub；无结果进入人工复核。",
		Enabled:             true,
		EntryNodeID:         "advanced-source-group",
		Edges:               edges,
		ID:                  AdvancedUploadPipelineTemplateID,
		Name:                "高级上传回退流程",
		Nodes:               nodes,
		UI:                  &PipelineTemplateUI{Viewport: &PipelineViewport{X: 0, Y: 0, Zoom: 0.75}},
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
		Templates:        []PipelineTemplate{template, AdvancedUploadPipelineTemplate(now)},
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
		Templates:        []PipelineTemplate{template, AdvancedUploadPipelineTemplate(now)},
		UpdatedAt:        now,
	}
	return NormalizePipelineWorkspaceForSave(workspace, schemaVersion, now, sanitize, nil, nil)
}

func ensureBuiltInPipelineTemplates(templates []PipelineTemplate, now string) []PipelineTemplate {
	defaultTemplate := DefaultPipelineTemplate(now)
	advancedTemplate := AdvancedUploadPipelineTemplate(now)
	hasDefault := false
	hasAdvanced := false
	for index := range templates {
		switch strings.TrimSpace(templates[index].ID) {
		case DefaultPipelineTemplateID:
			hasDefault = true
			templates[index] = ensureDefaultPipelineTemplateDefaults(templates[index])
		case AdvancedUploadPipelineTemplateID:
			hasAdvanced = true
		}
	}
	if !hasDefault {
		templates = append([]PipelineTemplate{defaultTemplate}, templates...)
	}
	if !hasAdvanced {
		templates = append(templates, advancedTemplate)
	}
	return templates
}

func ensureDefaultPipelineTemplateDefaults(template PipelineTemplate) PipelineTemplate {
	if strings.TrimSpace(template.ID) != DefaultPipelineTemplateID {
		return template
	}
	for index := range template.Nodes {
		node := &template.Nodes[index]
		if normalizePipelineNodeType(node.NodeType) != PipelineNodeTypeProbe {
			continue
		}
		if node.Config == nil {
			node.Config = map[string]any{}
		}
		for key, value := range pipelineProbeFullModeDefaultConfig() {
			if _, ok := node.Config[key]; !ok {
				node.Config[key] = value
			}
		}
	}
	return template
}

func LegacyPipelineProfileStoreFromWorkspace(workspace PipelineWorkspace, schemaVersion string, now string, sanitize func(map[string]any) map[string]any) PipelineProfileStore {
	workspace = NormalizePipelineWorkspaceForSave(workspace, DefaultPipelineWorkspaceSchemaVersion, now, sanitize, nil, nil)
	activeTemplateID := strings.TrimSpace(workspace.ActiveTemplateID)
	if activeTemplateID == "" && len(workspace.Templates) > 0 {
		activeTemplateID = strings.TrimSpace(workspace.Templates[0].ID)
	}
	items := make([]PipelineProfile, 0, 1)
	for index, target := range workspace.Targets {
		if strings.TrimSpace(target.TemplateID) != activeTemplateID {
			continue
		}
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
		workspace.Templates[index] = migratePipelineCheckOutputNodes(migratePipelineRunProbeNodes(*item, now), now)
	}
	workspace.Templates = ensureBuiltInPipelineTemplates(workspace.Templates, now)
	if strings.TrimSpace(workspace.ActiveTemplateID) == "" && len(workspace.Templates) > 0 {
		workspace.ActiveTemplateID = workspace.Templates[0].ID
	}
	workspace.Targets = normalizePipelineWorkspaceTargets(workspace.Templates, legacyTargets, legacyActiveTargetID, now, sanitize, newTargetID)
	if len(workspace.Targets) > 0 {
		activeTargetID := activeTargetIDForWorkspace(workspace.Templates, workspace.Targets, workspace.ActiveTemplateID, legacyActiveTargetID)
		workspace.ActiveTargetID = firstNonEmptyString(activeTargetID, workspace.Targets[0].ID)
	} else {
		workspace.ActiveTargetID = ""
	}
	return workspace
}

func migratePipelineRunProbeNodes(template PipelineTemplate, now string) PipelineTemplate {
	usedNodeIDs := make(map[string]struct{}, len(template.Nodes)+1)
	usedEdgeIDs := make(map[string]struct{}, len(template.Edges)+1)
	for _, node := range template.Nodes {
		nodeID := strings.TrimSpace(node.ID)
		if nodeID != "" {
			usedNodeIDs[nodeID] = struct{}{}
		}
	}
	for _, edge := range template.Edges {
		edgeID := strings.TrimSpace(edge.ID)
		if edgeID != "" {
			usedEdgeIDs[edgeID] = struct{}{}
		}
	}
	originalNodeCount := len(template.Nodes)
	for index := 0; index < originalNodeCount; index++ {
		node := &template.Nodes[index]
		if strings.ToLower(strings.TrimSpace(node.Action)) != legacyPipelineNodeActionRunProbe {
			continue
		}
		nodeID := strings.TrimSpace(node.ID)
		if nodeID == "" {
			continue
		}
		baseConfig := normalizePipelineNodeConfig(legacyPipelineNodeActionRunProbe, PipelineNodeActionProbeTCP, node.Config)
		node.Action = PipelineNodeActionProbeTCP
		node.NodeType = PipelineNodeTypeProbe
		node.Config = clonePipelineSnapshot(baseConfig)
		if strings.TrimSpace(node.Name) == "" || strings.TrimSpace(node.Name) == "测速" {
			node.Name = "TCP 延迟测速"
		}

		traceID := uniquePipelineElementID(nodeID+"-trace", usedNodeIDs)
		usedNodeIDs[traceID] = struct{}{}
		downloadID := uniquePipelineElementID(nodeID+"-download", usedNodeIDs)
		usedNodeIDs[downloadID] = struct{}{}

		for edgeIndex := range template.Edges {
			if strings.TrimSpace(template.Edges[edgeIndex].SourceNode) == nodeID {
				template.Edges[edgeIndex].SourceNode = downloadID
			}
		}

		template.Nodes = append(template.Nodes,
			PipelineNode{
				Action:    PipelineNodeActionProbeTrace,
				Config:    normalizePipelineNodeConfig(legacyPipelineNodeActionRunProbe, PipelineNodeActionProbeTrace, baseConfig),
				ID:        traceID,
				Name:      "追踪测试",
				NodeType:  PipelineNodeTypeProbe,
				UI:        shiftedPipelineNodeUI(node.UI, 360),
				UpdatedAt: firstNonEmptyString(node.UpdatedAt, now),
			},
			PipelineNode{
				Action:    PipelineNodeActionProbeDownload,
				Config:    normalizePipelineNodeConfig(legacyPipelineNodeActionRunProbe, PipelineNodeActionProbeDownload, baseConfig),
				ID:        downloadID,
				Name:      "下载测速",
				NodeType:  PipelineNodeTypeProbe,
				UI:        shiftedPipelineNodeUI(node.UI, 720),
				UpdatedAt: firstNonEmptyString(node.UpdatedAt, now),
			},
		)

		edgeToTraceID := uniquePipelineElementID("edge-"+nodeID+"-"+traceID, usedEdgeIDs)
		usedEdgeIDs[edgeToTraceID] = struct{}{}
		edgeToDownloadID := uniquePipelineElementID("edge-"+traceID+"-"+downloadID, usedEdgeIDs)
		usedEdgeIDs[edgeToDownloadID] = struct{}{}
		template.Edges = append(template.Edges,
			PipelineEdge{ID: edgeToTraceID, SourceNode: nodeID, TargetNode: traceID},
			PipelineEdge{ID: edgeToDownloadID, SourceNode: traceID, TargetNode: downloadID},
		)
	}
	return template
}

func shiftedPipelineNodeUI(ui *PipelineNodeUI, offsetX float64) *PipelineNodeUI {
	next := &PipelineNodeUI{Width: 320}
	if ui != nil {
		next.Collapsed = ui.Collapsed
		next.Width = ui.Width
		if ui.Position != nil {
			next.Position = &PipelineCanvasPosition{X: ui.Position.X + offsetX, Y: ui.Position.Y}
		}
	}
	if next.Width <= 0 {
		next.Width = 320
	}
	return next
}

func migratePipelineCheckOutputNodes(template PipelineTemplate, now string) PipelineTemplate {
	usedNodeIDs := make(map[string]struct{}, len(template.Nodes)+1)
	outDegree := make(map[string]int, len(template.Nodes))
	checkOutputIDs := make([]string, 0)
	for index := range template.Nodes {
		node := &template.Nodes[index]
		nodeID := strings.TrimSpace(node.ID)
		if nodeID != "" {
			usedNodeIDs[nodeID] = struct{}{}
			outDegree[nodeID] = 0
		}
		if normalizePipelineNodeAction(node.Action) == PipelineNodeActionCheckOutput {
			node.NodeType = PipelineNodeTypeDeliver
			checkOutputIDs = append(checkOutputIDs, nodeID)
		}
	}
	usedEdgeIDs := make(map[string]struct{}, len(template.Edges)+1)
	for _, edge := range template.Edges {
		edgeID := strings.TrimSpace(edge.ID)
		if edgeID != "" {
			usedEdgeIDs[edgeID] = struct{}{}
		}
		sourceID := strings.TrimSpace(edge.SourceNode)
		if sourceID != "" {
			outDegree[sourceID]++
		}
	}
	for _, nodeID := range checkOutputIDs {
		if nodeID == "" || outDegree[nodeID] > 0 {
			continue
		}
		endID := uniquePipelineElementID(nodeID+"-end", usedNodeIDs)
		usedNodeIDs[endID] = struct{}{}
		edgeID := uniquePipelineElementID("edge-"+nodeID+"-end", usedEdgeIDs)
		usedEdgeIDs[edgeID] = struct{}{}
		template.Nodes = append(template.Nodes, PipelineNode{
			Action: PipelineNodeActionEnd,
			Config: map[string]any{
				"message": "流程已结束。",
				"status":  "completed",
			},
			ID:        endID,
			Name:      "结束",
			NodeType:  PipelineNodeTypeEnd,
			UI:        &PipelineNodeUI{Width: 320},
			UpdatedAt: now,
		})
		template.Edges = append(template.Edges, PipelineEdge{
			ID:         edgeID,
			SourceNode: nodeID,
			TargetNode: endID,
		})
		outDegree[nodeID]++
	}
	return template
}

func uniquePipelineElementID(base string, used map[string]struct{}) string {
	normalized := strings.TrimSpace(base)
	if normalized == "" {
		normalized = "pipeline-element"
	}
	if _, exists := used[normalized]; !exists {
		return normalized
	}
	for index := 2; ; index++ {
		candidate := fmt.Sprintf("%s-%d", normalized, index)
		if _, exists := used[candidate]; !exists {
			return candidate
		}
	}
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
	store, err := ParsePipelineProfileStore(value)
	if err != nil {
		return PipelineProfileStore{}
	}
	return store
}

func ParsePipelineProfileStore(value any) (PipelineProfileStore, error) {
	var store PipelineProfileStore
	if value == nil {
		return store, nil
	}
	if err := marshalInto(value, &store); err != nil {
		return PipelineProfileStore{}, err
	}
	return store, nil
}

func PipelineProfilesFromAny(value any) []PipelineProfile {
	profiles, err := ParsePipelineProfiles(value)
	if err != nil {
		return nil
	}
	return profiles
}

func ParsePipelineProfiles(value any) ([]PipelineProfile, error) {
	var profiles []PipelineProfile
	if value == nil {
		return nil, nil
	}
	if err := marshalInto(value, &profiles); err != nil {
		return nil, err
	}
	return profiles, nil
}

func PipelineWorkspaceFromAny(value any) PipelineWorkspace {
	workspace, err := ParsePipelineWorkspace(value)
	if err != nil {
		return PipelineWorkspace{}
	}
	return workspace
}

func ParsePipelineWorkspace(value any) (PipelineWorkspace, error) {
	var workspace PipelineWorkspace
	if value == nil {
		return workspace, nil
	}
	if err := marshalInto(value, &workspace); err != nil {
		return PipelineWorkspace{}, err
	}
	return workspace, nil
}

func PipelineTemplatesFromAny(value any) []PipelineTemplate {
	templates, err := ParsePipelineTemplates(value)
	if err != nil {
		return nil
	}
	return templates
}

func ParsePipelineTemplates(value any) ([]PipelineTemplate, error) {
	var templates []PipelineTemplate
	if value == nil {
		return nil, nil
	}
	if err := marshalInto(value, &templates); err != nil {
		return nil, err
	}
	return templates, nil
}

func PipelineTargetsFromAny(value any) []PipelineTarget {
	targets, err := ParsePipelineTargets(value)
	if err != nil {
		return nil
	}
	return targets
}

func ParsePipelineTargets(value any) ([]PipelineTarget, error) {
	var targets []PipelineTarget
	if value == nil {
		return nil, nil
	}
	if err := marshalInto(value, &targets); err != nil {
		return nil, err
	}
	return targets, nil
}

func marshalInto(value any, target any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, target)
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

func NormalizePipelineNodeType(value string) string {
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

func normalizePipelineNodeType(value string) string {
	return NormalizePipelineNodeType(value)
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
		return PipelineNodeActionProbeTCP
	}
}

func NormalizePipelineNodeAction(value string) string {
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
	case PipelineNodeActionFilterSources:
		return PipelineNodeActionFilterSources
	case PipelineNodeActionCheckOutput:
		return PipelineNodeActionCheckOutput
	case PipelineNodeActionProbeTCP:
		return PipelineNodeActionProbeTCP
	case PipelineNodeActionProbeTrace:
		return PipelineNodeActionProbeTrace
	case PipelineNodeActionProbeDownload:
		return PipelineNodeActionProbeDownload
	default:
		return normalized
	}
}

func normalizePipelineNodeAction(value string) string {
	return NormalizePipelineNodeAction(value)
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
			target.Tags = normalizeStringSlice(slices.Clone(legacy.Tags))
		}
		targets = append(targets, target)
	}
	return targets
}

func normalizePipelineWorkspaceTargets(templates []PipelineTemplate, targets []PipelineTarget, activeTargetID string, now string, sanitize func(map[string]any) map[string]any, newTargetID func(index int) string) []PipelineTarget {
	if len(templates) == 0 {
		return []PipelineTarget{}
	}
	templateIDs := make(map[string]struct{}, len(templates))
	for _, template := range templates {
		if templateID := strings.TrimSpace(template.ID); templateID != "" {
			templateIDs[templateID] = struct{}{}
		}
	}
	next := make([]PipelineTarget, 0, len(targets)+len(templates))
	seenIDs := make(map[string]struct{}, len(targets)+len(templates))
	targetCountByTemplate := make(map[string]int, len(templates))
	for index, target := range targets {
		target.TemplateID = strings.TrimSpace(target.TemplateID)
		if target.TemplateID == "" || !pipelineTemplateIDExists(templateIDs, target.TemplateID) {
			target.TemplateID = strings.TrimSpace(templates[0].ID)
		}
		target.ID = strings.TrimSpace(target.ID)
		if target.ID == "" {
			if newTargetID != nil {
				target.ID = strings.TrimSpace(newTargetID(index))
			}
			if target.ID == "" {
				target.ID = fmt.Sprintf("%s-target-%d", firstNonEmptyString(target.TemplateID, "pipeline-template"), index+1)
			}
		}
		target.ID = uniquePipelineElementID(target.ID, seenIDs)
		seenIDs[target.ID] = struct{}{}
		target.Name = firstNonEmptyString(strings.TrimSpace(target.Name), fmt.Sprintf("目标 %d", index+1))
		target.CreatedAt = firstNonEmptyString(target.CreatedAt, now)
		target.UpdatedAt = firstNonEmptyString(target.UpdatedAt, now)
		target.DNSPushPolicy = NormalizePipelineDNSPushPolicy(target.DNSPushPolicy)
		if sanitize != nil {
			target.ConfigSnapshot = sanitize(clonePipelineSnapshot(target.ConfigSnapshot))
		} else {
			target.ConfigSnapshot = clonePipelineSnapshot(target.ConfigSnapshot)
		}
		target.Domain = firstNonEmptyString(strings.TrimSpace(target.Domain), pipelineDomainFromSnapshot(target.ConfigSnapshot))
		target.Region = firstNonEmptyString(strings.TrimSpace(target.Region), "当前配置")
		target.Tags = normalizeStringSlice(slices.Clone(target.Tags))
		next = append(next, target)
		targetCountByTemplate[target.TemplateID]++
	}
	for index, target := range pipelineWorkspaceCompatibilityTargets(templates, targets, activeTargetID, now, sanitize, newTargetID) {
		if targetCountByTemplate[strings.TrimSpace(target.TemplateID)] > 0 {
			continue
		}
		target.ID = uniquePipelineElementID(firstNonEmptyString(strings.TrimSpace(target.ID), fmt.Sprintf("pipeline-target-%d", index+1)), seenIDs)
		seenIDs[target.ID] = struct{}{}
		next = append(next, target)
		targetCountByTemplate[strings.TrimSpace(target.TemplateID)]++
	}
	return next
}

func pipelineTemplateIDExists(templateIDs map[string]struct{}, templateID string) bool {
	_, ok := templateIDs[strings.TrimSpace(templateID)]
	return ok
}

func compatibilityTargetIDForTemplate(templates []PipelineTemplate, targets []PipelineTarget, activeTemplateID string) string {
	activeTemplateID = strings.TrimSpace(activeTemplateID)
	if activeTemplateID == "" && len(templates) > 0 {
		activeTemplateID = strings.TrimSpace(templates[0].ID)
	}
	var first string
	for _, target := range targets {
		if strings.TrimSpace(target.TemplateID) != activeTemplateID {
			continue
		}
		targetID := strings.TrimSpace(target.ID)
		if first == "" {
			first = targetID
		}
		if target.Enabled {
			return targetID
		}
	}
	return first
}

func activeTargetIDForWorkspace(templates []PipelineTemplate, targets []PipelineTarget, activeTemplateID string, activeTargetID string) string {
	activeTemplateID = strings.TrimSpace(activeTemplateID)
	if activeTemplateID == "" && len(templates) > 0 {
		activeTemplateID = strings.TrimSpace(templates[0].ID)
	}
	activeTargetID = strings.TrimSpace(activeTargetID)
	if activeTargetID != "" {
		for _, target := range targets {
			if strings.TrimSpace(target.ID) == activeTargetID && strings.TrimSpace(target.TemplateID) == activeTemplateID {
				return activeTargetID
			}
		}
	}
	return compatibilityTargetIDForTemplate(templates, targets, activeTemplateID)
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
