package mobileapi

import (
	"sync"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/probecore"
	"github.com/axuitomo/CFST-GUI/utils"
)

const schemaVersion = "cfst-gui-mobile-v1"

type EventSink interface {
	OnProbeEvent(eventJSON string)
}

type Service struct {
	runMu sync.Mutex

	stateMu           sync.Mutex
	baseDir           string
	eventSink         EventSink
	eventSeq          int
	currentTaskID     string
	cancelTaskID      string
	cancelRequested   bool
	pauseRequested    bool
	pausedTaskID      string
	pauseCond         *sync.Cond
	downloadCancel    func()
	downloadCancelSeq int64
	progressThrottle  time.Duration
	lastProgressStage string
	lastProgressAt    time.Time
}

type probeConfig = probecore.ProbeConfig

type commandResult struct {
	Code          string   `json:"code"`
	Data          any      `json:"data"`
	Message       string   `json:"message"`
	OK            bool     `json:"ok"`
	SchemaVersion string   `json:"schema_version"`
	TaskID        *string  `json:"task_id"`
	Warnings      []string `json:"warnings"`
}

type sourceSummary = probecore.SourceSummary
type probeTaskContext = probecore.TaskContext

type probeRunResult struct {
	Config         probeConfig           `json:"config"`
	DebugLogPath   string                `json:"debugLogPath,omitempty"`
	DurationMS     int64                 `json:"durationMs"`
	OutputFile     string                `json:"outputFile"`
	Results        []probeRow            `json:"results"`
	Source         sourceSummary         `json:"source"`
	SourceStatuses []desktopSourceStatus `json:"sourceStatuses"`
	StartedAt      string                `json:"startedAt"`
	Summary        probeSummary          `json:"summary"`
	TaskContext    probeTaskContext      `json:"task_context"`
	Warnings       []string              `json:"warnings"`
	SchemaVersion  string                `json:"schemaVersion"`

	rawResults []utils.CloudflareIPData
}

type probeSummary = probecore.ProbeSummary
type probeRow = probecore.ProbeRow

type probeResultRow struct {
	Address         string   `json:"address"`
	Colo            *string  `json:"colo"`
	DownloadMbps    *float64 `json:"download_mbps"`
	ExportStatus    string   `json:"export_status"`
	LastErrorCode   *string  `json:"last_error_code"`
	MaxDownloadMbps *float64 `json:"max_download_mbps"`
	StageStatus     string   `json:"stage_status"`
	TCPLatencyMS    *float64 `json:"tcp_latency_ms"`
	TestPort        *int     `json:"test_port"`
	TraceLatencyMS  *float64 `json:"trace_latency_ms"`
}

type desktopSource struct {
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

type desktopSourceStatus struct {
	ID               string `json:"id"`
	LastFetchedAt    string `json:"last_fetched_at"`
	LastFetchedCount int    `json:"last_fetched_count"`
	StatusText       string `json:"status_text"`
}

type desktopProbePayload struct {
	AndroidExportURI string          `json:"android_export_uri"`
	Config           map[string]any  `json:"config"`
	ConfigSource     string          `json:"config_source"`
	Sources          []desktopSource `json:"sources"`
	TaskID           string          `json:"task_id"`
}

type sourcePreviewPayload struct {
	Config       map[string]any `json:"config"`
	PersistState bool           `json:"persist_state"`
	PreviewLimit int            `json:"preview_limit"`
	Source       desktopSource  `json:"source"`
}
