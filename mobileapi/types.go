package mobileapi

import (
	"context"
	"sync"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/appcore"
	"github.com/axuitomo/CFST-GUI/internal/probecore"
	"github.com/axuitomo/CFST-GUI/internal/runtimecleanup"
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
	traceCancels      map[int64]func()
	traceCancelSeq    int64
	downloadCancels   map[int64]func()
	downloadCancelSeq int64
	progressThrottle  time.Duration
	lastProgressStage string
	lastProgressAt    time.Time
	taskSnapshots     map[string]taskSnapshot
	taskEventMetadata map[string]map[string]any
	runtimeCleanupMu  sync.Mutex
	cleaner           *runtimecleanup.Cleaner

	processMonitorMu   sync.Mutex
	heartbeatCancel    context.CancelFunc
	heartbeatDone      chan struct{}
	heartbeatStartedAt time.Time
}

type mobileSchedulerConfig struct {
	Enabled          bool     `json:"enabled"`
	IntervalMinutes  int      `json:"interval_minutes"`
	DailyTimes       []string `json:"daily_times"`
	AutoDNSPush      bool     `json:"auto_dns_push"`
	AutoGitHubExport bool     `json:"auto_github_export"`
	SkipIfActive     bool     `json:"skip_if_active"`
	RunMode          string   `json:"run_mode"`
}

type mobileSchedulerStatus struct {
	Enabled               bool   `json:"enabled"`
	NextRunAt             string `json:"next_run_at"`
	LastRunAt             string `json:"last_run_at"`
	LastTaskID            string `json:"last_task_id"`
	LastProbeStatus       string `json:"last_probe_status"`
	LastDNSStatus         string `json:"last_dns_status"`
	LastGitHubStatus      string `json:"last_github_status"`
	LastMessage           string `json:"last_message"`
	RunStage              string `json:"run_stage"`
	ConfigSource          string `json:"config_source"`
	UploadInputCount      int    `json:"upload_input_count"`
	UploadFilteredCount   int    `json:"upload_filtered_count"`
	CloudflareUploadCount int    `json:"cloudflare_upload_count"`
	GitHubUploadCount     int    `json:"github_upload_count"`
}

type probeConfig = probecore.ProbeConfig

type commandResult = appcore.CommandResult

type sourceSummary = probecore.SourceSummary
type probeTaskContext = probecore.TaskContext

type probeRunResult = appcore.ProbeRunResult

type probeSummary = probecore.ProbeSummary
type probeRow = probecore.ProbeRow

type probeResultRow = appcore.ProbeResultRow
type desktopSource = appcore.Source
type desktopSourceStatus = appcore.SourceStatus
type desktopProbePayload = appcore.ProbePayload

type taskProgressSnapshot struct {
	Failed    int    `json:"failed"`
	Passed    int    `json:"passed"`
	Processed int    `json:"processed"`
	Stage     string `json:"stage"`
	Total     int    `json:"total,omitempty"`
}

type exportRecordSnapshot struct {
	FileName     string `json:"file_name"`
	Format       string `json:"format"`
	LastWriteAt  string `json:"last_write_at,omitempty"`
	SourcePath   string `json:"source_path,omitempty"`
	TargetDir    string `json:"target_dir"`
	TaskID       string `json:"task_id"`
	WrittenCount int    `json:"written_count"`
}

type taskSnapshot struct {
	CompletedAt     string                `json:"completed_at,omitempty"`
	ConfigDigest    string                `json:"config_digest,omitempty"`
	CurrentStage    string                `json:"current_stage,omitempty"`
	ExportRecord    *exportRecordSnapshot `json:"export_record,omitempty"`
	FailureSummary  map[string]any        `json:"failure_summary,omitempty"`
	Progress        *taskProgressSnapshot `json:"progress,omitempty"`
	ResumeCapable   bool                  `json:"resume_capable,omitempty"`
	RuntimeAttached bool                  `json:"runtime_attached,omitempty"`
	SessionState    string                `json:"session_state,omitempty"`
	StartedAt       string                `json:"started_at,omitempty"`
	Status          string                `json:"status"`
	TaskContext     map[string]any        `json:"task_context,omitempty"`
	TaskID          string                `json:"task_id"`
	UpdatedAt       string                `json:"updated_at"`
}

type sourcePreviewPayload struct {
	Config       map[string]any `json:"config"`
	PersistState bool           `json:"persist_state"`
	PreviewLimit int            `json:"preview_limit"`
	Source       desktopSource  `json:"source"`
}
