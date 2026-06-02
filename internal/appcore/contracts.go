package appcore

import (
	"github.com/axuitomo/CFST-GUI/internal/probecore"
	"github.com/axuitomo/CFST-GUI/internal/utils"
)

type CommandResult struct {
	Code          string   `json:"code"`
	Data          any      `json:"data"`
	Message       string   `json:"message"`
	OK            bool     `json:"ok"`
	SchemaVersion string   `json:"schema_version"`
	TaskID        *string  `json:"task_id"`
	Warnings      []string `json:"warnings"`
}

type ProbePayload struct {
	AndroidExportURI  string         `json:"android_export_uri,omitempty"`
	Config            map[string]any `json:"config"`
	ConfigSource      string         `json:"config_source"`
	PipelineDomain    string         `json:"pipeline_domain,omitempty"`
	PipelineID        string         `json:"pipeline_id,omitempty"`
	PipelineProfile   string         `json:"pipeline_profile_name,omitempty"`
	PipelineProfileID string         `json:"pipeline_profile_id,omitempty"`
	PipelineRegion    string         `json:"pipeline_region,omitempty"`
	Sources           []Source       `json:"sources"`
	TaskID            string         `json:"task_id"`
}

type Source struct {
	ColoFilter       string `json:"colo_filter"`
	ColoFilterMode   string `json:"colo_filter_mode"`
	Content          string `json:"content"`
	Enabled          bool   `json:"enabled"`
	ID               string `json:"id"`
	IPLimit          int    `json:"ip_limit"`
	IPMode           string `json:"ip_mode"`
	Kind             string `json:"kind"`
	Label            string `json:"label"`
	LastFetchedAt    string `json:"last_fetched_at"`
	LastFetchedCount int    `json:"last_fetched_count"`
	Name             string `json:"name"`
	Path             string `json:"path"`
	StatusText       string `json:"status_text"`
	URL              string `json:"url"`
}

type SourceStatus struct {
	ID               string `json:"id"`
	LastFetchedAt    string `json:"last_fetched_at"`
	LastFetchedCount int    `json:"last_fetched_count"`
	StatusText       string `json:"status_text"`
}

type ProbeRunResult struct {
	Config           probecore.ProbeConfig    `json:"config"`
	DebugLogPath     string                   `json:"debugLogPath,omitempty"`
	DurationMS       int64                    `json:"durationMs"`
	FailureStage     string                   `json:"failure_stage,omitempty"`
	OutputFile       string                   `json:"outputFile"`
	Results          []probecore.ProbeRow     `json:"results"`
	Source           probecore.SourceSummary  `json:"source"`
	SourceStatuses   []SourceStatus           `json:"sourceStatuses"`
	StartedAt        string                   `json:"startedAt"`
	Summary          probecore.ProbeSummary   `json:"summary"`
	TaskContext      probecore.TaskContext    `json:"task_context"`
	TraceDiagnostics map[string]any           `json:"trace_diagnostics,omitempty"`
	Warnings         []string                 `json:"warnings"`
	SchemaVersion    string                   `json:"schemaVersion"`
	RawResults       []utils.CloudflareIPData `json:"-"`
}

type ProbeResultRow struct {
	Address         string   `json:"address"`
	Colo            *string  `json:"colo"`
	DownloadMbps    *float64 `json:"download_mbps"`
	ExportStatus    string   `json:"export_status"`
	LastErrorCode   *string  `json:"last_error_code"`
	MaxDownloadMbps *float64 `json:"max_download_mbps"`
	SourcePort      *int     `json:"source_port"`
	StageStatus     string   `json:"stage_status"`
	TCPLatencyMS    *float64 `json:"tcp_latency_ms"`
	TestPort        *int     `json:"test_port"`
	TraceLatencyMS  *float64 `json:"trace_latency_ms"`
}

type SourceProfileItem struct {
	CreatedAt string   `json:"created_at"`
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Sources   []Source `json:"sources"`
	UpdatedAt string   `json:"updated_at"`
}

type SourceProfileStore struct {
	ActiveProfileID string              `json:"active_profile_id"`
	Items           []SourceProfileItem `json:"items"`
	SchemaVersion   string              `json:"schema_version"`
	UpdatedAt       string              `json:"updated_at"`
}
