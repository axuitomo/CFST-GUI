package app

import (
	"sync"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/task"
)

const desktopProbeEventName = "desktop:probe"

type desktopProbeEventEnvelope struct {
	Event         string         `json:"event"`
	Payload       map[string]any `json:"payload"`
	SchemaVersion string         `json:"schema_version"`
	Seq           int            `json:"seq"`
	TaskID        string         `json:"task_id"`
	TS            string         `json:"ts"`
}

type desktopProbeEmitter struct {
	app            *App
	metadata       map[string]any
	taskID         string
	throttle       time.Duration
	mu             sync.Mutex
	seq            int
	lastStage      string
	lastProgressAt time.Time
}

func newDesktopProbeEmitter(app *App, taskID string, throttle time.Duration, metadata ...map[string]any) *desktopProbeEmitter {
	if throttle <= 0 {
		throttle = 100 * time.Millisecond
	}
	mergedMetadata := map[string]any{}
	if len(metadata) > 0 {
		for key, value := range metadata[0] {
			mergedMetadata[key] = value
		}
	}
	return &desktopProbeEmitter{
		app:      app,
		metadata: mergedMetadata,
		taskID:   taskID,
		throttle: throttle,
	}
}

func (e *desktopProbeEmitter) emit(event string, payload map[string]any) {
	if e == nil {
		return
	}
	if payload == nil {
		payload = map[string]any{}
	}
	for key, value := range e.metadata {
		if _, exists := payload[key]; !exists {
			payload[key] = value
		}
	}

	e.mu.Lock()
	e.seq++
	seq := e.seq
	e.mu.Unlock()

	if e.app == nil {
		return
	}

	e.app.recordTaskSnapshotEvent(e.taskID, event, payload)

	e.app.emitProbeEvent(desktopProbeEventEnvelope{
		Event:         event,
		Payload:       payload,
		SchemaVersion: guiSchemaVersion,
		Seq:           seq,
		TaskID:        e.taskID,
		TS:            time.Now().Format(time.RFC3339),
	})
}

func (e *desktopProbeEmitter) emitProgress(stage string, processed, passed, failed, total int) {
	if e == nil {
		return
	}

	now := time.Now()
	e.mu.Lock()
	shouldEmit := processed <= 1 || total <= 0 || processed >= total || stage != e.lastStage || now.Sub(e.lastProgressAt) >= e.throttle
	if shouldEmit {
		e.lastStage = stage
		e.lastProgressAt = now
	}
	e.mu.Unlock()

	if !shouldEmit {
		return
	}

	e.emit("probe.progress", map[string]any{
		"failed":    failed,
		"passed":    passed,
		"processed": processed,
		"stage":     stage,
		"total":     total,
	})
}

func (e *desktopProbeEmitter) emitSpeed(sample task.DownloadSpeedSample) {
	if e == nil {
		return
	}
	e.emit("probe.speed", map[string]any{
		"average_speed_mb_s":  sample.AverageSpeedMBs,
		"average_ready":       sample.AverageReady,
		"attempt":             sample.Attempt,
		"body_read":           sample.BodyRead,
		"bytes_read":          sample.BytesRead,
		"colo":                sample.Colo,
		"current_ready":       sample.CurrentReady,
		"current_speed_mb_s":  sample.CurrentSpeedMBs,
		"elapsed_ms":          sample.ElapsedMS,
		"ip":                  sample.IP,
		"measured_bytes":      sample.MeasuredBytes,
		"measured_elapsed_ms": sample.MeasuredElapsedMS,
		"sample_bytes":        sample.SampleBytes,
		"sample_elapsed_ms":   sample.SampleElapsedMS,
		"stage":               sample.Stage,
		"transfer_complete":   sample.TransferComplete,
	})
}
