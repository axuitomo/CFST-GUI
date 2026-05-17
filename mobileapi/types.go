package mobileapi

import (
	"sync"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/appcore"
	"github.com/axuitomo/CFST-GUI/internal/probecore"
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
	taskSnapshots     map[string]taskSnapshot
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
	FileName    string `json:"file_name"`
	Format      string `json:"format"`
	LastWriteAt string `json:"last_write_at,omitempty"`
	TargetDir   string `json:"target_dir"`
	TaskID      string `json:"task_id"`
	WrittenCount int   `json:"written_count"`
}

type taskSnapshot struct {
	CompletedAt   string                 `json:"completed_at,omitempty"`
	ConfigDigest  string                 `json:"config_digest,omitempty"`
	CurrentStage  string                 `json:"current_stage,omitempty"`
	ExportRecord  *exportRecordSnapshot  `json:"export_record,omitempty"`
	FailureSummary map[string]any        `json:"failure_summary,omitempty"`
	Progress      *taskProgressSnapshot  `json:"progress,omitempty"`
	ResumeCapable bool                   `json:"resume_capable,omitempty"`
	RuntimeAttached bool                 `json:"runtime_attached,omitempty"`
	SessionState  string                 `json:"session_state,omitempty"`
	StartedAt     string                 `json:"started_at,omitempty"`
	Status        string                 `json:"status"`
	TaskContext   map[string]any         `json:"task_context,omitempty"`
	TaskID        string                 `json:"task_id"`
	UpdatedAt     string                 `json:"updated_at"`
}

type sourcePreviewPayload struct {
	Config       map[string]any `json:"config"`
	PersistState bool           `json:"persist_state"`
	PreviewLimit int            `json:"preview_limit"`
	Source       desktopSource  `json:"source"`
}
