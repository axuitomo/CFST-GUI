package appcore

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/colodict"
	"github.com/axuitomo/CFST-GUI/internal/githubcore"
	"github.com/axuitomo/CFST-GUI/internal/probecore"
	"github.com/axuitomo/CFST-GUI/internal/runtimecleanup"
	"github.com/axuitomo/CFST-GUI/internal/sourceparse"
	"github.com/axuitomo/CFST-GUI/internal/utils"
)

const (
	CommandSchemaVersion = "cfst-gui-command-v2"
	ConfigSchemaVersion  = "cfst-gui-config-v2"
	EventSchemaVersion   = "cfst-gui-event-v2"
	ProbeEventChannel    = "probe:event"
)

type Clock func() time.Time

type RuntimeLimits struct {
	MaxStage1Concurrency int
	MaxStage2Concurrency int
	MaxStage3Concurrency int
}

type ServiceOptions struct {
	ArchiveSaveSourceProfiles func(SourceProfileStore) error
	ArchiveStorageState       func() any
	ArchiveWriteConfig        func(map[string]any) error
	AppVersion                string
	CancelTimeout             time.Duration
	Clock                     Clock
	CloudflareAPIBaseURL      string
	ColoPaths                 colodict.Paths
	ConfigSavedHook           func(map[string]any)
	ConfigPolicy              ConfigPolicy
	DebugLogger               *utils.DebugLogger
	DefaultExportDir          string
	EventSink                 EventSink
	GitHubAPIBaseURL          string
	GitHubDefaults            githubcore.ConfigDefaults
	HTTPClient                *http.Client
	Limits                    RuntimeLimits
	ProbeConfigOptions        probecore.ConfigSnapshotOptions
	ProbeHooks                ProbeExecutionHooks
	RuntimeCleanupBusy        func() bool
	RuntimeCleanupHook        func()
	SchedulerDefaults         SchedulerConfig
	SourceHTTPTimeout         time.Duration
	SourceResolver            sourceparse.Resolver
	Storage                   StorageLayout
	StorageCommands           StorageCommandHooks
	TelegramAPIBaseURL        string
}

type Service struct {
	mu               sync.RWMutex
	options          ServiceOptions
	runtime          *ProbeRuntime
	taskStore        *TaskStore
	probeMu          sync.Mutex
	eventSeq         map[string]int
	terminalEvents   map[string]string
	publishMu        sync.Mutex
	runtimeCleanupMu sync.Mutex
	schedulerRunMu   sync.Mutex
	cleaner          *runtimecleanup.Cleaner
}

func (s *Service) SetRuntimeCleanupHooks(busy func() bool, cleanup func()) {
	s.mu.Lock()
	s.options.RuntimeCleanupBusy = busy
	s.options.RuntimeCleanupHook = cleanup
	s.mu.Unlock()
}

func (s *Service) SetConfigSavedHook(hook func(map[string]any)) {
	s.mu.Lock()
	s.options.ConfigSavedHook = hook
	s.mu.Unlock()
}

func (s *Service) SetArchiveStorageStateProvider(provider func() any) {
	s.mu.Lock()
	s.options.ArchiveStorageState = provider
	s.mu.Unlock()
}

func (s *Service) SetArchiveImportHooks(writeConfig func(map[string]any) error, saveProfiles func(SourceProfileStore) error) {
	s.mu.Lock()
	s.options.ArchiveWriteConfig = writeConfig
	s.options.ArchiveSaveSourceProfiles = saveProfiles
	s.mu.Unlock()
}

func NewService(options ServiceOptions) *Service {
	if options.CancelTimeout <= 0 {
		options.CancelTimeout = 2 * time.Second
	}
	if options.Clock == nil {
		options.Clock = time.Now
	}
	if options.HTTPClient == nil {
		options.HTTPClient = http.DefaultClient
	}
	if options.DebugLogger == nil {
		options.DebugLogger = utils.NewDebugLogger()
	}
	return &Service{
		options:        options,
		runtime:        NewProbeRuntime(),
		taskStore:      NewTaskStore(options.Storage.TasksRoot(), options.Clock),
		eventSeq:       map[string]int{},
		terminalEvents: map[string]string{},
	}
}

func (s *Service) DebugLogger() *utils.DebugLogger {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.options.DebugLogger
}

func (s *Service) ProbeRuntime() *ProbeRuntime {
	return s.runtime
}

func (s *Service) SetEventSink(sink EventSink) {
	s.mu.Lock()
	s.options.EventSink = sink
	s.mu.Unlock()
}

func (s *Service) SetStorageLayout(layout StorageLayout) {
	s.mu.Lock()
	s.options.Storage = layout
	if s.taskStore == nil {
		s.taskStore = NewTaskStore(layout.TasksRoot(), s.options.Clock)
	} else {
		s.taskStore.SetRoot(layout.TasksRoot())
	}
	s.mu.Unlock()
}

func (s *Service) SetStorageCommandHooks(hooks StorageCommandHooks) {
	s.mu.Lock()
	s.options.StorageCommands = hooks
	s.mu.Unlock()
}

func (s *Service) SetColoPaths(paths colodict.Paths) {
	s.mu.Lock()
	s.options.ColoPaths = paths
	s.mu.Unlock()
}

func (s *Service) ColoPaths() colodict.Paths {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.options.ColoPaths
}

func (s *Service) StorageLayout() StorageLayout {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.options.Storage
}

func (s *Service) Emit(ctx context.Context, event ProbeEvent) {
	if ctx == nil {
		ctx = context.Background()
	}
	s.publishMu.Lock()
	defer s.publishMu.Unlock()
	event, sink, ok := s.prepareProbeEvent(event)
	if !ok {
		return
	}
	emitPreparedProbeEvent(ctx, sink, event)
}

func (s *Service) prepareProbeEvent(event ProbeEvent) (ProbeEvent, EventSink, bool) {
	taskID := strings.TrimSpace(event.TaskID)
	s.mu.Lock()
	if terminal := s.terminalEvents[taskID]; terminal != "" && isSharedProbeLifecycleEvent(event.Event) {
		s.mu.Unlock()
		return event, nil, false
	}
	if isSharedProbeTerminalEvent(event.Event) {
		s.terminalEvents[taskID] = event.Event
	}
	sink := s.options.EventSink
	clock := s.options.Clock
	if event.Seq <= 0 {
		s.eventSeq[taskID]++
		event.Seq = s.eventSeq[taskID]
	} else if event.Seq > s.eventSeq[taskID] {
		s.eventSeq[taskID] = event.Seq
	}
	s.mu.Unlock()
	if event.SchemaVersion == "" {
		event.SchemaVersion = EventSchemaVersion
	}
	if event.TS == "" {
		event.TS = clock().Format(time.RFC3339)
	}
	return event, sink, true
}

func emitPreparedProbeEvent(ctx context.Context, sink EventSink, event ProbeEvent) {
	if sink == nil {
		return
	}
	sink.EmitProbeEvent(ctx, event)
}

func (s *Service) PublishProbeEvent(ctx context.Context, event ProbeEvent) error {
	if ctx == nil {
		ctx = context.Background()
	}
	s.publishMu.Lock()
	defer s.publishMu.Unlock()
	if event.Payload == nil {
		event.Payload = map[string]any{}
	}
	event, sink, ok := s.prepareProbeEvent(event)
	if !ok {
		return nil
	}
	err := s.RecordProbeEvent(event.TaskID, event.Event, event.Payload)
	if event.Event == "probe.failed" {
		s.recordTaskFailureNotification(ctx, event.TaskID, event.Payload)
	}
	emitPreparedProbeEvent(ctx, sink, event)
	return err
}

func (s *Service) resetProbeEventState(taskID string) {
	taskID = strings.TrimSpace(taskID)
	s.mu.Lock()
	delete(s.eventSeq, taskID)
	delete(s.terminalEvents, taskID)
	s.mu.Unlock()
}

func isSharedProbeLifecycleEvent(event string) bool {
	switch event {
	case "probe.preprocessed", "probe.progress", "probe.resumed", "probe.speed", "probe.partial_export", "probe.cooling", "probe.cancelled", "probe.completed", "probe.failed":
		return true
	default:
		return false
	}
}

func isSharedProbeTerminalEvent(event string) bool {
	switch event {
	case "probe.cancelled", "probe.completed", "probe.failed":
		return true
	default:
		return false
	}
}

func NewCommandResult(code string, data any, message string, ok bool, taskID *string, warnings []string) CommandResult {
	if warnings == nil {
		warnings = []string{}
	}
	return CommandResult{
		Code:          code,
		Data:          data,
		Message:       message,
		OK:            ok,
		SchemaVersion: CommandSchemaVersion,
		TaskID:        taskID,
		Warnings:      probecoreDedupeStrings(warnings),
	}
}

func probecoreDedupeStrings(values []string) []string {
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
