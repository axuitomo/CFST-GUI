package contracts

import (
	"github.com/axuitomo/CFST-GUI/internal/kernel/domain/archivecore"
	"github.com/axuitomo/CFST-GUI/internal/kernel/domain/probe/probecore"
	"github.com/axuitomo/CFST-GUI/internal/kernel/infra/utils"
)

const (
	CommandSchemaVersion   = "cfst-gui-command-v2"
	ConfigSchemaVersion    = "cfst-gui-config-v2"
	MaxSourceContentBytes  = archivecore.MaxConfigArchiveBytes
	MaxSourceHTTPBodyBytes = MaxSourceContentBytes
	MaxTaskResultsBytes    = MaxSourceContentBytes
)

func NewCommandResult(code string, data any, message string, ok bool, taskID *string, warnings []string) CommandResult {
	return CommandResult{
		Code:          code,
		Data:          data,
		Message:       message,
		OK:            ok,
		SchemaVersion: CommandSchemaVersion,
		TaskID:        taskID,
		Warnings:      dedupeCommandWarnings(warnings),
	}
}

func dedupeCommandWarnings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

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
	AndroidExportURI     string         `json:"android_export_uri,omitempty"`
	Config               map[string]any `json:"config"`
	ConfigSource         string         `json:"config_source"`
	DisablePostProbePush bool           `json:"disable_post_probe_push,omitempty"`
	Sources              []Source       `json:"sources"`
	TaskID               string         `json:"task_id"`
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

type SourcePreviewRequest struct {
	Config       map[string]any `json:"config"`
	PersistState bool           `json:"persist_state"`
	PreviewLimit int            `json:"preview_limit"`
	Source       Source         `json:"source"`
}

type ProbeRunResult struct {
	Config             probecore.ProbeConfig    `json:"config"`
	DebugLogPath       string                   `json:"debugLogPath,omitempty"`
	DurationMS         int64                    `json:"durationMs"`
	FailureStage       string                   `json:"failure_stage,omitempty"`
	OutputFile         string                   `json:"outputFile"`
	Results            []probecore.ProbeRow     `json:"results"`
	Source             probecore.SourceSummary  `json:"source"`
	SourceStatuses     []SourceStatus           `json:"sourceStatuses"`
	StartedAt          string                   `json:"startedAt"`
	Summary            probecore.ProbeSummary   `json:"summary"`
	TaskID             string                   `json:"task_id"`
	TaskContext        probecore.TaskContext    `json:"task_context"`
	TraceDiagnostics   map[string]any           `json:"trace_diagnostics,omitempty"`
	UploadNotification *UploadNotification      `json:"upload_notification,omitempty"`
	Warnings           []string                 `json:"warnings"`
	SchemaVersion      string                   `json:"schemaVersion"`
	RawResults         []utils.CloudflareIPData `json:"-"`
}

type ProbeResultRow struct {
	Address         string   `json:"address"`
	Colo            *string  `json:"colo"`
	DownloadMbps    *float64 `json:"download_mbps"`
	ExportStatus    string   `json:"export_status"`
	LastErrorCode   *string  `json:"last_error_code"`
	MaxDownloadMbps *float64 `json:"max_download_mbps"`
	Note            *string  `json:"note"`
	SourcePort      *int     `json:"source_port"`
	StageStatus     string   `json:"stage_status"`
	TCPLatencyMS    *float64 `json:"tcp_latency_ms"`
	TestPort        *int     `json:"test_port"`
	TraceLatencyMS  *float64 `json:"trace_latency_ms"`
}

type SourceProfileItem struct {
	ColoFilter     string   `json:"colo_filter,omitempty"`
	ColoFilterMode string   `json:"colo_filter_mode,omitempty"`
	CreatedAt      string   `json:"created_at"`
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	IPMode         string   `json:"ip_mode,omitempty"`
	Sources        []Source `json:"sources"`
	UpdatedAt      string   `json:"updated_at"`
}

type SourceProfileStore struct {
	ActiveProfileID string              `json:"active_profile_id"`
	Items           []SourceProfileItem `json:"items"`
	SchemaVersion   string              `json:"schema_version"`
	UpdatedAt       string              `json:"updated_at"`
}

type TaskResultsRequest struct {
	TaskID   string `json:"task_id"`
	Path     string `json:"path"`
	SortBy   string `json:"sort_by"`
	Order    string `json:"order"`
	Filter   string `json:"filter"`
	IPFilter string `json:"ip_filter"`
	Limit    int    `json:"limit"`
	Offset   int    `json:"offset"`
}

type TaskResultsPage struct {
	Count       int              `json:"count"`
	Results     []ProbeResultRow `json:"results"`
	TotalCount  int              `json:"total_count"`
	SourceCount int              `json:"-"`
	Found       bool             `json:"-"`
	SourceKind  string           `json:"-"`
	SourcePath  string           `json:"-"`
}

type StorageSetResult struct {
	Migration any
	Storage   any
}

type StorageHealthResult struct {
	Health  any
	Storage any
}

type StorageCommandHooks struct {
	Health func(map[string]any) (StorageHealthResult, error)
	Set    func(map[string]any) (StorageSetResult, error)
}

type UploadProviderReport struct {
	Status      string `json:"status"`
	UploadCount int    `json:"upload_count"`
}

type UploadProviderResult struct {
	ID          string `json:"id,omitempty"`
	Label       string `json:"label"`
	Provider    string `json:"provider"`
	Status      string `json:"status"`
	UploadCount int    `json:"upload_count"`
}

type UploadNotificationTopEntry struct {
	Colo            string  `json:"colo,omitempty"`
	DownloadSpeedMB float64 `json:"download_speed_mb,omitempty"`
	IP              string  `json:"ip"`
	Rank            int     `json:"rank"`
	TCPDelayMS      float64 `json:"tcp_delay_ms,omitempty"`
	TraceDelayMS    float64 `json:"trace_delay_ms,omitempty"`
}

type UploadNotificationFunnel struct {
	Candidates int `json:"candidates"`
	Passed     int `json:"passed"`
	Qualified  int `json:"qualified"`
	Scanned    int `json:"scanned"`
}

type UploadNotification struct {
	CloudflareStatus      string                       `json:"cloudflare_status,omitempty"`
	CloudflareUploadCount int                          `json:"cloudflare_upload_count,omitempty"`
	CreatedAt             string                       `json:"created_at"`
	DurationMS            int64                        `json:"duration_ms,omitempty"`
	Funnel                *UploadNotificationFunnel    `json:"funnel,omitempty"`
	GitHubStatus          string                       `json:"github_status,omitempty"`
	GitHubUploadCount     int                          `json:"github_upload_count,omitempty"`
	Message               string                       `json:"message"`
	Providers             []UploadProviderResult       `json:"providers,omitempty"`
	Source                string                       `json:"source"`
	Status                string                       `json:"status"`
	TaskID                string                       `json:"task_id,omitempty"`
	TopEntries            []UploadNotificationTopEntry `json:"top_entries,omitempty"`
}
