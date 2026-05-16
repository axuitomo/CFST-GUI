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

type sourcePreviewPayload struct {
	Config       map[string]any `json:"config"`
	PersistState bool           `json:"persist_state"`
	PreviewLimit int            `json:"preview_limit"`
	Source       desktopSource  `json:"source"`
}
