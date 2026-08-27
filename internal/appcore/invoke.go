package appcore

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/axuitomo/CFST-GUI/internal/probecore"
)

func (s *Service) Invoke(command, payloadJSON string) CommandResult {
	result, handled := s.TryInvoke(command, payloadJSON)
	if handled {
		return result
	}
	command = strings.ToLower(strings.TrimSpace(command))
	return NewCommandResult("COMMAND_UNKNOWN", nil, fmt.Sprintf("unknown command: %s", command), false, nil, nil)
}

func (s *Service) TryInvoke(command, payloadJSON string) (CommandResult, bool) {
	command = strings.ToLower(strings.TrimSpace(command))
	switch command {
	case "config.load", "config.save":
		return s.invokeConfig(command, payloadJSON), true
	case "config.export":
		return s.invokeConfigExport(payloadJSON), true
	case "config.backup":
		return s.invokeConfigBackup(payloadJSON), true
	case "storage.set", "storage.health":
		return s.invokeStorage(command, payloadJSON), true
	case "draft.load", "draft.save", "draft.discard":
		return s.invokeDraft(command, payloadJSON), true
	case "source.preview", "source.fetch":
		request, err := decodeCommandPayload[SourcePreviewRequest](payloadJSON)
		if err != nil {
			return NewCommandResult("SOURCE_PAYLOAD_INVALID", nil, err.Error(), false, nil, nil), true
		}
		return s.inspectSource(request, command == "source.fetch"), true
	case "source_profiles.load", "source_profiles.save", "source_profiles.update_current", "source_profiles.save_store", "source_profiles.switch", "source_profiles.delete":
		return s.invokeSourceProfiles(command, payloadJSON), true
	case "telegram.test":
		return s.invokeTelegramTest(payloadJSON), true
	case "github.test":
		return s.invokeGitHubTest(payloadJSON), true
	case "github.export":
		return s.invokeGitHubExport(payloadJSON), true
	case "results.export_csv":
		return s.invokeResultsCSVExport(payloadJSON), true
	case "cloudflare.list":
		return s.invokeCloudflareList(payloadJSON), true
	case "cloudflare.push":
		return s.invokeCloudflarePush(payloadJSON), true
	case "colo.status":
		return s.invokeColoStatus(payloadJSON), true
	case "colo.update":
		return s.invokeColoUpdate(payloadJSON), true
	case "colo.process":
		return s.invokeColoProcess(payloadJSON), true
	case "archive.export":
		return s.invokeArchiveExport(payloadJSON), true
	case "archive.import":
		return s.invokeArchiveImport(payloadJSON, "配置压缩包已导入。"), true
	case "webdav.test":
		return s.invokeWebDAVTest(payloadJSON), true
	case "webdav.backup":
		return s.invokeWebDAVBackup(payloadJSON), true
	case "webdav.restore":
		return s.invokeWebDAVRestore(payloadJSON), true
	case "scheduler.status":
		return s.invokeSchedulerStatus(payloadJSON), true
	case "scheduler.refresh":
		return s.invokeSchedulerRefresh(payloadJSON), true
	case "scheduler.run":
		return s.invokeSchedulerRun(payloadJSON), true
	case "debug.export":
		return s.invokeDebugExport(payloadJSON), true
	case "diagnostics.export":
		return s.invokeDiagnosticExport(payloadJSON), true
	case "storage.cleanup", "storage.cleanup_status", "storage.force_clean_legacy":
		return s.invokeStorageMaintenance(command, payloadJSON), true
	case "runtime.status":
		if _, err := decodeCommandPayload[struct{}](payloadJSON); err != nil {
			return NewCommandResult("COMMAND_PAYLOAD_INVALID", nil, err.Error(), false, nil, nil), true
		}
		return s.RuntimeStatus(), true
	case "probe.start", "probe.run":
		request, err := decodeCommandPayload[ProbePayload](payloadJSON)
		if err != nil {
			return NewCommandResult("PROBE_PAYLOAD_INVALID", nil, err.Error(), false, nil, nil), true
		}
		if command == "probe.start" {
			return s.StartProbe(request), true
		}
		result, runErr := s.RunProbe(request)
		taskID := strings.TrimSpace(request.TaskID)
		if runErr != nil {
			code := "PROBE_FAILED"
			var taskErr *ProbeTaskError
			if errors.As(runErr, &taskErr) && taskErr.Code != "" {
				code = taskErr.Code
			} else if errors.Is(runErr, probecore.ErrProbeCanceled) {
				code = "PROBE_CANCELLED"
			}
			return NewCommandResult(code, result, runErr.Error(), false, stringPtr(taskID), result.Warnings), true
		}
		return NewCommandResult("PROBE_COMPLETED", result, "探测任务已完成。", true, stringPtr(resultTaskID(result.TaskID, taskID, request.TaskID)), result.Warnings), true
	case "probe.pause", "probe.cancel", "probe.resume", "task.get", "task.list", "task.results":
	default:
		return CommandResult{}, false
	}
	payload, err := decodeCommandObject(payloadJSON)
	if err != nil {
		return NewCommandResult("COMMAND_PAYLOAD_INVALID", nil, err.Error(), false, nil, nil), true
	}
	taskID := strings.TrimSpace(stringValue(firstNonNil(payload["task_id"], payload["taskId"]), ""))
	switch command {
	case "probe.pause":
		return s.PauseProbe(ProbeControlRequest{TaskID: taskID}), true
	case "probe.cancel":
		return s.CancelProbe(ProbeControlRequest{TaskID: taskID}), true
	case "probe.resume":
		return s.ResumeProbe(ProbeControlRequest{TaskID: taskID}), true
	case "task.get":
		return s.GetTask(TaskQueryRequest{TaskID: taskID}), true
	case "task.list":
		limit := intValue(firstNonNil(payload["limit"], payload["page_size"], payload["pageSize"]), 20)
		return s.ListTasks(TaskQueryRequest{Limit: limit}), true
	case "task.results":
		return s.InvokeTaskResults(payloadJSON), true
	default:
		return CommandResult{}, false
	}
}

func resultTaskID(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func (s *Service) invokeSourceProfiles(command, payloadJSON string) CommandResult {
	switch command {
	case "source_profiles.load":
		if _, err := decodeCommandPayload[struct{}](payloadJSON); err != nil {
			return NewCommandResult("COMMAND_PAYLOAD_INVALID", nil, err.Error(), false, nil, nil)
		}
		return s.LoadSourceProfiles()
	case "source_profiles.save":
		request, err := decodeCommandPayload[SourceProfileSaveRequest](payloadJSON)
		if err != nil {
			return NewCommandResult("SOURCE_PROFILE_INVALID", nil, err.Error(), false, nil, nil)
		}
		return s.SaveSourceProfile(request)
	case "source_profiles.update_current":
		request, err := decodeCommandPayload[SourceProfileUpdateRequest](payloadJSON)
		if err != nil {
			return NewCommandResult("SOURCE_PROFILE_INVALID", nil, err.Error(), false, nil, nil)
		}
		return s.UpdateCurrentSourceProfile(request)
	case "source_profiles.save_store":
		request, err := decodeCommandPayload[SourceProfileStoreSaveRequest](payloadJSON)
		if err != nil {
			return NewCommandResult("SOURCE_PROFILE_INVALID", nil, err.Error(), false, nil, nil)
		}
		return s.SaveSourceProfileStore(request)
	case "source_profiles.switch":
		request, err := decodeCommandPayload[SourceProfileSelectRequest](payloadJSON)
		if err != nil {
			return NewCommandResult("SOURCE_PROFILE_INVALID", nil, err.Error(), false, nil, nil)
		}
		return s.SwitchSourceProfile(request)
	case "source_profiles.delete":
		request, err := decodeCommandPayload[SourceProfileSelectRequest](payloadJSON)
		if err != nil {
			return NewCommandResult("SOURCE_PROFILE_INVALID", nil, err.Error(), false, nil, nil)
		}
		return s.DeleteSourceProfile(request)
	default:
		return NewCommandResult("COMMAND_UNKNOWN", nil, fmt.Sprintf("unknown command: %s", command), false, nil, nil)
	}
}

func decodeCommandPayload[T any](payloadJSON string) (T, error) {
	var payload T
	payloadJSON = strings.TrimSpace(payloadJSON)
	if payloadJSON == "" {
		payloadJSON = "{}"
	}
	err := json.Unmarshal([]byte(payloadJSON), &payload)
	return payload, err
}

func decodeCommandObject(payloadJSON string) (map[string]any, error) {
	payloadJSON = strings.TrimSpace(payloadJSON)
	if payloadJSON == "" {
		payloadJSON = "{}"
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(payloadJSON), &payload); err != nil {
		return nil, err
	}
	if payload == nil {
		payload = map[string]any{}
	}
	return payload, nil
}
