//go:build !webui

package app

import (
	"fmt"

	"github.com/axuitomo/CFST-GUI/internal/app/wailsruntime"
	"github.com/axuitomo/CFST-GUI/internal/appcore"
	"github.com/axuitomo/CFST-GUI/internal/utils"
)

func (a *App) emitProbeEvent(event appcore.ProbeEvent) {
	fmt.Printf("[DBG-EVENT] emitProbeEvent event=%s task_id=%s ctxNil=%v hubNil=%v\n", event.Event, event.TaskID, a.ctx == nil, a.eventHub == nil)
	defer func() {
		if recovered := recover(); recovered != nil {
			_ = utils.AppendErrorLog(errorLogFilePath(), "desktop.probe_event_emit_failed", map[string]any{
				"event":   event.Event,
				"message": fmt.Sprintf("桌面探测事件发送失败：%v", recovered),
				"task_id": event.TaskID,
			})
		}
	}()
	if a.eventHub != nil {
		a.eventHub.publish(event)
	}
	if a.ctx == nil {
		fmt.Printf("[DBG-EVENT] ctx is nil, skipping Wails emit for %s\n", event.Event)
		return
	}
	wailsruntime.EventsEmit(a.ctx, appcore.ProbeEventChannel, event)
	fmt.Printf("[DBG-EVENT] wails Emit called for %s\n", event.Event)
}
