package appcore

import (
	"strings"
	"time"
)

type DraftSaveRequest struct {
	ConfigSnapshot       map[string]any `json:"config_snapshot"`
	LegacyConfigSnapshot map[string]any `json:"configSnapshot"`
}

func (request DraftSaveRequest) Snapshot() map[string]any {
	if len(request.ConfigSnapshot) > 0 {
		return request.ConfigSnapshot
	}
	return request.LegacyConfigSnapshot
}

type DraftStatus struct {
	ConfigSavedAt    string         `json:"config_saved_at,omitempty"`
	ConfigSnapshot   map[string]any `json:"config_snapshot,omitempty"`
	Error            string         `json:"error,omitempty"`
	Exists           bool           `json:"exists"`
	IsNewerThanSaved bool           `json:"is_newer_than_saved"`
	Path             string         `json:"path"`
	SavedAt          string         `json:"saved_at,omitempty"`
}

func (status DraftStatus) Payload() map[string]any {
	payload := map[string]any{
		"exists":              status.Exists,
		"is_newer_than_saved": status.IsNewerThanSaved,
		"path":                status.Path,
	}
	if status.ConfigSavedAt != "" {
		payload["config_saved_at"] = status.ConfigSavedAt
	}
	if len(status.ConfigSnapshot) > 0 {
		payload["config_snapshot"] = status.ConfigSnapshot
	}
	if status.Error != "" {
		payload["error"] = status.Error
	}
	if status.SavedAt != "" {
		payload["saved_at"] = status.SavedAt
	}
	return payload
}

func (s *Service) DraftStatus() DraftStatus {
	draft, draftErr := s.LoadDraft()
	status := DraftStatus{
		Exists: draft.Existed,
		Path:   draft.Path,
	}
	if draftErr != nil {
		status.Error = draftErr.Error()
		return status
	}
	if !draft.Existed {
		return status
	}
	status.ConfigSnapshot = draft.Snapshot
	if !draft.SavedAt.IsZero() {
		status.SavedAt = draft.SavedAt.Format(time.RFC3339)
	}
	config, configErr := s.LoadConfig()
	if configErr == nil && !config.SavedAt.IsZero() {
		status.ConfigSavedAt = config.SavedAt.Format(time.RFC3339)
	}
	status.IsNewerThanSaved = !draft.SavedAt.IsZero() && (config.SavedAt.IsZero() || draft.SavedAt.After(config.SavedAt))
	return status
}

func (s *Service) invokeDraft(command, payloadJSON string) CommandResult {
	switch command {
	case "draft.load":
		if _, err := decodeCommandPayload[struct{}](payloadJSON); err != nil {
			return NewCommandResult("COMMAND_PAYLOAD_INVALID", nil, err.Error(), false, nil, nil)
		}
		return NewCommandResult("DRAFT_READY", s.DraftStatus().Payload(), "草稿状态已读取。", true, nil, nil)
	case "draft.save":
		request, err := decodeCommandPayload[DraftSaveRequest](payloadJSON)
		if err != nil {
			return NewCommandResult("DRAFT_INVALID", nil, err.Error(), false, nil, nil)
		}
		snapshot := request.Snapshot()
		if len(snapshot) == 0 {
			return NewCommandResult("DRAFT_INVALID", nil, "缺少 config_snapshot。", false, nil, nil)
		}
		if _, err := s.SaveDraft(snapshot); err != nil {
			return NewCommandResult("DRAFT_WRITE_FAILED", nil, err.Error(), false, nil, nil)
		}
		return NewCommandResult("DRAFT_SAVE_OK", s.DraftStatus().Payload(), "草稿已保存。", true, nil, nil)
	case "draft.discard":
		if _, err := decodeCommandPayload[struct{}](payloadJSON); err != nil {
			return NewCommandResult("COMMAND_PAYLOAD_INVALID", nil, err.Error(), false, nil, nil)
		}
		if err := s.DiscardDraft(); err != nil {
			return NewCommandResult("DRAFT_DISCARD_FAILED", s.DraftStatus().Payload(), err.Error(), false, nil, nil)
		}
		return NewCommandResult("DRAFT_DISCARDED", s.DraftStatus().Payload(), "草稿已丢弃。", true, nil, nil)
	default:
		return NewCommandResult("COMMAND_UNKNOWN", nil, "unknown command: "+strings.TrimSpace(command), false, nil, nil)
	}
}
