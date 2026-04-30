package main

import (
	"sync"
	"time"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const desktopProbeEventName = "desktop:probe"

type desktopProbeEventEnvelope struct {
	Event         string                 `json:"event"`
	Payload       map[string]interface{} `json:"payload"`
	SchemaVersion string                 `json:"schema_version"`
	Seq           int                    `json:"seq"`
	TaskID        string                 `json:"task_id"`
	TS            string                 `json:"ts"`
}

type desktopProbeEmitter struct {
	app            *App
	taskID         string
	throttle       time.Duration
	mu             sync.Mutex
	seq            int
	lastStage      string
	lastProgressAt time.Time
}

func newDesktopProbeEmitter(app *App, taskID string, throttle time.Duration) *desktopProbeEmitter {
	if throttle <= 0 {
		throttle = 100 * time.Millisecond
	}
	return &desktopProbeEmitter{
		app:      app,
		taskID:   taskID,
		throttle: throttle,
	}
}

func (e *desktopProbeEmitter) emit(event string, payload map[string]interface{}) {
	if e == nil {
		return
	}
	if payload == nil {
		payload = map[string]interface{}{}
	}

	e.mu.Lock()
	e.seq++
	seq := e.seq
	e.mu.Unlock()

	if e.app == nil || e.app.ctx == nil {
		return
	}

	wruntime.EventsEmit(e.app.ctx, desktopProbeEventName, desktopProbeEventEnvelope{
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

	e.emit("probe.progress", map[string]interface{}{
		"failed":    failed,
		"passed":    passed,
		"processed": processed,
		"stage":     stage,
		"total":     total,
	})
}
