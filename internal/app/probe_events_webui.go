//go:build webui

package app

import (
	"fmt"

	"github.com/axuitomo/CFST-GUI/internal/utils"
)

func (a *App) emitProbeEvent(event desktopProbeEventEnvelope) {
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
}
