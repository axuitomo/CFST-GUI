package mobileapi

import (
	"context"
	"fmt"
	"strings"

	"github.com/axuitomo/CFST-GUI/internal/appcore"
	"github.com/axuitomo/CFST-GUI/internal/configvalue"
	"github.com/axuitomo/CFST-GUI/internal/utils"
)

func (s *Service) recordAndroidExportResult(payload map[string]any) string {
	taskID := strings.TrimSpace(configvalue.String(configvalue.FirstNonNil(payload["task_id"], payload["taskId"]), ""))
	targetURI := strings.TrimSpace(configvalue.String(configvalue.FirstNonNil(payload["target_uri"], payload["targetUri"], payload["uri"], payload["target_path"], payload["targetPath"]), ""))
	sourcePath := strings.TrimSpace(configvalue.String(configvalue.FirstNonNil(payload["source_path"], payload["sourcePath"], payload["android_export_source_path"], payload["androidExportSourcePath"]), ""))
	written := configvalue.Int(configvalue.FirstNonNil(payload["written"], payload["written_count"], payload["writtenCount"]), 0)
	status := strings.TrimSpace(configvalue.String(configvalue.FirstNonNil(payload["status"], payload["android_export_status"], payload["androidExportStatus"]), ""))
	ok := configvalue.Bool(payload["ok"], false) || status == "written"
	message := strings.TrimSpace(configvalue.String(configvalue.FirstNonNil(payload["message"], payload["error"], payload["android_export_error"], payload["androidExportError"]), ""))
	eventPayload := map[string]any{
		"source_path": sourcePath,
		"stage":       "export",
		"target_path": targetURI,
		"written":     written,
	}
	if ok {
		if message == "" {
			message = "Android 系统导出文件已写入。"
		}
		eventPayload["message"] = message
		s.emit(taskID, "probe.export_completed", eventPayload)
		return encodeCommand(appcore.NewCommandResult("ANDROID_EXPORT_OK", eventPayload, message, true, &taskID, nil))
	}
	if message == "" {
		message = "Android 系统导出文件失败。"
	}
	eventPayload["message"] = message
	eventPayload["recoverable"] = true
	s.emit(taskID, "probe.export_failed", eventPayload)
	return encodeCommand(appcore.NewCommandResult("ANDROID_EXPORT_FAILED", eventPayload, message, false, &taskID, nil))
}

func (s *Service) emit(taskID, event string, payload map[string]any) {
	if payload == nil {
		payload = map[string]any{}
	}
	if err := s.core.PublishProbeEvent(context.Background(), appcore.ProbeEvent{Event: event, Payload: payload, TaskID: taskID}); err != nil {
		s.core.DebugLogger().Event("mobile.snapshot.persist_failed", map[string]any{
			"error":   err.Error(),
			"event":   event,
			"task_id": taskID,
		})
		_ = utils.AppendErrorLog(s.errorLogPath(), "mobile.snapshot.persist_failed", map[string]any{
			"message":      err.Error(),
			"source_event": event,
			"task_id":      taskID,
		})
	}
}

func (s *Service) deliverProbeEvent(event appcore.ProbeEvent) {
	s.stateMu.Lock()
	sink := s.eventSink
	s.stateMu.Unlock()
	if sink == nil {
		return
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			_ = utils.AppendErrorLog(s.errorLogPath(), "mobile.probe_event_emit_failed", map[string]any{
				"event":   event.Event,
				"message": fmt.Sprintf("移动端探测事件发送失败：%v", recovered),
				"task_id": event.TaskID,
			})
		}
	}()
	sink.OnProbeEvent(encodeJSON(event))
}
