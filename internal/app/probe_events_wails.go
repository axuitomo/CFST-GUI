//go:build !webui

package app

import (
	"fmt"

	"github.com/axuitomo/CFST-GUI/internal/app/wailsruntime"
	"github.com/axuitomo/CFST-GUI/internal/appcore"
	"github.com/axuitomo/CFST-GUI/internal/utils"
)

func (a *App) emitProbeEvent(event appcore.ProbeEvent) {
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
	// 上传结论的系统通知在 Go 侧发：托盘常驻时窗口可能是隐藏的，前端发起通知既拿不到
	// 焦点也没法保证宿主可用。消息体与前端此前的通知文案保持一致。
	if body := uploadNotificationBody(event); body != "" {
		go sendDesktopNotification("CFST 上传结论", body)
	}
	if a.ctx == nil {
		return
	}
	wailsruntime.EventsEmit(a.ctx, appcore.ProbeEventChannel, event)
}
