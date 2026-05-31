package mobileapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/appcore"
	"github.com/axuitomo/CFST-GUI/internal/probecore"
)

func (s *Service) LoadPipelineProfiles() string {
	workspace, warnings, err := s.loadPipelineWorkspaceOrDefault()
	if err != nil {
		return encodeCommand(commandResultFor("PIPELINE_PROFILE_LOAD_FAILED", nil, err.Error(), false, nil, warnings))
	}
	return encodeCommand(commandResultFor("PIPELINE_PROFILE_LOAD_OK", s.pipelineProfileStoreFromWorkspace(workspace), "策略管道已加载。", true, nil, warnings))
}

func (s *Service) SavePipelineProfiles(payloadJSON string) string {
	payload, err := decodeObject(payloadJSON)
	if err != nil {
		return encodeCommand(commandResultFor("PIPELINE_PROFILE_INVALID", nil, err.Error(), false, nil, nil))
	}
	store := appcore.PipelineProfileStoreFromAny(firstNonNil(payload["pipeline_profiles"], payload["pipelineProfiles"], payload["store"]))
	if len(store.Items) == 0 {
		snapshot, loadErr := s.loadConfigSnapshotFromDisk()
		if loadErr != nil {
			return encodeCommand(commandResultFor("PIPELINE_PROFILE_INVALID", nil, loadErr.Error(), false, nil, nil))
		}
		store = s.defaultPipelineProfileStoreFromSnapshot(snapshot)
	}
	workspace, warnings, loadErr := s.loadPipelineWorkspaceOrDefault()
	if loadErr != nil {
		return encodeCommand(commandResultFor("PIPELINE_PROFILE_LOAD_FAILED", nil, loadErr.Error(), false, nil, warnings))
	}
	workspace = s.applyLegacyProfileStoreToWorkspace(workspace, s.normalizePipelineProfileStoreForSave(store))
	if err := appcore.ValidatePipelineWorkspace(workspace); err != nil {
		return encodeCommand(commandResultFor("PIPELINE_PROFILE_INVALID", nil, err.Error(), false, nil, warnings))
	}
	if err := s.savePipelineWorkspace(workspace); err != nil {
		return encodeCommand(commandResultFor("PIPELINE_PROFILE_SAVE_FAILED", nil, err.Error(), false, nil, nil))
	}
	return encodeCommand(commandResultFor("PIPELINE_PROFILE_SAVE_OK", s.pipelineProfileStoreFromWorkspace(workspace), "策略管道已保存。", true, nil, nil))
}

func (s *Service) SavePipelineProfile(payloadJSON string) string {
	payload, err := decodeObject(payloadJSON)
	if err != nil {
		return encodeCommand(commandResultFor("PIPELINE_PROFILE_INVALID", nil, err.Error(), false, nil, nil))
	}
	profile := mobilePipelineProfileFromPayload(payload)
	targetPayload := map[string]any{
		"config_snapshot": profile.ConfigSnapshot,
		"created_at":      profile.CreatedAt,
		"dns_push_policy": profile.DNSPushPolicy,
		"domain":          profile.Domain,
		"enabled":         profile.Enabled,
		"id":              profile.ID,
		"name":            profile.Name,
		"set_active":      firstNonNil(payload["set_active"], payload["setActive"]),
		"target_id":       profile.ID,
		"region":          profile.Region,
		"updated_at":      profile.UpdatedAt,
	}
	command := decodeCommandResult(s.SavePipelineTarget(encodeJSON(targetPayload)))
	if !command.OK {
		return encodeCommand(commandResultFor("PIPELINE_PROFILE_SAVE_FAILED", nil, command.Message, false, nil, command.Warnings))
	}
	return encodeCommand(commandResultFor("PIPELINE_PROFILE_SAVE_OK", s.pipelineProfileStoreFromWorkspace(mobilePipelineWorkspaceFromAny(command.Data)), "策略已保存。", true, nil, command.Warnings))
}

func (s *Service) DeletePipelineProfile(payloadJSON string) string {
	payload, err := decodeObject(payloadJSON)
	if err != nil {
		return encodeCommand(commandResultFor("PIPELINE_PROFILE_INVALID", nil, err.Error(), false, nil, nil))
	}
	profileID := strings.TrimSpace(stringValue(firstNonNil(payload["profile_id"], payload["profileId"], payload["id"]), ""))
	if profileID == "" {
		return encodeCommand(commandResultFor("PIPELINE_PROFILE_INVALID", nil, "缺少 profile_id。", false, nil, nil))
	}
	command := decodeCommandResult(s.DeletePipelineTarget(encodeJSON(map[string]any{"target_id": profileID})))
	if !command.OK {
		return encodeCommand(commandResultFor("PIPELINE_PROFILE_DELETE_FAILED", nil, command.Message, false, nil, command.Warnings))
	}
	return encodeCommand(commandResultFor("PIPELINE_PROFILE_DELETE_OK", s.pipelineProfileStoreFromWorkspace(mobilePipelineWorkspaceFromAny(command.Data)), "策略已删除。", true, nil, command.Warnings))
}

func (s *Service) LoadPipelineWorkspace() string {
	workspace, warnings, err := s.loadPipelineWorkspaceOrDefault()
	if err != nil {
		return encodeCommand(commandResultFor("PIPELINE_WORKSPACE_LOAD_FAILED", nil, err.Error(), false, nil, warnings))
	}
	return encodeCommand(commandResultFor("PIPELINE_WORKSPACE_LOAD_OK", workspace, "策略工作流已加载。", true, nil, warnings))
}

func (s *Service) SavePipelineWorkspace(payloadJSON string) string {
	payload, err := decodeObject(payloadJSON)
	if err != nil {
		return encodeCommand(commandResultFor("PIPELINE_WORKSPACE_INVALID", nil, err.Error(), false, nil, nil))
	}
	workspace := mobilePipelineWorkspaceFromPayload(payload)
	if len(workspace.Templates) == 0 && len(workspace.Targets) == 0 {
		snapshot, loadErr := s.loadConfigSnapshotFromDisk()
		if loadErr != nil {
			return encodeCommand(commandResultFor("PIPELINE_WORKSPACE_INVALID", nil, loadErr.Error(), false, nil, nil))
		}
		workspace = s.defaultPipelineWorkspaceFromSnapshot(snapshot)
	}
	workspace = s.normalizePipelineWorkspaceForSave(workspace)
	if err := appcore.ValidatePipelineWorkspace(workspace); err != nil {
		return encodeCommand(commandResultFor("PIPELINE_WORKSPACE_INVALID", nil, err.Error(), false, nil, nil))
	}
	if err := s.savePipelineWorkspace(workspace); err != nil {
		return encodeCommand(commandResultFor("PIPELINE_WORKSPACE_SAVE_FAILED", nil, err.Error(), false, nil, nil))
	}
	return encodeCommand(commandResultFor("PIPELINE_WORKSPACE_SAVE_OK", workspace, "策略工作流已保存。", true, nil, nil))
}

func (s *Service) SavePipelineTemplate(payloadJSON string) string {
	payload, err := decodeObject(payloadJSON)
	if err != nil {
		return encodeCommand(commandResultFor("PIPELINE_WORKSPACE_INVALID", nil, err.Error(), false, nil, nil))
	}
	workspace, warnings, err := s.loadPipelineWorkspaceOrDefault()
	if err != nil {
		return encodeCommand(commandResultFor("PIPELINE_WORKSPACE_LOAD_FAILED", nil, err.Error(), false, nil, warnings))
	}
	template := mobilePipelineTemplateFromPayload(payload)
	if strings.TrimSpace(template.ID) == "" {
		template.ID = fmt.Sprintf("pipeline-template-%d", time.Now().UnixNano())
	}
	now := nowRFC3339()
	template.UpdatedAt = now
	if strings.TrimSpace(template.CreatedAt) == "" {
		template.CreatedAt = now
	}
	if strings.TrimSpace(template.Name) == "" {
		template.Name = "工作流"
	}
	if len(template.Nodes) == 0 {
		template.Nodes = appcore.DefaultPipelineTemplate(now).Nodes
	}
	if len(template.Edges) == 0 {
		template.Edges = appcore.DefaultPipelineTemplate(now).Edges
	}
	if strings.TrimSpace(template.EntryNodeID) == "" && len(template.Nodes) > 0 {
		template.EntryNodeID = template.Nodes[0].ID
	}
	updated := false
	for index := range workspace.Templates {
		if workspace.Templates[index].ID != template.ID {
			continue
		}
		if strings.TrimSpace(template.CreatedAt) == "" {
			template.CreatedAt = workspace.Templates[index].CreatedAt
		}
		workspace.Templates[index] = template
		updated = true
		break
	}
	if !updated {
		workspace.Templates = append(workspace.Templates, template)
	}
	if boolValue(firstNonNil(payload["set_active"], payload["setActive"]), true) {
		workspace.ActiveTemplateID = template.ID
	}
	workspace = s.normalizePipelineWorkspaceForSave(workspace)
	if err := appcore.ValidatePipelineWorkspace(workspace); err != nil {
		return encodeCommand(commandResultFor("PIPELINE_WORKSPACE_INVALID", nil, err.Error(), false, nil, warnings))
	}
	if err := s.savePipelineWorkspace(workspace); err != nil {
		return encodeCommand(commandResultFor("PIPELINE_WORKSPACE_SAVE_FAILED", nil, err.Error(), false, nil, warnings))
	}
	return encodeCommand(commandResultFor("PIPELINE_WORKSPACE_SAVE_OK", workspace, "工作流模板已保存。", true, nil, warnings))
}

func (s *Service) DeletePipelineTemplate(payloadJSON string) string {
	payload, err := decodeObject(payloadJSON)
	if err != nil {
		return encodeCommand(commandResultFor("PIPELINE_WORKSPACE_INVALID", nil, err.Error(), false, nil, nil))
	}
	templateID := strings.TrimSpace(stringValue(firstNonNil(payload["template_id"], payload["templateId"], payload["id"]), ""))
	if templateID == "" {
		return encodeCommand(commandResultFor("PIPELINE_WORKSPACE_INVALID", nil, "缺少 template_id。", false, nil, nil))
	}
	workspace, warnings, err := s.loadPipelineWorkspaceOrDefault()
	if err != nil {
		return encodeCommand(commandResultFor("PIPELINE_WORKSPACE_LOAD_FAILED", nil, err.Error(), false, nil, warnings))
	}
	nextItems := make([]pipelineTemplate, 0, len(workspace.Templates))
	deleted := false
	for _, item := range workspace.Templates {
		if item.ID == templateID {
			deleted = true
			continue
		}
		nextItems = append(nextItems, item)
	}
	if !deleted {
		return encodeCommand(commandResultFor("PIPELINE_WORKSPACE_NOT_FOUND", nil, "未找到工作流模板。", false, nil, warnings))
	}
	workspace.Templates = nextItems
	if workspace.ActiveTemplateID == templateID {
		workspace.ActiveTemplateID = ""
	}
	for index := range workspace.Targets {
		if workspace.Targets[index].TemplateID == templateID {
			workspace.Targets[index].TemplateID = mobileFirstNonEmpty(workspace.ActiveTemplateID, appcore.DefaultPipelineTemplateID)
		}
	}
	workspace = s.normalizePipelineWorkspaceForSave(workspace)
	if err := appcore.ValidatePipelineWorkspace(workspace); err != nil {
		return encodeCommand(commandResultFor("PIPELINE_WORKSPACE_INVALID", nil, err.Error(), false, nil, warnings))
	}
	if err := s.savePipelineWorkspace(workspace); err != nil {
		return encodeCommand(commandResultFor("PIPELINE_WORKSPACE_SAVE_FAILED", nil, err.Error(), false, nil, warnings))
	}
	return encodeCommand(commandResultFor("PIPELINE_WORKSPACE_SAVE_OK", workspace, "工作流模板已删除。", true, nil, warnings))
}

func (s *Service) SavePipelineTarget(payloadJSON string) string {
	payload, err := decodeObject(payloadJSON)
	if err != nil {
		return encodeCommand(commandResultFor("PIPELINE_WORKSPACE_INVALID", nil, err.Error(), false, nil, nil))
	}
	workspace, warnings, err := s.loadPipelineWorkspaceOrDefault()
	if err != nil {
		return encodeCommand(commandResultFor("PIPELINE_WORKSPACE_LOAD_FAILED", nil, err.Error(), false, nil, warnings))
	}
	target := mobilePipelineTargetFromPayload(payload)
	if len(target.ConfigSnapshot) == 0 {
		snapshot := mapValue(firstNonNil(payload["config_snapshot"], payload["configSnapshot"]))
		if len(snapshot) == 0 {
			snapshot, err = s.loadConfigSnapshotFromDisk()
			if err != nil {
				return encodeCommand(commandResultFor("PIPELINE_WORKSPACE_INVALID", nil, err.Error(), false, nil, warnings))
			}
		}
		target.ConfigSnapshot = sanitizeMobileConfigSnapshot(snapshot)
	}
	if strings.TrimSpace(target.ID) == "" {
		target.ID = fmt.Sprintf("pipeline-target-%d", time.Now().UnixNano())
	}
	now := nowRFC3339()
	target.UpdatedAt = now
	if strings.TrimSpace(target.CreatedAt) == "" {
		target.CreatedAt = now
	}
	if strings.TrimSpace(target.Name) == "" {
		target.Name = "目标"
	}
	if strings.TrimSpace(target.Domain) == "" {
		target.Domain = mobilePipelineDomainFromSnapshot(target.ConfigSnapshot)
	}
	if strings.TrimSpace(target.Region) == "" {
		target.Region = "未分组"
	}
	if strings.TrimSpace(target.TemplateID) == "" {
		target.TemplateID = mobileFirstNonEmpty(workspace.ActiveTemplateID, appcore.DefaultPipelineTemplateID)
	}
	target.DNSPushPolicy = appcore.NormalizePipelineDNSPushPolicy(target.DNSPushPolicy)
	if !mobilePipelineTargetPayloadHasEnabled(payload) {
		target.Enabled = true
	}
	updated := false
	for index := range workspace.Targets {
		if workspace.Targets[index].ID != target.ID {
			continue
		}
		if strings.TrimSpace(target.CreatedAt) == "" {
			target.CreatedAt = workspace.Targets[index].CreatedAt
		}
		workspace.Targets[index] = target
		updated = true
		break
	}
	if !updated {
		workspace.Targets = append(workspace.Targets, target)
	}
	if boolValue(firstNonNil(payload["set_active"], payload["setActive"]), true) {
		workspace.ActiveTargetID = target.ID
	}
	workspace = s.normalizePipelineWorkspaceForSave(workspace)
	if err := appcore.ValidatePipelineWorkspace(workspace); err != nil {
		return encodeCommand(commandResultFor("PIPELINE_WORKSPACE_INVALID", nil, err.Error(), false, nil, warnings))
	}
	if err := s.savePipelineWorkspace(workspace); err != nil {
		return encodeCommand(commandResultFor("PIPELINE_WORKSPACE_SAVE_FAILED", nil, err.Error(), false, nil, warnings))
	}
	return encodeCommand(commandResultFor("PIPELINE_WORKSPACE_SAVE_OK", workspace, "工作流目标已保存。", true, nil, warnings))
}

func (s *Service) DeletePipelineTarget(payloadJSON string) string {
	payload, err := decodeObject(payloadJSON)
	if err != nil {
		return encodeCommand(commandResultFor("PIPELINE_WORKSPACE_INVALID", nil, err.Error(), false, nil, nil))
	}
	targetID := strings.TrimSpace(stringValue(firstNonNil(payload["target_id"], payload["targetId"], payload["profile_id"], payload["profileId"], payload["id"]), ""))
	if targetID == "" {
		return encodeCommand(commandResultFor("PIPELINE_WORKSPACE_INVALID", nil, "缺少 target_id。", false, nil, nil))
	}
	workspace, warnings, err := s.loadPipelineWorkspaceOrDefault()
	if err != nil {
		return encodeCommand(commandResultFor("PIPELINE_WORKSPACE_LOAD_FAILED", nil, err.Error(), false, nil, warnings))
	}
	nextItems := make([]pipelineTarget, 0, len(workspace.Targets))
	deleted := false
	for _, item := range workspace.Targets {
		if item.ID == targetID {
			deleted = true
			continue
		}
		nextItems = append(nextItems, item)
	}
	if !deleted {
		return encodeCommand(commandResultFor("PIPELINE_WORKSPACE_NOT_FOUND", nil, "未找到工作流目标。", false, nil, warnings))
	}
	workspace.Targets = nextItems
	if workspace.ActiveTargetID == targetID {
		workspace.ActiveTargetID = ""
	}
	workspace = s.normalizePipelineWorkspaceForSave(workspace)
	if err := appcore.ValidatePipelineWorkspace(workspace); err != nil {
		return encodeCommand(commandResultFor("PIPELINE_WORKSPACE_INVALID", nil, err.Error(), false, nil, warnings))
	}
	if err := s.savePipelineWorkspace(workspace); err != nil {
		return encodeCommand(commandResultFor("PIPELINE_WORKSPACE_SAVE_FAILED", nil, err.Error(), false, nil, warnings))
	}
	return encodeCommand(commandResultFor("PIPELINE_WORKSPACE_SAVE_OK", workspace, "工作流目标已删除。", true, nil, warnings))
}

func (s *Service) RunPipeline(payloadJSON string) string {
	var payload pipelineRunPayload
	if err := decodeInto(payloadJSON, &payload); err != nil {
		return encodeCommand(commandResultFor("PIPELINE_PAYLOAD_INVALID", nil, err.Error(), false, nil, nil))
	}
	payload = normalizeMobilePipelineRunPayload(payload)
	if ok, current := s.claimPipeline(payload.PipelineID); !ok {
		return encodeCommand(commandResultFor("PIPELINE_ALREADY_RUNNING", nil, probeAlreadyRunningMessage, false, &current, nil))
	}
	defer s.clearPipeline(payload.PipelineID)
	result, err := s.runPipelineClaimed(payload)
	if err != nil {
		return encodeCommand(commandResultFor("PIPELINE_FAILED", result, err.Error(), false, &payload.PipelineID, result.Warnings))
	}
	return encodeCommand(commandResultFor("PIPELINE_COMPLETED", result, "策略管道已完成。", true, &payload.PipelineID, result.Warnings))
}

func (s *Service) StartPipeline(payloadJSON string) string {
	var payload pipelineRunPayload
	if err := decodeInto(payloadJSON, &payload); err != nil {
		return encodeCommand(commandResultFor("PIPELINE_PAYLOAD_INVALID", nil, err.Error(), false, nil, nil))
	}
	payload = normalizeMobilePipelineRunPayload(payload)
	if ok, current := s.claimPipeline(payload.PipelineID); !ok {
		return encodeCommand(commandResultFor("PIPELINE_ALREADY_RUNNING", nil, probeAlreadyRunningMessage, false, &current, nil))
	}
	go func() {
		defer s.clearPipeline(payload.PipelineID)
		_, _ = s.runPipelineClaimed(payload)
	}()
	return encodeCommand(commandResultFor("PIPELINE_ACCEPTED", map[string]any{
		"accepted":    true,
		"pipeline_id": payload.PipelineID,
		"task_id":     payload.TaskID,
	}, "策略管道已提交。", true, &payload.PipelineID, nil))
}

func (s *Service) CancelPipeline(payloadJSON string) string {
	payload, _ := decodeObject(payloadJSON)
	pipelineID := strings.TrimSpace(stringValue(firstNonNil(payload["pipeline_id"], payload["pipelineId"], payload["task_id"], payload["taskId"]), ""))
	s.stateMu.Lock()
	if pipelineID == "" {
		pipelineID = s.currentPipelineID
	}
	if pipelineID == "" || pipelineID != s.currentPipelineID {
		s.stateMu.Unlock()
		return encodeCommand(commandResultFor("PIPELINE_CANCEL_UNAVAILABLE", nil, "当前没有可终止的策略管道。", false, &pipelineID, nil))
	}
	s.pipelineCancel = true
	s.stateMu.Unlock()
	_ = s.CancelProbe(encodeJSON(map[string]any{"mode": "cancel"}))
	return encodeCommand(commandResultFor("PIPELINE_CANCEL_REQUESTED", nil, "已请求终止策略管道。", true, &pipelineID, nil))
}

func (s *Service) GetPipelineSnapshot(payloadJSON string) string {
	payload, _ := decodeObject(payloadJSON)
	pipelineID := strings.TrimSpace(stringValue(firstNonNil(payload["pipeline_id"], payload["pipelineId"], payload["task_id"], payload["taskId"]), ""))
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	if pipelineID == "" {
		pipelineID = s.currentPipelineID
	}
	if pipelineID != "" {
		if result, ok := s.pipelineResults[pipelineID]; ok {
			return encodeCommand(commandResultFor("PIPELINE_SNAPSHOT_READY", result, "策略管道快照已读取。", true, &pipelineID, nil))
		}
		return encodeCommand(commandResultFor("PIPELINE_SNAPSHOT_NOT_FOUND", nil, "未找到策略管道快照。", false, &pipelineID, nil))
	}
	for id, result := range s.pipelineResults {
		pipelineID = id
		return encodeCommand(commandResultFor("PIPELINE_SNAPSHOT_READY", result, "策略管道快照已读取。", true, &pipelineID, nil))
	}
	return encodeCommand(commandResultFor("PIPELINE_SNAPSHOT_NOT_FOUND", nil, "未找到策略管道快照。", false, &pipelineID, nil))
}

func (s *Service) ListPipelineResults(payloadJSON string) string {
	payload, _ := decodeObject(payloadJSON)
	pipelineID := strings.TrimSpace(stringValue(firstNonNil(payload["pipeline_id"], payload["pipelineId"], payload["task_id"], payload["taskId"]), ""))
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	if pipelineID != "" {
		if result, ok := s.pipelineResults[pipelineID]; ok {
			return encodeCommand(commandResultFor("PIPELINE_RESULTS_READY", []pipelineRunResult{result}, "策略管道结果已读取。", true, &pipelineID, nil))
		}
		return encodeCommand(commandResultFor("PIPELINE_RESULTS_READY", []pipelineRunResult{}, "策略管道结果已读取。", true, &pipelineID, nil))
	}
	for _, result := range s.pipelineResults {
		return encodeCommand(commandResultFor("PIPELINE_RESULTS_READY", []pipelineRunResult{result}, "策略管道结果已读取。", true, nil, nil))
	}
	return encodeCommand(commandResultFor("PIPELINE_RESULTS_READY", []pipelineRunResult{}, "策略管道结果已读取。", true, nil, nil))
}

func (s *Service) runPipelineClaimed(payload pipelineRunPayload) (pipelineRunResult, error) {
	start := time.Now()
	profiles, warnings, err := s.pipelineProfilesForRun(payload)
	result := pipelineRunResult{
		PipelineID: payload.PipelineID,
		StartedAt:  start.Format(time.RFC3339),
		Status:     "running",
		TaskID:     payload.TaskID,
		Total:      len(profiles),
		Warnings:   warnings,
	}
	if err != nil {
		result.Status = "failed"
		result.CompletedAt = nowRFC3339()
		result.DurationMS = time.Since(start).Milliseconds()
		result.Warnings = dedupeStrings(append(result.Warnings, err.Error()))
		s.rememberPipelineResult(result)
		s.emit(payload.PipelineID, "pipeline.failed", map[string]any{"message": err.Error(), "pipeline_id": payload.PipelineID})
		return result, err
	}
	s.emit(payload.PipelineID, "pipeline.started", map[string]any{"pipeline_id": payload.PipelineID, "task_id": payload.TaskID, "total": len(profiles)})
	for index, profile := range profiles {
		if s.isPipelineCancelRequested(payload.PipelineID) {
			result.Skipped += len(profiles) - index
			break
		}
		profileResult := s.runPipelineProfile(payload, profile, index)
		result.Results = append(result.Results, profileResult)
		switch profileResult.Status {
		case "completed", "dns_failed":
			result.Succeeded++
		case "skipped":
			result.Skipped++
		default:
			result.Failed++
		}
	}
	result.CompletedAt = nowRFC3339()
	result.DurationMS = time.Since(start).Milliseconds()
	switch {
	case s.isPipelineCancelRequested(payload.PipelineID):
		result.Status = "cancelled"
	case result.Failed > 0 && result.Succeeded == 0:
		result.Status = "failed"
	case result.Failed > 0 || result.Skipped > 0:
		result.Status = "partial"
	default:
		result.Status = "completed"
	}
	s.rememberPipelineResult(result)
	s.emit(payload.PipelineID, "pipeline.completed", map[string]any{
		"completed_at": result.CompletedAt,
		"duration_ms":  result.DurationMS,
		"failed":       result.Failed,
		"pipeline_id":  result.PipelineID,
		"skipped":      result.Skipped,
		"status":       result.Status,
		"succeeded":    result.Succeeded,
		"total":        result.Total,
	})
	if result.Failed > 0 {
		return result, errors.New("策略管道部分策略执行失败")
	}
	return result, nil
}

func (s *Service) runPipelineProfile(payload pipelineRunPayload, profile pipelineProfile, index int) appcore.PipelineProfileRunResult {
	profileTaskID := mobilePipelineProfileTaskID(payload.PipelineID, profile, index)
	baseResult := appcore.PipelineProfileRunResult{
		Domain:      profile.Domain,
		ProfileID:   profile.ID,
		ProfileName: profile.Name,
		Region:      profile.Region,
		Status:      "running",
		TaskID:      profileTaskID,
	}
	if !profile.Enabled {
		baseResult.Status = "skipped"
		baseResult.Message = "策略未启用，已跳过。"
		s.emit(payload.PipelineID, "pipeline.profile_skipped", mobilePipelineProfileEventPayload(profile, profileTaskID, map[string]any{"message": baseResult.Message, "pipeline_id": payload.PipelineID}))
		return baseResult
	}
	snapshot := mobilePipelineSnapshotForRun(profile)
	s.emit(payload.PipelineID, "pipeline.profile_started", mobilePipelineProfileEventPayload(profile, profileTaskID, map[string]any{"index": index, "pipeline_id": payload.PipelineID}))
	response := s.RunProbe(encodeJSON(desktopProbePayload{
		Config:            snapshot,
		ConfigSource:      mobileFirstNonEmpty(payload.ConfigSource, "pipeline"),
		PipelineDomain:    profile.Domain,
		PipelineID:        payload.PipelineID,
		PipelineProfile:   profile.Name,
		PipelineProfileID: profile.ID,
		PipelineRegion:    profile.Region,
		Sources:           mobileSourcesFromAny(snapshot["sources"]),
		TaskID:            profileTaskID,
	}))
	command, probeResult := mobilePipelineProbeCommand(response)
	if !command.OK {
		baseResult.Status = "failed"
		baseResult.Message = mobileFirstNonEmpty(command.Message, "策略探测失败。")
		baseResult.Warnings = command.Warnings
		s.emit(payload.PipelineID, "pipeline.profile_failed", mobilePipelineProfileEventPayload(profile, profileTaskID, map[string]any{"message": baseResult.Message, "pipeline_id": payload.PipelineID}))
		return baseResult
	}
	baseResult.ProbeResult = &probeResult
	baseResult.Status = "completed"
	baseResult.Message = fmt.Sprintf("策略完成，可用结果 %d 条。", len(probeResult.Results))
	baseResult.Warnings = probeResult.Warnings
	if appcore.PipelineDNSPushEnabled(profile.DNSPushPolicy) && len(probeResult.Results) > 0 {
		dnsRows, dnsWarnings, dnsSelectErr := mobilePipelineDNSRows(snapshot, probeResult.Results, probeResult.Config.DownloadSpeedMetric)
		baseResult.Warnings = dedupeStrings(append(baseResult.Warnings, dnsWarnings...))
		if dnsSelectErr != nil {
			baseResult.Status = "dns_failed"
			baseResult.Warnings = dedupeStrings(append(baseResult.Warnings, dnsSelectErr.Error()))
			baseResult.DNSResult = commandResult{
				Code:    "DNS_CONFIG_INVALID",
				Message: dnsSelectErr.Error(),
				OK:      false,
			}
			s.emit(payload.PipelineID, "pipeline.profile_completed", mobilePipelineProfileEventPayload(profile, profileTaskID, map[string]any{
				"dns_result":   baseResult.DNSResult,
				"pipeline_id":  payload.PipelineID,
				"result_count": len(probeResult.Results),
				"status":       baseResult.Status,
			}))
			return baseResult
		}
		dnsCommand := mobilePipelineDNSCommand(s.PushCloudflareDNSRecords(encodeJSON(map[string]any{
			"config": snapshot,
			"ipsRaw": mobileProbeRowsIPList(dnsRows),
		})))
		baseResult.DNSResult = dnsCommand
		if !dnsCommand.OK {
			baseResult.Status = "dns_failed"
			baseResult.Warnings = dedupeStrings(append(baseResult.Warnings, dnsCommand.Message))
		}
	}
	s.emit(payload.PipelineID, "pipeline.profile_completed", mobilePipelineProfileEventPayload(profile, profileTaskID, map[string]any{
		"dns_result":   baseResult.DNSResult,
		"pipeline_id":  payload.PipelineID,
		"result_count": len(probeResult.Results),
		"status":       baseResult.Status,
	}))
	return baseResult
}

func (s *Service) pipelineProfilesForRun(payload pipelineRunPayload) ([]pipelineProfile, []string, error) {
	warnings := []string{}
	storeProfiles := payload.Profiles
	if len(storeProfiles) == 0 && (len(payload.TargetIDs) > 0 || len(payload.Workspace.Targets) > 0 || strings.TrimSpace(payload.TemplateID) != "") {
		workspace, workspaceWarnings, err := s.pipelineWorkspaceForRunPayload(payload)
		warnings = append(warnings, workspaceWarnings...)
		if err != nil {
			return nil, warnings, err
		}
		storeProfiles = s.pipelineProfilesFromWorkspaceSelection(workspace, payload.TemplateID, payload.TargetIDs)
	}
	if len(storeProfiles) == 0 {
		store, storeWarnings, err := s.loadPipelineProfileStoreOrDefault()
		warnings = append(warnings, storeWarnings...)
		if err != nil {
			return nil, warnings, err
		}
		storeProfiles = store.Items
	}
	selectedIDs := map[string]struct{}{}
	for _, id := range payload.ProfileIDs {
		if id = strings.TrimSpace(id); id != "" {
			selectedIDs[id] = struct{}{}
		}
	}
	profiles := make([]pipelineProfile, 0, len(storeProfiles))
	for _, profile := range storeProfiles {
		if len(selectedIDs) > 0 {
			if _, ok := selectedIDs[profile.ID]; !ok {
				continue
			}
		}
		profiles = append(profiles, profile)
	}
	if len(profiles) == 0 {
		return nil, warnings, errors.New("策略管道没有可执行的策略")
	}
	return profiles, warnings, nil
}

func (s *Service) loadPipelineProfileStoreOrDefault() (mobilePipelineProfileStore, []string, error) {
	workspace, warnings, err := s.loadPipelineWorkspaceOrDefault()
	if err != nil {
		return mobilePipelineProfileStore{}, warnings, err
	}
	store := s.pipelineProfileStoreFromWorkspace(workspace)
	if len(store.Items) > 0 {
		return s.normalizePipelineProfileStoreForSave(store), warnings, nil
	}
	snapshot, err := s.loadConfigSnapshotFromDisk()
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return store, warnings, err
		}
		snapshot = defaultConfigSnapshot()
	}
	return s.defaultPipelineProfileStoreFromSnapshot(snapshot), warnings, nil
}

func (s *Service) defaultPipelineProfileStoreFromSnapshot(snapshot map[string]any) mobilePipelineProfileStore {
	return appcore.DefaultPipelineProfileStoreFromSnapshot(snapshot, pipelineProfilesSchemaVersion, nowRFC3339(), sanitizeMobileConfigSnapshot)
}

func (s *Service) normalizePipelineProfileStoreForSave(store mobilePipelineProfileStore) mobilePipelineProfileStore {
	return appcore.NormalizePipelineProfileStoreForSave(store, pipelineProfilesSchemaVersion, nowRFC3339(), sanitizeMobileConfigSnapshot, func(index int) string {
		return fmt.Sprintf("pipeline-profile-%d", time.Now().UnixNano()+int64(index))
	})
}

func normalizeMobilePipelineRunPayload(payload pipelineRunPayload) pipelineRunPayload {
	payload.PipelineID = strings.TrimSpace(payload.PipelineID)
	if payload.PipelineID == "" {
		payload.PipelineID = strings.TrimSpace(payload.TaskID)
	}
	if payload.PipelineID == "" {
		payload.PipelineID = fmt.Sprintf("pipeline-%d", time.Now().UnixNano())
	}
	payload.TaskID = strings.TrimSpace(payload.TaskID)
	if payload.TaskID == "" {
		payload.TaskID = payload.PipelineID
	}
	if payload.TargetIDs == nil {
		payload.TargetIDs = []string{}
	}
	if payload.ProfileIDs == nil {
		payload.ProfileIDs = []string{}
	}
	payload.TemplateID = strings.TrimSpace(payload.TemplateID)
	return payload
}

func mobilePipelineProfileFromPayload(payload map[string]any) pipelineProfile {
	rawProfile := mapValue(firstNonNil(payload["profile"], payload["item"]))
	if len(rawProfile) == 0 {
		rawProfile = payload
	}
	profiles := appcore.PipelineProfilesFromAny([]any{rawProfile})
	if len(profiles) == 0 {
		return pipelineProfile{}
	}
	return profiles[0]
}

func mobilePipelinePayloadHasEnabled(payload map[string]any) bool {
	if _, ok := payload["enabled"]; ok {
		return true
	}
	profile := mapValue(firstNonNil(payload["profile"], payload["item"]))
	_, ok := profile["enabled"]
	return ok
}

func mobilePipelineTargetPayloadHasEnabled(payload map[string]any) bool {
	if _, ok := payload["enabled"]; ok {
		return true
	}
	target := mapValue(firstNonNil(payload["target"], payload["item"], payload["profile"]))
	_, ok := target["enabled"]
	return ok
}

func (s *Service) loadPipelineWorkspaceOrDefault() (pipelineWorkspace, []string, error) {
	workspace, migrated, err := s.loadPipelineWorkspace()
	if err != nil {
		return workspace, nil, err
	}
	if len(workspace.Targets) > 0 && len(workspace.Templates) > 0 {
		workspace = s.normalizePipelineWorkspaceForSave(workspace)
		if migrated {
			if err := s.savePipelineWorkspace(workspace); err != nil {
				return workspace, []string{"已识别旧版策略数据，但写入新工作流文件失败。"}, nil
			}
			return workspace, []string{"已从 pipeline-profiles.json 自动迁移到 pipeline-workspace.json。"}, nil
		}
		return workspace, nil, nil
	}
	snapshot, snapshotErr := s.loadConfigSnapshotFromDisk()
	if snapshotErr != nil {
		if !errors.Is(snapshotErr, os.ErrNotExist) {
			return workspace, nil, snapshotErr
		}
		snapshot = defaultConfigSnapshot()
	}
	return s.defaultPipelineWorkspaceFromSnapshot(snapshot), nil, nil
}

func (s *Service) defaultPipelineWorkspaceFromSnapshot(snapshot map[string]any) pipelineWorkspace {
	return appcore.DefaultPipelineWorkspaceFromSnapshot(snapshot, pipelineWorkspaceSchemaVersion, nowRFC3339(), sanitizeMobileConfigSnapshot)
}

func (s *Service) normalizePipelineWorkspaceForSave(workspace pipelineWorkspace) pipelineWorkspace {
	return appcore.NormalizePipelineWorkspaceForSave(workspace, pipelineWorkspaceSchemaVersion, nowRFC3339(), sanitizeMobileConfigSnapshot, func(index int) string {
		return fmt.Sprintf("pipeline-template-%d", time.Now().UnixNano()+int64(index))
	}, func(index int) string {
		return fmt.Sprintf("pipeline-target-%d", time.Now().UnixNano()+int64(index))
	})
}

func (s *Service) pipelineProfileStoreFromWorkspace(workspace pipelineWorkspace) mobilePipelineProfileStore {
	return appcore.LegacyPipelineProfileStoreFromWorkspace(workspace, pipelineProfilesSchemaVersion, nowRFC3339(), sanitizeMobileConfigSnapshot)
}

func (s *Service) applyLegacyProfileStoreToWorkspace(workspace pipelineWorkspace, store mobilePipelineProfileStore) pipelineWorkspace {
	workspace = s.normalizePipelineWorkspaceForSave(workspace)
	next := appcore.PipelineWorkspaceFromProfileStore(store, pipelineWorkspaceSchemaVersion, nowRFC3339(), sanitizeMobileConfigSnapshot)
	if len(workspace.Templates) > 0 {
		next.Templates = workspace.Templates
		next.ActiveTemplateID = mobileFirstNonEmpty(workspace.ActiveTemplateID, next.ActiveTemplateID)
	}
	existingTargets := make(map[string]pipelineTarget, len(workspace.Targets))
	for _, item := range workspace.Targets {
		existingTargets[item.ID] = item
	}
	for index := range next.Targets {
		if existing, ok := existingTargets[next.Targets[index].ID]; ok {
			next.Targets[index].TemplateID = mobileFirstNonEmpty(existing.TemplateID, next.Targets[index].TemplateID, next.ActiveTemplateID, appcore.DefaultPipelineTemplateID)
			next.Targets[index].Tags = append([]string{}, existing.Tags...)
		} else if strings.TrimSpace(next.Targets[index].TemplateID) == "" {
			next.Targets[index].TemplateID = mobileFirstNonEmpty(next.ActiveTemplateID, appcore.DefaultPipelineTemplateID)
		}
	}
	if strings.TrimSpace(store.ActiveProfileID) != "" {
		next.ActiveTargetID = strings.TrimSpace(store.ActiveProfileID)
	}
	return s.normalizePipelineWorkspaceForSave(next)
}

func mobilePipelineWorkspaceFromPayload(payload map[string]any) pipelineWorkspace {
	rawWorkspace := firstNonNil(payload["workspace"], payload["pipeline_workspace"], payload["pipelineWorkspace"])
	workspace := appcore.PipelineWorkspaceFromAny(rawWorkspace)
	if len(workspace.Templates) == 0 && len(workspace.Targets) == 0 {
		workspace.Templates = appcore.PipelineTemplatesFromAny(firstNonNil(payload["templates"], payload["pipeline_templates"], payload["pipelineTemplates"]))
		workspace.Targets = appcore.PipelineTargetsFromAny(firstNonNil(payload["targets"], payload["pipeline_targets"], payload["pipelineTargets"]))
		workspace.ActiveTemplateID = strings.TrimSpace(stringValue(firstNonNil(payload["active_template_id"], payload["activeTemplateId"]), ""))
		workspace.ActiveTargetID = strings.TrimSpace(stringValue(firstNonNil(payload["active_target_id"], payload["activeTargetId"]), ""))
		workspace.SchemaVersion = strings.TrimSpace(stringValue(firstNonNil(payload["schema_version"], payload["schemaVersion"]), ""))
		workspace.UpdatedAt = strings.TrimSpace(stringValue(firstNonNil(payload["updated_at"], payload["updatedAt"]), ""))
	}
	return workspace
}

func mobilePipelineWorkspaceFromAny(value any) pipelineWorkspace {
	return appcore.PipelineWorkspaceFromAny(value)
}

func mobilePipelineTemplateFromPayload(payload map[string]any) pipelineTemplate {
	rawTemplate := mapValue(firstNonNil(payload["template"], payload["item"]))
	if len(rawTemplate) == 0 {
		rawTemplate = payload
	}
	templates := appcore.PipelineTemplatesFromAny([]any{rawTemplate})
	if len(templates) == 0 {
		return pipelineTemplate{}
	}
	return templates[0]
}

func mobilePipelineTargetFromPayload(payload map[string]any) pipelineTarget {
	rawTarget := mapValue(firstNonNil(payload["target"], payload["item"]))
	if len(rawTarget) == 0 {
		rawTarget = payload
	}
	targets := appcore.PipelineTargetsFromAny([]any{rawTarget})
	if len(targets) == 0 {
		profile := mobilePipelineProfileFromPayload(payload)
		return pipelineTarget{
			ConfigSnapshot: profile.ConfigSnapshot,
			CreatedAt:      profile.CreatedAt,
			DNSPushPolicy:  profile.DNSPushPolicy,
			Domain:         profile.Domain,
			Enabled:        profile.Enabled,
			ID:             profile.ID,
			Name:           profile.Name,
			Region:         profile.Region,
			TemplateID:     mobileFirstNonEmpty(strings.TrimSpace(stringValue(firstNonNil(payload["template_id"], payload["templateId"]), "")), appcore.DefaultPipelineTemplateID),
			UpdatedAt:      profile.UpdatedAt,
		}
	}
	return targets[0]
}

func (s *Service) pipelineWorkspaceForRunPayload(payload pipelineRunPayload) (pipelineWorkspace, []string, error) {
	if len(payload.Workspace.Templates) > 0 || len(payload.Workspace.Targets) > 0 {
		return s.normalizePipelineWorkspaceForSave(payload.Workspace), nil, nil
	}
	workspace, warnings, err := s.loadPipelineWorkspaceOrDefault()
	if err != nil {
		return pipelineWorkspace{}, warnings, err
	}
	return workspace, warnings, nil
}

func (s *Service) pipelineProfilesFromWorkspaceSelection(workspace pipelineWorkspace, templateID string, targetIDs []string) []pipelineProfile {
	selectedIDs := make(map[string]struct{}, len(targetIDs))
	for _, id := range targetIDs {
		id = strings.TrimSpace(id)
		if id != "" {
			selectedIDs[id] = struct{}{}
		}
	}
	templateID = strings.TrimSpace(templateID)
	profiles := make([]pipelineProfile, 0, len(workspace.Targets))
	for _, target := range workspace.Targets {
		if len(selectedIDs) > 0 {
			if _, ok := selectedIDs[target.ID]; !ok {
				continue
			}
		}
		if templateID != "" && strings.TrimSpace(target.TemplateID) != templateID {
			continue
		}
		profiles = append(profiles, pipelineProfile{
			ConfigSnapshot: mobileDeepCloneMap(target.ConfigSnapshot),
			CreatedAt:      target.CreatedAt,
			DNSPushPolicy:  target.DNSPushPolicy,
			Domain:         target.Domain,
			Enabled:        target.Enabled,
			ID:             target.ID,
			Name:           target.Name,
			Region:         target.Region,
			UpdatedAt:      target.UpdatedAt,
		})
	}
	return profiles
}

func mobilePipelineSnapshotForRun(profile pipelineProfile) map[string]any {
	snapshot := sanitizeMobileConfigSnapshot(mobileDeepCloneMap(profile.ConfigSnapshot))
	exportCfg := mapValue(snapshot["export"])
	if strings.TrimSpace(stringValue(firstNonNil(exportCfg["file_name_template"], exportCfg["fileNameTemplate"]), "")) == "" {
		exportCfg["file_name_template"] = "result-{profile}-{task_id}.csv"
	}
	snapshot["export"] = exportCfg
	if strings.TrimSpace(profile.Domain) != "" {
		cloudflare := mapValue(snapshot["cloudflare"])
		cloudflare["record_name"] = strings.TrimSpace(profile.Domain)
		snapshot["cloudflare"] = cloudflare
	}
	return snapshot
}

func mobilePipelineProfileTaskID(pipelineID string, profile pipelineProfile, index int) string {
	safeID := probecore.SanitizeTemplateFileName(profile.ID)
	if safeID == "" {
		safeID = fmt.Sprintf("profile-%d", index+1)
	}
	return fmt.Sprintf("%s-%02d-%s", probecore.SanitizeTemplateFileName(pipelineID), index+1, safeID)
}

func mobilePipelineProbeMetadata(payload desktopProbePayload) map[string]any {
	metadata := map[string]any{}
	if value := strings.TrimSpace(payload.PipelineID); value != "" {
		metadata["pipeline_id"] = value
	}
	if value := strings.TrimSpace(payload.PipelineProfileID); value != "" {
		metadata["profile_id"] = value
		metadata["pipeline_profile_id"] = value
	}
	if value := strings.TrimSpace(payload.PipelineProfile); value != "" {
		metadata["profile_name"] = value
		metadata["pipeline_profile_name"] = value
	}
	if value := strings.TrimSpace(payload.PipelineDomain); value != "" {
		metadata["domain"] = value
		metadata["pipeline_domain"] = value
	}
	if value := strings.TrimSpace(payload.PipelineRegion); value != "" {
		metadata["region"] = value
		metadata["pipeline_region"] = value
	}
	return metadata
}

func mobilePipelineProfileEventPayload(profile pipelineProfile, taskID string, extra map[string]any) map[string]any {
	payload := map[string]any{
		"domain":                profile.Domain,
		"pipeline_domain":       profile.Domain,
		"pipeline_profile_id":   profile.ID,
		"pipeline_profile_name": profile.Name,
		"pipeline_region":       profile.Region,
		"profile_id":            profile.ID,
		"profile_name":          profile.Name,
		"region":                profile.Region,
		"task_id":               taskID,
	}
	for key, value := range extra {
		payload[key] = value
	}
	return payload
}

func (s *Service) claimPipeline(pipelineID string) (bool, string) {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	if s.currentPipelineID != "" {
		return false, s.currentPipelineID
	}
	if s.currentTaskID != "" || s.pausedTaskID != "" || s.pauseRequested {
		return false, mobileFirstNonEmpty(s.currentTaskID, s.pausedTaskID)
	}
	s.currentPipelineID = pipelineID
	s.pipelineCancel = false
	s.pipelineResults = map[string]appcore.PipelineRunResult{}
	return true, pipelineID
}

func (s *Service) clearPipeline(pipelineID string) {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	if s.currentPipelineID == pipelineID {
		s.currentPipelineID = ""
		s.pipelineCancel = false
	}
}

func (s *Service) isPipelineCancelRequested(pipelineID string) bool {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	return s.currentPipelineID == pipelineID && s.pipelineCancel
}

func (s *Service) rememberPipelineResult(result pipelineRunResult) {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	s.pipelineResults = map[string]appcore.PipelineRunResult{
		result.PipelineID: result,
	}
}

func (s *Service) setTaskEventMetadata(taskID string, metadata map[string]any) {
	if len(metadata) == 0 {
		return
	}
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	if s.taskEventMetadata == nil {
		s.taskEventMetadata = map[string]map[string]any{}
	}
	s.taskEventMetadata[taskID] = metadata
}

func (s *Service) clearTaskEventMetadata(taskID string) {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	delete(s.taskEventMetadata, taskID)
}

func (s *Service) taskEventMetadataFor(taskID string) map[string]any {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	metadata := s.taskEventMetadata[taskID]
	if len(metadata) == 0 {
		return nil
	}
	cloned := make(map[string]any, len(metadata))
	for key, value := range metadata {
		cloned[key] = value
	}
	return cloned
}

func mobilePipelineProbeCommand(response string) (commandResult, probeRunResult) {
	var command commandResult
	if err := json.Unmarshal([]byte(response), &command); err != nil {
		return commandResult{Code: "PIPELINE_PROBE_DECODE_FAILED", Message: err.Error(), OK: false}, probeRunResult{}
	}
	var result probeRunResult
	raw, err := json.Marshal(command.Data)
	if err == nil {
		_ = json.Unmarshal(raw, &result)
	}
	return command, result
}

func mobilePipelineDNSCommand(response string) commandResult {
	var command commandResult
	if err := json.Unmarshal([]byte(response), &command); err != nil {
		return commandResult{Code: "PIPELINE_DNS_DECODE_FAILED", Message: err.Error(), OK: false}
	}
	return command
}

func decodeCommandResult(response string) commandResult {
	var command commandResult
	if err := json.Unmarshal([]byte(response), &command); err != nil {
		return commandResult{Code: "PIPELINE_DECODE_FAILED", Message: err.Error(), OK: false}
	}
	return command
}

func mobilePipelineDNSRows(snapshot map[string]any, rows []probeRow, metric string) ([]probeRow, []string, error) {
	selection, err := appcore.BuildUploadSelection(snapshot, rows, metric)
	if err != nil {
		return nil, nil, err
	}
	recordType := stringValue(mapValue(snapshot["cloudflare"])["record_type"], cloudflareRecordTypeA)
	filtered := appcore.FilterRowsForCloudflareRecordType(selection.CloudflareRows, recordType)
	return filtered, selection.Warnings, nil
}

func mobileProbeRowsIPList(rows []probeRow) string {
	values := make([]string, 0, len(rows))
	for _, row := range rows {
		if ip := strings.TrimSpace(row.IP); ip != "" {
			values = append(values, ip)
		}
	}
	return strings.Join(values, "\n")
}

func mobileFirstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func mobilePipelineDomainFromSnapshot(snapshot map[string]any) string {
	cloudflare := mapValue(snapshot["cloudflare"])
	return strings.TrimSpace(stringValue(firstNonNil(cloudflare["record_name"], cloudflare["recordName"]), ""))
}

func mobileDeepCloneMap(input map[string]any) map[string]any {
	if input == nil {
		return map[string]any{}
	}
	raw, err := json.Marshal(input)
	if err != nil {
		return map[string]any{}
	}
	var output map[string]any
	if err := json.Unmarshal(raw, &output); err != nil {
		return map[string]any{}
	}
	return output
}
