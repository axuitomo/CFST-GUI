package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/appcore"
	"github.com/axuitomo/CFST-GUI/internal/probecore"
	"github.com/axuitomo/CFST-GUI/internal/task"
	"github.com/axuitomo/CFST-GUI/internal/utils"
)

type pipelineEventEmitter struct {
	app        *App
	pipelineID string
	seq        int
}

type pipelineRuntimeContext struct {
	ConfigSnapshot      map[string]any
	DNSResult           any
	FilteredRows        []ProbeRow
	LastUploadSelection *UploadSelectionResult
	NodeOutputs         map[string]any
	Payload             PipelineRunPayload
	ProbeStage          *pipelineProbeStageState
	ProbeStageSnapshot  map[string]any
	ProbeResult         *ProbeRunResult
	Profile             PipelineProfile
	SchedulerOverrides  appcore.PipelineRuntimeOverrides
	SelectedSources     []DesktopSource
	TaskID              string
	Target              PipelineTarget
	Template            pipelineTemplateItem
	Warnings            []string
}

type pipelineProbeStageState struct {
	CompletedStages []string
	Config          ProbeConfig
	ConfigWarnings  []string
	Prepared        preparedDesktopSources
	Source          SourceSummary
	SourcePorts     map[string]int
	StartedAt       time.Time
	TaskContext     ProbeTaskContext
	TCPData         utils.PingDelaySet
	TestPorts       map[string]int
	TraceData       utils.PingDelaySet
	Warnings        []string
}

type pipelineNodeExecutionResult struct {
	Message       string
	Metrics       map[string]any
	Outcome       string
	Output        any
	OutputSummary string
	Status        string
}

type pipelineNodeExecutor func(node appcore.PipelineNode, runtimeCtx *pipelineRuntimeContext) (pipelineNodeExecutionResult, error)

func (a *App) LoadPipelineWorkspace() DesktopCommandResult {
	workspace, warnings, err := loadPipelineWorkspaceOrDefault()
	if err != nil {
		return desktopCommandResult("PIPELINE_WORKSPACE_LOAD_FAILED", nil, err.Error(), false, nil, warnings)
	}
	return desktopCommandResult("PIPELINE_WORKSPACE_LOAD_OK", workspace, "策略工作流已加载。", true, nil, warnings)
}

func (a *App) LoadPipelineNodeCatalog() DesktopCommandResult {
	return desktopCommandResult("PIPELINE_NODE_CATALOG_OK", appcore.DefaultPipelineNodeCatalog(), "工作流节点目录已加载。", true, nil, nil)
}

func (a *App) SavePipelineWorkspace(payload map[string]any) DesktopCommandResult {
	workspace := pipelineWorkspaceFromPayload(payload)
	if len(workspace.Templates) == 0 && len(workspace.Targets) == 0 {
		current, err := loadDesktopConfigSnapshotFromDisk()
		if err != nil {
			return desktopCommandResult("PIPELINE_WORKSPACE_INVALID", nil, err.Error(), false, nil, nil)
		}
		workspace = defaultPipelineWorkspaceFromSnapshot(current)
	}
	workspace = normalizePipelineWorkspaceForSave(workspace)
	if err := appcore.ValidatePipelineWorkspace(workspace); err != nil {
		return desktopCommandResult("PIPELINE_WORKSPACE_INVALID", nil, err.Error(), false, nil, nil)
	}
	if err := savePipelineWorkspace(workspace); err != nil {
		return desktopCommandResult("PIPELINE_WORKSPACE_SAVE_FAILED", nil, err.Error(), false, nil, nil)
	}
	return desktopCommandResult("PIPELINE_WORKSPACE_SAVE_OK", workspace, "策略工作流已保存。", true, nil, nil)
}

func (a *App) SavePipelineTemplate(payload map[string]any) DesktopCommandResult {
	workspace, warnings, err := loadPipelineWorkspaceOrDefault()
	if err != nil {
		return desktopCommandResult("PIPELINE_WORKSPACE_LOAD_FAILED", nil, err.Error(), false, nil, warnings)
	}
	template := pipelineTemplateFromPayload(payload)
	if strings.TrimSpace(template.ID) == "" {
		template.ID = fmt.Sprintf("pipeline-template-%d", time.Now().UnixNano())
	}
	now := time.Now().Format(time.RFC3339)
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
	workspace = normalizePipelineWorkspaceForSave(workspace)
	if err := appcore.ValidatePipelineWorkspace(workspace); err != nil {
		return desktopCommandResult("PIPELINE_WORKSPACE_INVALID", nil, err.Error(), false, nil, warnings)
	}
	if err := savePipelineWorkspace(workspace); err != nil {
		return desktopCommandResult("PIPELINE_WORKSPACE_SAVE_FAILED", nil, err.Error(), false, nil, warnings)
	}
	return desktopCommandResult("PIPELINE_WORKSPACE_SAVE_OK", workspace, "工作流模板已保存。", true, nil, warnings)
}

func (a *App) DeletePipelineTemplate(payload map[string]any) DesktopCommandResult {
	templateID := strings.TrimSpace(stringValue(firstNonNil(payload["template_id"], payload["templateId"], payload["id"]), ""))
	if templateID == "" {
		return desktopCommandResult("PIPELINE_WORKSPACE_INVALID", nil, "缺少 template_id。", false, nil, nil)
	}
	if templateID == appcore.DefaultPipelineTemplateID || templateID == appcore.AdvancedUploadPipelineTemplateID {
		return desktopCommandResult("PIPELINE_WORKSPACE_INVALID", nil, "内置工作流不能删除。", false, nil, nil)
	}
	workspace, warnings, err := loadPipelineWorkspaceOrDefault()
	if err != nil {
		return desktopCommandResult("PIPELINE_WORKSPACE_LOAD_FAILED", nil, err.Error(), false, nil, warnings)
	}
	nextItems := make([]pipelineTemplateItem, 0, len(workspace.Templates))
	deleted := false
	for _, item := range workspace.Templates {
		if item.ID == templateID {
			deleted = true
			continue
		}
		nextItems = append(nextItems, item)
	}
	if !deleted {
		return desktopCommandResult("PIPELINE_WORKSPACE_NOT_FOUND", nil, "未找到工作流模板。", false, nil, warnings)
	}
	workspace.Templates = nextItems
	if workspace.ActiveTemplateID == templateID {
		workspace.ActiveTemplateID = ""
	}
	for index := range workspace.Targets {
		if workspace.Targets[index].TemplateID == templateID {
			workspace.Targets[index].TemplateID = firstNonEmptyString(workspace.ActiveTemplateID, appcore.DefaultPipelineTemplateID)
		}
	}
	workspace = normalizePipelineWorkspaceForSave(workspace)
	if err := appcore.ValidatePipelineWorkspace(workspace); err != nil {
		return desktopCommandResult("PIPELINE_WORKSPACE_INVALID", nil, err.Error(), false, nil, warnings)
	}
	if err := savePipelineWorkspace(workspace); err != nil {
		return desktopCommandResult("PIPELINE_WORKSPACE_SAVE_FAILED", nil, err.Error(), false, nil, warnings)
	}
	return desktopCommandResult("PIPELINE_WORKSPACE_SAVE_OK", workspace, "工作流模板已删除。", true, nil, warnings)
}

func (a *App) SavePipelineTarget(payload map[string]any) DesktopCommandResult {
	workspace, warnings, err := loadPipelineWorkspaceOrDefault()
	if err != nil {
		return desktopCommandResult("PIPELINE_WORKSPACE_LOAD_FAILED", nil, err.Error(), false, nil, warnings)
	}
	target := pipelineTargetFromPayload(payload)
	if len(target.ConfigSnapshot) == 0 {
		snapshot := mapValue(firstNonNil(payload["config_snapshot"], payload["configSnapshot"]))
		if len(snapshot) == 0 {
			snapshot, err = loadDesktopConfigSnapshotFromDisk()
			if err != nil {
				return desktopCommandResult("PIPELINE_WORKSPACE_INVALID", nil, err.Error(), false, nil, warnings)
			}
		}
		target.ConfigSnapshot = sanitizeDesktopConfigSnapshot(snapshot)
	}
	if strings.TrimSpace(target.ID) == "" {
		target.ID = fmt.Sprintf("pipeline-target-%d", time.Now().UnixNano())
	}
	now := time.Now().Format(time.RFC3339)
	target.UpdatedAt = now
	if strings.TrimSpace(target.CreatedAt) == "" {
		target.CreatedAt = now
	}
	if strings.TrimSpace(target.Name) == "" {
		target.Name = "目标"
	}
	if strings.TrimSpace(target.Domain) == "" {
		target.Domain = pipelineDomainFromSnapshot(target.ConfigSnapshot)
	}
	if strings.TrimSpace(target.Region) == "" {
		target.Region = "未分组"
	}
	if strings.TrimSpace(target.TemplateID) == "" {
		target.TemplateID = firstNonEmptyString(workspace.ActiveTemplateID, appcore.DefaultPipelineTemplateID)
	}
	target.DNSPushPolicy = appcore.NormalizePipelineDNSPushPolicy(target.DNSPushPolicy)
	target.Enabled = true
	target.ConfigSnapshot = sanitizeDesktopConfigSnapshot(deepCloneMap(target.ConfigSnapshot))
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
		workspace.ActiveTemplateID = target.TemplateID
	}
	templateSynced := false
	for index := range workspace.Templates {
		if strings.TrimSpace(workspace.Templates[index].ID) != strings.TrimSpace(target.TemplateID) {
			continue
		}
		workspace.Templates[index].BoundConfigSnapshot = deepCloneMap(target.ConfigSnapshot)
		workspace.Templates[index].UpdatedAt = now
		templateSynced = true
		break
	}
	if !templateSynced {
		template := appcore.DefaultPipelineTemplate(now)
		template.ID = firstNonEmptyString(strings.TrimSpace(target.TemplateID), template.ID)
		template.BoundConfigSnapshot = deepCloneMap(target.ConfigSnapshot)
		template.UpdatedAt = now
		workspace.Templates = append(workspace.Templates, template)
	}
	workspace = normalizePipelineWorkspaceForSave(workspace)
	if err := appcore.ValidatePipelineWorkspace(workspace); err != nil {
		return desktopCommandResult("PIPELINE_WORKSPACE_INVALID", nil, err.Error(), false, nil, warnings)
	}
	if err := savePipelineWorkspace(workspace); err != nil {
		return desktopCommandResult("PIPELINE_WORKSPACE_SAVE_FAILED", nil, err.Error(), false, nil, warnings)
	}
	return desktopCommandResult("PIPELINE_WORKSPACE_SAVE_OK", workspace, "工作流目标已保存。", true, nil, warnings)
}

func (a *App) DeletePipelineTarget(payload map[string]any) DesktopCommandResult {
	targetID := strings.TrimSpace(stringValue(firstNonNil(payload["target_id"], payload["targetId"], payload["profile_id"], payload["profileId"], payload["id"]), ""))
	if targetID == "" {
		return desktopCommandResult("PIPELINE_WORKSPACE_INVALID", nil, "缺少 target_id。", false, nil, nil)
	}
	workspace, warnings, err := loadPipelineWorkspaceOrDefault()
	if err != nil {
		return desktopCommandResult("PIPELINE_WORKSPACE_LOAD_FAILED", nil, err.Error(), false, nil, warnings)
	}
	nextItems := make([]pipelineTargetItem, 0, len(workspace.Targets))
	deleted := false
	deletedTemplateID := ""
	now := time.Now().Format(time.RFC3339)
	for _, item := range workspace.Targets {
		if item.ID == targetID {
			deleted = true
			deletedTemplateID = strings.TrimSpace(item.TemplateID)
			continue
		}
		nextItems = append(nextItems, item)
	}
	if !deleted {
		return desktopCommandResult("PIPELINE_WORKSPACE_NOT_FOUND", nil, "未找到工作流目标。", false, nil, warnings)
	}
	workspace.Targets = nextItems
	if workspace.ActiveTargetID == targetID {
		workspace.ActiveTargetID = ""
	}
	if deletedTemplateID != "" {
		for index := range workspace.Templates {
			if strings.TrimSpace(workspace.Templates[index].ID) != deletedTemplateID {
				continue
			}
			workspace.Templates[index].BoundConfigSnapshot = map[string]any{}
			workspace.Templates[index].UpdatedAt = now
			break
		}
	}
	workspace = normalizePipelineWorkspaceForSave(workspace)
	if err := appcore.ValidatePipelineWorkspace(workspace); err != nil {
		return desktopCommandResult("PIPELINE_WORKSPACE_INVALID", nil, err.Error(), false, nil, warnings)
	}
	if err := savePipelineWorkspace(workspace); err != nil {
		return desktopCommandResult("PIPELINE_WORKSPACE_SAVE_FAILED", nil, err.Error(), false, nil, warnings)
	}
	return desktopCommandResult("PIPELINE_WORKSPACE_SAVE_OK", workspace, "工作流绑定配置已清空。", true, nil, warnings)
}

func (a *App) LoadPipelineProfiles() DesktopCommandResult {
	workspace, warnings, err := loadPipelineWorkspaceOrDefault()
	if err != nil {
		return desktopCommandResult("PIPELINE_PROFILE_LOAD_FAILED", nil, err.Error(), false, nil, warnings)
	}
	store := pipelineProfileStoreFromWorkspace(workspace)
	return desktopCommandResult("PIPELINE_PROFILE_LOAD_OK", store, "策略管道已加载。", true, nil, warnings)
}

func (a *App) SavePipelineProfiles(payload map[string]any) DesktopCommandResult {
	rawStore := firstNonNil(payload["pipeline_profiles"], payload["pipelineProfiles"], payload["store"])
	store := appcore.PipelineProfileStoreFromAny(rawStore)
	if len(store.Items) == 0 {
		current, err := loadDesktopConfigSnapshotFromDisk()
		if err != nil {
			return desktopCommandResult("PIPELINE_PROFILE_INVALID", nil, err.Error(), false, nil, nil)
		}
		store = defaultPipelineProfileStoreFromSnapshot(current)
	}
	workspace, warnings, loadErr := loadPipelineWorkspaceOrDefault()
	if loadErr != nil {
		return desktopCommandResult("PIPELINE_PROFILE_LOAD_FAILED", nil, loadErr.Error(), false, nil, warnings)
	}
	workspace = applyLegacyProfileStoreToWorkspace(workspace, normalizePipelineProfileStoreForSave(store))
	if err := savePipelineWorkspace(workspace); err != nil {
		return desktopCommandResult("PIPELINE_PROFILE_SAVE_FAILED", nil, err.Error(), false, nil, nil)
	}
	return desktopCommandResult("PIPELINE_PROFILE_SAVE_OK", pipelineProfileStoreFromWorkspace(workspace), "策略管道已保存。", true, nil, nil)
}

func (a *App) SavePipelineProfile(payload map[string]any) DesktopCommandResult {
	profile := pipelineProfileFromPayload(payload)
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
	result := a.SavePipelineTarget(targetPayload)
	if !result.OK {
		return desktopCommandResult("PIPELINE_PROFILE_SAVE_FAILED", nil, result.Message, false, result.TaskID, result.Warnings)
	}
	return desktopCommandResult("PIPELINE_PROFILE_SAVE_OK", pipelineProfileStoreFromWorkspace(pipelineWorkspaceFromAny(result.Data)), "策略已保存。", true, nil, result.Warnings)
}

func (a *App) DeletePipelineProfile(payload map[string]any) DesktopCommandResult {
	profileID := strings.TrimSpace(stringValue(firstNonNil(payload["profile_id"], payload["profileId"], payload["id"]), ""))
	if profileID == "" {
		return desktopCommandResult("PIPELINE_PROFILE_INVALID", nil, "缺少 profile_id。", false, nil, nil)
	}
	result := a.DeletePipelineTarget(map[string]any{"target_id": profileID})
	if !result.OK {
		return desktopCommandResult("PIPELINE_PROFILE_DELETE_FAILED", nil, result.Message, false, result.TaskID, result.Warnings)
	}
	return desktopCommandResult("PIPELINE_PROFILE_DELETE_OK", pipelineProfileStoreFromWorkspace(pipelineWorkspaceFromAny(result.Data)), "策略已删除。", true, nil, result.Warnings)
}

func (a *App) RunPipeline(payload PipelineRunPayload) DesktopCommandResult {
	normalized := normalizePipelineRunPayload(payload)
	result, err := a.runPipeline(normalized)
	taskID := normalized.PipelineID
	if strings.TrimSpace(result.PipelineID) != "" {
		taskID = result.PipelineID
	}
	if err != nil {
		code := "PIPELINE_FAILED"
		var data any = result
		if strings.Contains(err.Error(), probeAlreadyRunningMessage) {
			code = "PIPELINE_ALREADY_RUNNING"
		}
		if strings.TrimSpace(result.PipelineID) == "" {
			data = nil
		}
		return desktopCommandResult(code, data, err.Error(), false, &taskID, result.Warnings)
	}
	return desktopCommandResult("PIPELINE_COMPLETED", result, "策略管道已完成。", true, &taskID, result.Warnings)
}

func (a *App) runPipeline(payload PipelineRunPayload) (PipelineRunResult, error) {
	payload = normalizePipelineRunPayload(payload)
	if ok, current := a.claimPipeline(payload.PipelineID); !ok {
		return PipelineRunResult{}, fmt.Errorf("%s：%s", probeAlreadyRunningMessage, current)
	}
	defer a.clearPipeline(payload.PipelineID)
	return a.runPipelineClaimed(payload)
}

func (a *App) StartPipeline(payload PipelineRunPayload) DesktopCommandResult {
	payload = normalizePipelineRunPayload(payload)
	if ok, current := a.claimPipeline(payload.PipelineID); !ok {
		if strings.TrimSpace(current) == "" {
			current = payload.PipelineID
		}
		return desktopCommandResult("PIPELINE_ALREADY_RUNNING", nil, probeAlreadyRunningMessage, false, &current, nil)
	}
	go func() {
		defer a.clearPipeline(payload.PipelineID)
		defer func() {
			if recovered := recover(); recovered != nil {
				emitter := pipelineEventEmitter{app: a, pipelineID: payload.PipelineID}
				message := fmt.Sprintf("策略管道异常退出：%v", recovered)
				result := PipelineRunResult{
					CompletedAt: time.Now().Format(time.RFC3339),
					DurationMS:  0,
					PipelineID:  payload.PipelineID,
					StartedAt:   time.Now().Format(time.RFC3339),
					Status:      "failed",
					TaskID:      payload.TaskID,
					TemplateID:  strings.TrimSpace(payload.TemplateID),
					Warnings:    []string{message},
				}
				a.rememberPipelineResult(result)
				emitter.emit("pipeline.failed", map[string]any{
					"message":     message,
					"pipeline_id": payload.PipelineID,
					"task_id":     payload.TaskID,
				})
			}
		}()
		_, _ = a.runPipelineClaimed(payload)
	}()
	return desktopCommandResult("PIPELINE_ACCEPTED", map[string]any{
		"accepted":    true,
		"pipeline_id": payload.PipelineID,
		"task_id":     payload.TaskID,
	}, "策略管道已提交。", true, &payload.PipelineID, nil)
}

func (a *App) CancelPipeline(payload map[string]any) DesktopCommandResult {
	pipelineID := strings.TrimSpace(stringValue(firstNonNil(payload["pipeline_id"], payload["pipelineId"], payload["task_id"], payload["taskId"]), ""))
	a.pipelineMu.Lock()
	if pipelineID == "" {
		pipelineID = a.currentPipelineID
	}
	if pipelineID == "" || pipelineID != a.currentPipelineID {
		a.pipelineMu.Unlock()
		return desktopCommandResult("PIPELINE_CANCEL_UNAVAILABLE", nil, "当前没有可终止的策略管道。", false, &pipelineID, nil)
	}
	a.currentPipelineCancel = true
	a.pipelineMu.Unlock()
	cancelResult := a.CancelProbe(map[string]any{"mode": "cancel"})
	if !cancelResult.OK && cancelResult.Code != "PROBE_CANCEL_UNAVAILABLE" {
		return cancelResult
	}
	return desktopCommandResult("PIPELINE_CANCEL_REQUESTED", nil, "已请求终止策略管道。", true, &pipelineID, nil)
}

func (a *App) GetPipelineSnapshot(payload map[string]any) DesktopCommandResult {
	pipelineID := strings.TrimSpace(stringValue(firstNonNil(payload["pipeline_id"], payload["pipelineId"], payload["task_id"], payload["taskId"]), ""))
	a.pipelineMu.Lock()
	defer a.pipelineMu.Unlock()
	if pipelineID == "" {
		pipelineID = a.currentPipelineID
	}
	if pipelineID != "" {
		if result, ok := a.pipelineResults[pipelineID]; ok {
			return desktopCommandResult("PIPELINE_SNAPSHOT_READY", result, "策略管道快照已读取。", true, &pipelineID, nil)
		}
		return desktopCommandResult("PIPELINE_SNAPSHOT_NOT_FOUND", nil, "未找到策略管道快照。", false, &pipelineID, nil)
	}
	for id, result := range a.pipelineResults {
		pipelineID = id
		return desktopCommandResult("PIPELINE_SNAPSHOT_READY", result, "策略管道快照已读取。", true, &pipelineID, nil)
	}
	return desktopCommandResult("PIPELINE_SNAPSHOT_NOT_FOUND", nil, "未找到策略管道快照。", false, &pipelineID, nil)
}

func (a *App) ListPipelineResults(payload map[string]any) DesktopCommandResult {
	pipelineID := strings.TrimSpace(stringValue(firstNonNil(payload["pipeline_id"], payload["pipelineId"], payload["task_id"], payload["taskId"]), ""))
	a.pipelineMu.Lock()
	defer a.pipelineMu.Unlock()
	if pipelineID != "" {
		if result, ok := a.pipelineResults[pipelineID]; ok {
			return desktopCommandResult("PIPELINE_RESULTS_READY", []PipelineRunResult{result}, "策略管道结果已读取。", true, &pipelineID, nil)
		}
		return desktopCommandResult("PIPELINE_RESULTS_READY", []PipelineRunResult{}, "策略管道结果已读取。", true, &pipelineID, nil)
	}
	for _, result := range a.pipelineResults {
		return desktopCommandResult("PIPELINE_RESULTS_READY", []PipelineRunResult{result}, "策略管道结果已读取。", true, nil, nil)
	}
	return desktopCommandResult("PIPELINE_RESULTS_READY", []PipelineRunResult{}, "策略管道结果已读取。", true, nil, nil)
}

func (a *App) runPipelineClaimed(payload PipelineRunPayload) (PipelineRunResult, error) {
	start := time.Now()
	emitter := pipelineEventEmitter{app: a, pipelineID: payload.PipelineID}
	profiles, warnings, err := a.pipelineProfilesForRun(payload)
	template, templateWarnings, templateErr := pipelineTemplateForRunPayload(payload)
	warnings = append(warnings, templateWarnings...)
	targetIDs := make([]string, 0, len(profiles))
	for _, profile := range profiles {
		targetIDs = append(targetIDs, profile.ID)
	}
	result := PipelineRunResult{
		PipelineID: payload.PipelineID,
		StartedAt:  start.Format(time.RFC3339),
		Status:     "running",
		TaskID:     payload.TaskID,
		TargetIDs:  targetIDs,
		TemplateID: firstNonEmptyString(strings.TrimSpace(template.ID), strings.TrimSpace(payload.TemplateID)),
		Total:      len(profiles),
		Warnings:   warnings,
	}
	if err != nil || templateErr != nil {
		runErr := err
		if runErr == nil {
			runErr = templateErr
		}
		result.Status = "failed"
		result.CompletedAt = time.Now().Format(time.RFC3339)
		result.DurationMS = time.Since(start).Milliseconds()
		result.Warnings = dedupeStrings(append(result.Warnings, runErr.Error()))
		a.rememberPipelineResult(result)
		emitter.emit("pipeline.failed", map[string]any{"message": runErr.Error(), "pipeline_id": payload.PipelineID})
		return result, runErr
	}
	emitter.emit("pipeline.started", map[string]any{
		"pipeline_id": payload.PipelineID,
		"task_id":     payload.TaskID,
		"total":       len(profiles),
	})
	for index, profile := range profiles {
		if a.isPipelineCancelRequested(payload.PipelineID) {
			result.Skipped += len(profiles) - index
			break
		}
		profileResult := a.runPipelineProfile(payload, profile, template, index, &emitter)
		result.Results = append(result.Results, profileResult)
		result.TargetResults = append(result.TargetResults, profileResult)
		switch profileResult.Status {
		case "completed":
			result.Succeeded++
		case "skipped":
			result.Skipped++
		default:
			result.Failed++
		}
	}
	a.runPipelinePostProbePush(&result, profiles, template, payload.SchedulerOverrides)
	result.CompletedAt = time.Now().Format(time.RFC3339)
	result.DurationMS = time.Since(start).Milliseconds()
	switch {
	case a.isPipelineCancelRequested(payload.PipelineID):
		result.Status = "cancelled"
	case result.Failed > 0 && result.Succeeded == 0:
		result.Status = "failed"
	case result.Failed > 0 || result.Skipped > 0:
		result.Status = "partial"
	default:
		result.Status = "completed"
	}
	a.rememberPipelineResult(result)
	emitter.emit("pipeline.completed", map[string]any{
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

func (a *App) runPipelinePostProbePush(result *PipelineRunResult, profiles []PipelineProfile, template pipelineTemplateItem, overrides appcore.PipelineRuntimeOverrides) {
	if result == nil || overrides.DisablePostProbePush {
		return
	}
	hasDNSDeliver := pipelineTemplateHasAction(template, appcore.PipelineNodeActionDeliverDNS)
	hasGitHubDeliver := pipelineTemplateHasAction(template, appcore.PipelineNodeActionDeliverGitHub)
	if hasDNSDeliver && hasGitHubDeliver {
		return
	}
	for index := range result.Results {
		if index >= len(profiles) {
			break
		}
		profileResult := &result.Results[index]
		if profileResult.Status != "completed" || profileResult.ProbeResult == nil || len(profileResult.ProbeResult.Results) == 0 {
			continue
		}
		snapshot := pipelineSnapshotForRun(profiles[index], profileResult.TaskID)
		pushCfg := appcore.PostProbePushConfigFromSnapshot(snapshot)
		if hasDNSDeliver {
			pushCfg.CloudflareEnabled = false
		}
		if hasGitHubDeliver {
			pushCfg.GitHubEnabled = false
		}
		if !pushCfg.CloudflareEnabled && !pushCfg.GitHubEnabled {
			continue
		}
		pushSnapshot := deepCloneMap(snapshot)
		pushSnapshot["post_probe_push"] = map[string]any{
			"cloudflare_enabled": pushCfg.CloudflareEnabled,
			"github_enabled":     pushCfg.GitHubEnabled,
		}
		warnings := a.runPostProbePushForSnapshot(pushSnapshot, *profileResult.ProbeResult, profileResult.TaskID)
		profileResult.Warnings = dedupeStrings(append(profileResult.Warnings, warnings...))
		result.Warnings = dedupeStrings(append(result.Warnings, warnings...))
	}
	result.TargetResults = append([]appcore.PipelineProfileRunResult{}, result.Results...)
}

func pipelineTemplateHasAction(template pipelineTemplateItem, action string) bool {
	action = strings.TrimSpace(action)
	for _, node := range template.Nodes {
		if strings.TrimSpace(node.Action) == action {
			return true
		}
	}
	return false
}

func (a *App) runPipelineProfile(payload PipelineRunPayload, profile PipelineProfile, template pipelineTemplateItem, index int, emitter *pipelineEventEmitter) appcore.PipelineProfileRunResult {
	profileTaskID := pipelineProfileTaskID(payload.PipelineID, profile, index)
	baseResult := appcore.PipelineProfileRunResult{
		Domain:      profile.Domain,
		TargetID:    profile.ID,
		TargetName:  profile.Name,
		ProfileID:   profile.ID,
		ProfileName: profile.Name,
		Region:      profile.Region,
		Status:      "running",
		TaskID:      profileTaskID,
	}
	if !profile.Enabled {
		baseResult.Status = "skipped"
		baseResult.Message = "策略未启用，已跳过。"
		baseResult.NodeResults = append(baseResult.NodeResults, appcore.PipelineNodeRunResult{
			Action:        appcore.PipelineNodeActionEnd,
			CompletedAt:   time.Now().Format(time.RFC3339),
			Message:       baseResult.Message,
			NodeID:        "end-skipped",
			NodeName:      "跳过",
			NodeType:      appcore.PipelineNodeTypeEnd,
			OutputSummary: "skipped",
			StartedAt:     time.Now().Format(time.RFC3339),
			Status:        "skipped",
		})
		emitter.emit("pipeline.profile_skipped", pipelineProfileEventPayload(profile, profileTaskID, map[string]any{"message": baseResult.Message}))
		return baseResult
	}
	emitter.emit("pipeline.profile_started", pipelineProfileEventPayload(profile, profileTaskID, map[string]any{
		"index": index,
	}))
	runtimeCtx := &pipelineRuntimeContext{
		ConfigSnapshot:     pipelineSnapshotForRun(profile, profileTaskID),
		NodeOutputs:        map[string]any{},
		Payload:            payload,
		Profile:            profile,
		SchedulerOverrides: payload.SchedulerOverrides,
		TaskID:             profileTaskID,
		Target:             pipelineTargetFromProfile(profile, template.ID),
		Template:           template,
		Warnings:           []string{},
	}
	result, err := a.executeTemplateDAG(profile, template, runtimeCtx, profileTaskID, emitter)
	if err != nil {
		emitter.emit("pipeline.profile_failed", pipelineProfileEventPayload(profile, profileTaskID, map[string]any{
			"message": err.Error(),
			"status":  result.Status,
		}))
		return result
	}
	emitter.emit("pipeline.profile_completed", pipelineProfileEventPayload(profile, profileTaskID, map[string]any{
		"dns_result":   result.DNSResult,
		"node_results": result.NodeResults,
		"result_count": pipelineResultCount(result.ProbeResult, runtimeCtx.FilteredRows),
		"status":       result.Status,
	}))
	return result
}

func (a *App) executeTemplateDAG(profile PipelineProfile, template pipelineTemplateItem, runtimeCtx *pipelineRuntimeContext, profileTaskID string, emitter *pipelineEventEmitter) (appcore.PipelineProfileRunResult, error) {
	result := appcore.PipelineProfileRunResult{
		Domain:      profile.Domain,
		TargetID:    profile.ID,
		TargetName:  profile.Name,
		ProfileID:   profile.ID,
		ProfileName: profile.Name,
		Region:      profile.Region,
		Status:      "running",
		TaskID:      profileTaskID,
	}
	nodeByID := make(map[string]appcore.PipelineNode, len(template.Nodes))
	outgoing := make(map[string][]appcore.PipelineEdge, len(template.Nodes))
	for _, node := range template.Nodes {
		nodeByID[node.ID] = node
		outgoing[node.ID] = []appcore.PipelineEdge{}
	}
	for _, edge := range template.Edges {
		outgoing[edge.SourceNode] = append(outgoing[edge.SourceNode], edge)
	}
	currentNodeID := strings.TrimSpace(template.EntryNodeID)
	upstreamStatus := ""
	upstreamMessage := ""
	for currentNodeID != "" {
		node, ok := nodeByID[currentNodeID]
		if !ok {
			result.Status = "failed"
			result.Message = fmt.Sprintf("节点 %s 不存在。", currentNodeID)
			result.ProbeResult = runtimeCtx.ProbeResult
			result.DNSResult = runtimeCtx.DNSResult
			result.Warnings = dedupeStrings(append(result.Warnings, runtimeCtx.Warnings...))
			return result, errors.New(result.Message)
		}
		startedAt := time.Now().Format(time.RFC3339)
		emitter.emit("pipeline.node_started", pipelineProfileEventPayload(profile, profileTaskID, map[string]any{
			"action":    node.Action,
			"node_id":   node.ID,
			"node_name": node.Name,
			"node_type": node.NodeType,
		}))
		execResult, execErr := a.executePipelineNode(node, runtimeCtx)
		nodeStatus := strings.TrimSpace(execResult.Status)
		if nodeStatus == "" {
			nodeStatus = "completed"
		}
		if execErr != nil && nodeStatus == "completed" {
			nodeStatus = "failed"
		}
		nodeResult := appcore.PipelineNodeRunResult{
			Action:        node.Action,
			BranchTaken:   strings.TrimSpace(execResult.Outcome),
			CompletedAt:   time.Now().Format(time.RFC3339),
			Message:       firstNonEmptyString(strings.TrimSpace(execResult.Message), pipelineNodeFallbackMessage(node, nodeStatus)),
			Metrics:       execResult.Metrics,
			NodeID:        node.ID,
			NodeName:      node.Name,
			NodeType:      node.NodeType,
			Outcome:       strings.TrimSpace(execResult.Outcome),
			OutputSummary: strings.TrimSpace(execResult.OutputSummary),
			StartedAt:     startedAt,
			Status:        nodeStatus,
		}
		result.NodeResults = append(result.NodeResults, nodeResult)
		emitter.emit("pipeline.node_completed", pipelineProfileEventPayload(profile, profileTaskID, map[string]any{
			"action":         node.Action,
			"message":        nodeResult.Message,
			"node_id":        nodeResult.NodeID,
			"node_name":      nodeResult.NodeName,
			"node_type":      nodeResult.NodeType,
			"outcome":        nodeResult.Outcome,
			"output_summary": nodeResult.OutputSummary,
			"status":         nodeResult.Status,
		}))
		if normalizePipelineNodeType(node.NodeType) == appcore.PipelineNodeTypeBranch {
			emitter.emit("pipeline.branch_taken", pipelineProfileEventPayload(profile, profileTaskID, map[string]any{
				"action":       node.Action,
				"branch_taken": nodeResult.Outcome,
				"node_id":      nodeResult.NodeID,
				"node_name":    nodeResult.NodeName,
				"node_type":    nodeResult.NodeType,
				"result_count": pipelineRuntimeResultCount(runtimeCtx, node),
			}))
		}
		if execErr != nil {
			result.Status = pipelineProfileFailureStatus(node.Action, nodeStatus)
			result.Message = firstNonEmptyString(strings.TrimSpace(execResult.Message), execErr.Error())
			result.ProbeResult = runtimeCtx.ProbeResult
			result.DNSResult = runtimeCtx.DNSResult
			result.Warnings = dedupeStrings(append(result.Warnings, runtimeCtx.Warnings...))
			return result, execErr
		}
		if normalizePipelineNodeType(node.NodeType) == appcore.PipelineNodeTypeEnd {
			endStatus := normalizePipelineProfileStatus(nodeResult.Status)
			if endStatus == "completed" && upstreamStatus != "" {
				result.Status = upstreamStatus
				result.Message = firstNonEmptyString(upstreamMessage, nodeResult.Message)
			} else {
				result.Status = endStatus
				result.Message = nodeResult.Message
			}
			break
		}
		normalizedNodeStatus := normalizePipelineProfileStatus(nodeResult.Status)
		if normalizedNodeStatus != "completed" && upstreamStatus == "" {
			upstreamStatus = normalizedNodeStatus
			upstreamMessage = nodeResult.Message
		}
		nextNodeID, nextErr := pipelineNextNodeID(node, outgoing[node.ID], nodeResult.Outcome)
		if nextErr != nil {
			result.Status = "failed"
			result.Message = nextErr.Error()
			result.ProbeResult = runtimeCtx.ProbeResult
			result.DNSResult = runtimeCtx.DNSResult
			result.Warnings = dedupeStrings(append(result.Warnings, runtimeCtx.Warnings...))
			return result, nextErr
		}
		currentNodeID = nextNodeID
	}
	result.ProbeResult = runtimeCtx.ProbeResult
	result.DNSResult = runtimeCtx.DNSResult
	result.Warnings = dedupeStrings(append(result.Warnings, runtimeCtx.Warnings...))
	if result.Status == "running" {
		result.Status = "completed"
	}
	if strings.TrimSpace(result.Message) == "" {
		result.Message = pipelineDefaultProfileMessage(result.Status, pipelineResultCount(result.ProbeResult, runtimeCtx.FilteredRows))
	}
	return result, nil
}

func (a *App) executePipelineNode(node appcore.PipelineNode, runtimeCtx *pipelineRuntimeContext) (pipelineNodeExecutionResult, error) {
	executors := a.pipelineNodeExecutors()
	action := normalizePipelineNodeAction(node.Action)
	executor, ok := executors[action]
	if !ok {
		return pipelineNodeExecutionResult{}, fmt.Errorf("不支持的节点动作 %s", node.Action)
	}
	result, err := executor(node, runtimeCtx)
	if result.Output != nil {
		if runtimeCtx.NodeOutputs == nil {
			runtimeCtx.NodeOutputs = map[string]any{}
		}
		runtimeCtx.NodeOutputs[node.ID] = result.Output
	}
	return result, err
}

func (a *App) pipelineNodeExecutors() map[string]pipelineNodeExecutor {
	return map[string]pipelineNodeExecutor{
		appcore.PipelineNodeActionSelectSources:    a.executeSelectSourcesNode,
		appcore.PipelineNodeActionFilterSources:    a.executeFilterSourcesNode,
		appcore.PipelineNodeActionProbeTCP:         a.executeProbeTCPNode,
		appcore.PipelineNodeActionProbeTrace:       a.executeProbeTraceNode,
		appcore.PipelineNodeActionProbeDownload:    a.executeProbeDownloadNode,
		appcore.PipelineNodeActionFilterResults:    a.executeFilterResultsNode,
		appcore.PipelineNodeActionBranchHasResults: a.executeBranchHasResultsNode,
		appcore.PipelineNodeActionDeliverDNS:       a.executeDeliverDNSNode,
		appcore.PipelineNodeActionDeliverGitHub:    a.executeDeliverGitHubNode,
		appcore.PipelineNodeActionRecoveryMark:     a.executeRecoveryMarkNode,
		appcore.PipelineNodeActionCheckOutput:      a.executeCheckOutputNode,
		appcore.PipelineNodeActionEnd:              a.executeEndNode,
	}
}

func (a *App) executeSelectSourcesNode(node appcore.PipelineNode, runtimeCtx *pipelineRuntimeContext) (pipelineNodeExecutionResult, error) {
	sources, err := pipelineSourceGroupSourcesForNode(runtimeCtx, node)
	if err != nil {
		return pipelineNodeExecutionResult{Message: err.Error(), Status: "failed"}, err
	}
	runtimeCtx.SelectedSources = cloneDesktopSources(sources)
	enabledCount := 0
	for _, source := range sources {
		if source.Enabled {
			enabledCount++
		}
	}
	message := fmt.Sprintf("输入源组已选择 %d 个输入源。", len(sources))
	if len(sources) == 0 {
		message = "输入源组没有选中可用输入源。"
	}
	return pipelineNodeExecutionResult{
		Message: message,
		Metrics: map[string]any{
			"enabled_sources":  enabledCount,
			"selected_sources": len(sources),
		},
		Output:        cloneDesktopSources(sources),
		OutputSummary: fmt.Sprintf("%d 个输入源", len(sources)),
		Status:        "completed",
	}, nil
}

func (a *App) executeFilterSourcesNode(node appcore.PipelineNode, runtimeCtx *pipelineRuntimeContext) (pipelineNodeExecutionResult, error) {
	sources := pipelineProbeSourcesForNode(runtimeCtx, node)
	runtimeCtx.SelectedSources = cloneDesktopSources(sources)
	enabledCount := 0
	for _, source := range sources {
		if source.Enabled {
			enabledCount++
		}
	}
	message := fmt.Sprintf("输入源筛选已输出 %d 个输入源。", len(sources))
	if len(sources) == 0 {
		message = "输入源筛选后没有可用输入源。"
	}
	return pipelineNodeExecutionResult{
		Message: message,
		Metrics: map[string]any{
			"enabled_sources":  enabledCount,
			"selected_sources": len(sources),
		},
		Output:        cloneDesktopSources(sources),
		OutputSummary: fmt.Sprintf("%d 个输入源", len(sources)),
		Status:        "completed",
	}, nil
}

func (a *App) executeProbeTCPNode(node appcore.PipelineNode, runtimeCtx *pipelineRuntimeContext) (pipelineNodeExecutionResult, error) {
	stage, snapshot, err := a.preparePipelineProbeStage(node, runtimeCtx)
	if err != nil {
		return pipelineNodeExecutionResult{Message: err.Error(), Status: "failed"}, err
	}
	cfg := stage.Config
	applyProbeConfig(cfg)
	task.SourceColoFilters = task.CloneSourceColoFilterMap(stage.Prepared.SourceColoFilters)
	task.InitRandSeed()
	task.Httping = false
	info := probecore.StageInfo{Stage: probecore.StageTCP, Total: stage.Source.ValidCount}
	tcpData, err := probecore.RunTCPStage(info, probecore.StageWorkflowAdapter{
		RunTCP: func() utils.PingDelaySet {
			return desktopTCPProbeRunner()
		},
	})
	if err != nil {
		return pipelineNodeExecutionResult{Message: err.Error(), Status: "failed"}, err
	}
	stage.TCPData = append(utils.PingDelaySet(nil), tcpData...)
	stage.CompletedStages = append(stage.CompletedStages, probecore.StageTCP)
	runtimeCtx.ProbeStage = stage
	runtimeCtx.ProbeStageSnapshot = snapshot
	return pipelineNodeExecutionResult{
		Message: "TCP 延迟测速已完成。",
		Metrics: map[string]any{
			"input_count":  stage.Source.ValidCount,
			"passed_count": len(tcpData),
		},
		Output:        append(utils.PingDelaySet(nil), tcpData...),
		OutputSummary: fmt.Sprintf("%d / %d 个候选", len(tcpData), stage.Source.ValidCount),
		Status:        "completed",
	}, nil
}

func (a *App) executeProbeTraceNode(node appcore.PipelineNode, runtimeCtx *pipelineRuntimeContext) (pipelineNodeExecutionResult, error) {
	stage, err := pipelineExistingProbeStage(runtimeCtx, probecore.StageTCP)
	if err != nil {
		return pipelineNodeExecutionResult{Message: err.Error(), Status: "failed"}, err
	}
	snapshot := pipelineProbeSnapshotForNode(&pipelineRuntimeContext{ConfigSnapshot: runtimeCtx.ProbeStageSnapshot}, node)
	cfg, configWarnings := desktopConfigToProbeConfig(snapshot)
	cfg = applyDesktopExportConfig(cfg, snapshot, runtimeCtx.TaskID, runtimeCtx.Profile.Name)
	stage.Config = cfg
	stage.ConfigWarnings = dedupeStrings(append(stage.ConfigWarnings, configWarnings...))
	stage.TaskContext.ConfigSource = firstNonEmptyString(runtimeCtx.Payload.ConfigSource, "pipeline")
	applyProbeConfig(cfg)
	task.SourceColoFilters = task.CloneSourceColoFilterMap(stage.Prepared.SourceColoFilters)
	traceTotal := task.EstimateTraceProbeCount(len(stage.TCPData))
	info := probecore.StageInfo{Stage: probecore.StageTrace, Input: len(stage.TCPData), Total: traceTotal}
	traceData, err := probecore.RunTraceStage(info, stage.TCPData, probecore.StageWorkflowAdapter{RunTrace: desktopTraceProbeRunner})
	if err != nil {
		return pipelineNodeExecutionResult{Message: err.Error(), Status: "failed"}, err
	}
	stage.TraceData = append(utils.PingDelaySet(nil), traceData...)
	stage.CompletedStages = append(stage.CompletedStages, probecore.StageTrace)
	runtimeCtx.ProbeStage = stage
	runtimeCtx.ProbeStageSnapshot = snapshot
	message := "追踪测试已完成。"
	if len(traceData) == 0 && len(stage.TCPData) > 0 {
		message = "追踪测试未命中可用候选。"
	}
	return pipelineNodeExecutionResult{
		Message: message,
		Metrics: map[string]any{
			"input_count":  len(stage.TCPData),
			"passed_count": len(traceData),
		},
		Output:        append(utils.PingDelaySet(nil), traceData...),
		OutputSummary: fmt.Sprintf("%d / %d 个候选", len(traceData), len(stage.TCPData)),
		Status:        "completed",
	}, nil
}

func (a *App) executeProbeDownloadNode(node appcore.PipelineNode, runtimeCtx *pipelineRuntimeContext) (pipelineNodeExecutionResult, error) {
	stage, err := pipelineExistingProbeStage(runtimeCtx, probecore.StageTrace)
	if err != nil {
		return pipelineNodeExecutionResult{Message: err.Error(), Status: "failed"}, err
	}
	snapshot := pipelineProbeSnapshotForNode(&pipelineRuntimeContext{ConfigSnapshot: runtimeCtx.ProbeStageSnapshot}, node)
	cfg, configWarnings := desktopConfigToProbeConfig(snapshot)
	cfg = applyDesktopExportConfig(cfg, snapshot, runtimeCtx.TaskID, runtimeCtx.Profile.Name)
	stage.Config = cfg
	stage.ConfigWarnings = dedupeStrings(append(stage.ConfigWarnings, configWarnings...))
	applyProbeConfig(cfg)
	task.SourceColoFilters = task.CloneSourceColoFilterMap(stage.Prepared.SourceColoFilters)
	downloadInput := probecore.LimitPingDelaySet(stage.TraceData, cfg.Stage3Limit)
	downloadTotal := probecore.EstimateDownloadProbeCount(len(downloadInput))
	info := probecore.StageInfo{Stage: probecore.StageDownload, Input: len(downloadInput), Total: downloadTotal}
	speedData, err := probecore.RunDownloadStage(info, downloadInput, probecore.StageWorkflowAdapter{RunDownload: desktopDownloadProbeRunner})
	if err != nil {
		return pipelineNodeExecutionResult{Message: err.Error(), Status: "failed"}, err
	}
	stage.CompletedStages = append(stage.CompletedStages, probecore.StageDownload)
	resultData := []utils.CloudflareIPData(speedData)
	resultData = probecore.LimitFinalResults(resultData, cfg.PrintNum, cfg.DownloadSpeedMetric)
	rows := pipelineRowsFromRawResults(resultData, stage.SourcePorts, stage.TestPorts, cfg.TCPPort)
	warnings := probecore.BuildProbeWarnings(stage.Source)
	warnings = append(warnings, stage.ConfigWarnings...)
	warnings = append(warnings, stage.Warnings...)
	if len(stage.TraceData) == 0 && len(stage.TCPData) > 0 {
		warnings = append(warnings, "追踪探测未命中可用候选，已无可导出的结果。")
	}
	outputFile := ""
	if len(resultData) > 0 {
		outputFile = currentOutputFile(cfg)
		if outputFile != "" {
			applyProbeConfig(cfg)
			if exportErr := utils.ExportCsv(resultData); exportErr != nil {
				warnings = append(warnings, fmt.Sprintf("结果导出失败：%v", exportErr))
				outputFile = ""
			}
		}
	}
	probeResult := ProbeRunResult{
		Config:         cfg,
		DurationMS:     time.Since(stage.StartedAt).Milliseconds(),
		OutputFile:     outputFile,
		Results:        rows,
		Source:         stage.Source,
		SourceStatuses: stage.Prepared.SourceStatuses,
		StartedAt:      stage.StartedAt.Format(time.RFC3339),
		Summary:        probecore.SummarizeProbeRows(rows, downloadTotal),
		TaskContext:    stage.TaskContext,
		Warnings:       dedupeStrings(warnings),
		SchemaVersion:  guiSchemaVersion,
		RawResults:     append([]utils.CloudflareIPData(nil), resultData...),
	}
	runtimeCtx.ProbeResult = &probeResult
	runtimeCtx.FilteredRows = nil
	runtimeCtx.Warnings = dedupeStrings(append(runtimeCtx.Warnings, probeResult.Warnings...))
	runtimeCtx.ProbeStage = stage
	runtimeCtx.ProbeStageSnapshot = snapshot
	return pipelineNodeExecutionResult{
		Message: "下载测速已完成。",
		Metrics: map[string]any{
			"input_count":  len(downloadInput),
			"result_count": len(probeResult.Results),
		},
		Output:        probeResult,
		OutputSummary: fmt.Sprintf("%d 条测速结果", len(probeResult.Results)),
		Status:        "completed",
	}, nil
}

func (a *App) executeFilterResultsNode(node appcore.PipelineNode, runtimeCtx *pipelineRuntimeContext) (pipelineNodeExecutionResult, error) {
	selection, err := pipelineEnsureUploadSelection(runtimeCtx, node)
	if err != nil {
		return pipelineNodeExecutionResult{
			Message: err.Error(),
			Status:  "failed",
		}, err
	}
	runtimeCtx.FilteredRows = append([]ProbeRow{}, selection.FilteredRows...)
	runtimeCtx.Warnings = dedupeStrings(append(runtimeCtx.Warnings, selection.Warnings...))
	message := fmt.Sprintf("结果筛选保留 %d / %d 条结果。", len(selection.FilteredRows), len(selection.InputRows))
	if len(selection.FilteredRows) == 0 {
		message = "结果筛选后没有剩余结果。"
	}
	return pipelineNodeExecutionResult{
		Message: message,
		Metrics: map[string]any{
			"cloudflare_count": len(selection.CloudflareRows),
			"filtered_count":   len(selection.FilteredRows),
			"github_count":     len(selection.GitHubRows),
			"input_count":      len(selection.InputRows),
		},
		Output:        selection,
		OutputSummary: fmt.Sprintf("%d / %d 条", len(selection.FilteredRows), len(selection.InputRows)),
		Status:        "completed",
	}, nil
}

func (a *App) executeBranchHasResultsNode(node appcore.PipelineNode, runtimeCtx *pipelineRuntimeContext) (pipelineNodeExecutionResult, error) {
	rows := pipelineRowsForNodeSource(runtimeCtx, stringValue(mapValue(node.Config)["source"], ""))
	outcome := "false"
	message := "没有可用结果，进入回退路径。"
	if len(rows) > 0 {
		outcome = "true"
		message = "命中可用结果，继续后续投递。"
	}
	return pipelineNodeExecutionResult{
		Message: message,
		Metrics: map[string]any{
			"result_count": len(rows),
		},
		Outcome:       outcome,
		OutputSummary: fmt.Sprintf("result_count=%d", len(rows)),
		Status:        "completed",
	}, nil
}

func (a *App) executeDeliverDNSNode(node appcore.PipelineNode, runtimeCtx *pipelineRuntimeContext) (pipelineNodeExecutionResult, error) {
	if runtimeCtx.SchedulerOverrides.AllowDNSPush != nil && !*runtimeCtx.SchedulerOverrides.AllowDNSPush {
		return pipelineNodeExecutionResult{
			Message:       "定时调度已关闭自动 DNS 推送，本节点跳过。",
			OutputSummary: "scheduler skipped",
			Status:        "completed",
		}, nil
	}
	if !appcore.PipelineDNSPushEnabled(runtimeCtx.Target.DNSPushPolicy) {
		return pipelineNodeExecutionResult{
			Message:       "目标已配置为跳过 DNS 推送。",
			OutputSummary: "target skipped",
			Status:        "skipped",
		}, nil
	}
	selection, err := pipelineEnsureUploadSelection(runtimeCtx, node)
	if err != nil {
		return pipelineNodeExecutionResult{
			Message: err.Error(),
			Status:  "failed",
		}, err
	}
	dnsSnapshot := pipelineDNSSnapshotForNode(runtimeCtx, node)
	recordType := stringValue(mapValue(dnsSnapshot["cloudflare"])["record_type"], cloudflareRecordTypeA)
	rows := filterRowsForCloudflareRecordType(selection.CloudflareRows, recordType)
	if len(rows) == 0 {
		return pipelineNodeExecutionResult{
			Message:       "筛选后没有可推送到 Cloudflare 的 IP。",
			Metrics:       map[string]any{"cloudflare_count": 0},
			OutputSummary: "0 条",
			Status:        "skipped",
		}, nil
	}
	dnsResult := a.PushCloudflareDNSRecords(map[string]any{
		"config": dnsSnapshot,
		"ipsRaw": probeRowsIPList(rows),
	})
	runtimeCtx.DNSResult = dnsResult
	runtimeCtx.Warnings = dedupeStrings(append(runtimeCtx.Warnings, dnsResult.Warnings...))
	status := "completed"
	if !dnsResult.OK {
		status = "failed"
	}
	result := pipelineNodeExecutionResult{
		Message: dnsResult.Message,
		Metrics: map[string]any{
			"cloudflare_count": len(rows),
		},
		Output:        dnsResult,
		OutputSummary: fmt.Sprintf("%d 条", len(rows)),
		Status:        status,
	}
	if !dnsResult.OK {
		return result, errors.New(dnsResult.Message)
	}
	return result, nil
}

func (a *App) executeDeliverGitHubNode(node appcore.PipelineNode, runtimeCtx *pipelineRuntimeContext) (pipelineNodeExecutionResult, error) {
	if runtimeCtx.SchedulerOverrides.AllowGitHubExport != nil && !*runtimeCtx.SchedulerOverrides.AllowGitHubExport {
		return pipelineNodeExecutionResult{
			Message:       "定时调度已关闭自动 GitHub 导出，本节点跳过。",
			OutputSummary: "scheduler skipped",
			Status:        "completed",
		}, nil
	}
	selection, err := pipelineEnsureUploadSelection(runtimeCtx, node)
	if err != nil {
		return pipelineNodeExecutionResult{
			Message: err.Error(),
			Status:  "failed",
		}, err
	}
	if len(selection.GitHubRows) == 0 {
		return pipelineNodeExecutionResult{
			Message:       "筛选后没有可导出的 GitHub 结果。",
			Metrics:       map[string]any{"github_count": 0},
			OutputSummary: "0 条",
			Status:        "skipped",
		}, nil
	}
	exportResult := a.ExportResultsToGitHub(map[string]any{
		"config":  runtimeCtx.ConfigSnapshot,
		"results": selection.GitHubRows,
		"task_id": runtimeCtx.TaskID,
	})
	runtimeCtx.Warnings = dedupeStrings(append(runtimeCtx.Warnings, exportResult.Warnings...))
	status := "completed"
	if !exportResult.OK {
		status = "failed"
	}
	result := pipelineNodeExecutionResult{
		Message: exportResult.Message,
		Metrics: map[string]any{
			"github_count": len(selection.GitHubRows),
		},
		Output:        exportResult,
		OutputSummary: fmt.Sprintf("%d 条", len(selection.GitHubRows)),
		Status:        status,
	}
	if !exportResult.OK {
		return result, errors.New(exportResult.Message)
	}
	return result, nil
}

func (a *App) executeCheckOutputNode(node appcore.PipelineNode, runtimeCtx *pipelineRuntimeContext) (pipelineNodeExecutionResult, error) {
	sourceRows := pipelineRowsForNodeSource(runtimeCtx, stringValue(mapValue(node.Config)["source"], "probe_results"))
	if len(sourceRows) == 0 {
		return pipelineNodeExecutionResult{
			Message:       "没有可输出的测速结果，需要人工复核。",
			Metrics:       map[string]any{"result_count": 0},
			OutputSummary: "0 条结果",
			Status:        "manual_review",
		}, nil
	}
	selection, err := pipelineEnsureUploadSelection(runtimeCtx, node)
	if err != nil {
		return pipelineNodeExecutionResult{
			Message: err.Error(),
			Status:  "failed",
		}, err
	}
	rows := append([]ProbeRow{}, selection.FilteredRows...)
	runtimeCtx.Warnings = dedupeStrings(append(runtimeCtx.Warnings, selection.Warnings...))
	if len(rows) == 0 {
		return pipelineNodeExecutionResult{
			Message:       "结果筛选后没有可输出的测速结果，需要人工复核。",
			Metrics:       map[string]any{"input_count": len(sourceRows), "result_count": 0},
			OutputSummary: "0 条结果",
			Status:        "manual_review",
		}, nil
	}

	requireCSV := boolValue(mapValue(node.Config)["require_csv"], true)
	exportIfMissing := boolValue(mapValue(node.Config)["export_if_missing"], true)
	outputFile := ""
	if runtimeCtx.ProbeResult != nil {
		outputFile = strings.TrimSpace(runtimeCtx.ProbeResult.OutputFile)
	}
	csvWritten := false
	if outputFile != "" {
		if info, err := os.Stat(outputFile); err == nil && !info.IsDir() && info.Size() > 0 {
			csvWritten = true
		}
	}
	exportMessage := ""
	if requireCSV && !csvWritten && exportIfMissing {
		exportResult := a.ExportResultsCSV(map[string]any{
			"config":  runtimeCtx.ConfigSnapshot,
			"results": rows,
			"task_id": runtimeCtx.TaskID,
		})
		runtimeCtx.Warnings = dedupeStrings(append(runtimeCtx.Warnings, exportResult.Warnings...))
		exportMessage = exportResult.Message
		if !exportResult.OK {
			return pipelineNodeExecutionResult{
				Message:       firstNonEmptyString(exportResult.Message, "CSV 导出失败。"),
				Metrics:       map[string]any{"csv_written": false, "result_count": len(rows)},
				Output:        exportResult,
				OutputSummary: fmt.Sprintf("%d 条结果", len(rows)),
				Status:        "failed",
			}, errors.New(firstNonEmptyString(exportResult.Message, "CSV 导出失败"))
		}
		if data := mapValue(exportResult.Data); len(data) > 0 {
			outputFile = strings.TrimSpace(stringValue(firstNonNil(data["path"], data["target_path"], data["targetPath"]), outputFile))
		}
		csvWritten = true
	}

	if requireCSV && !csvWritten {
		return pipelineNodeExecutionResult{
			Message:       "测速结果存在，但 CSV 尚未写入。",
			Metrics:       map[string]any{"csv_written": false, "result_count": len(rows)},
			OutputSummary: fmt.Sprintf("%d 条结果", len(rows)),
			Status:        "manual_review",
		}, nil
	}

	message := fmt.Sprintf("结果检查完成：%d 条结果，CSV 已写入。", len(rows))
	if !requireCSV {
		message = fmt.Sprintf("结果检查完成：%d 条结果。", len(rows))
	} else if exportMessage != "" {
		message = exportMessage
	}
	return pipelineNodeExecutionResult{
		Message: message,
		Metrics: map[string]any{
			"csv_written":  csvWritten,
			"result_count": len(rows),
		},
		Output: map[string]any{
			"output_file":  outputFile,
			"result_count": len(rows),
		},
		OutputSummary: fmt.Sprintf("%d 条结果", len(rows)),
		Status:        "completed",
	}, nil
}

func (a *App) executeRecoveryMarkNode(node appcore.PipelineNode, runtimeCtx *pipelineRuntimeContext) (pipelineNodeExecutionResult, error) {
	status := firstNonEmptyString(strings.TrimSpace(stringValue(mapValue(node.Config)["status"], "")), "manual_review")
	message := firstNonEmptyString(strings.TrimSpace(stringValue(mapValue(node.Config)["message"], "")), "需要人工复核。")
	runtimeCtx.Warnings = dedupeStrings(append(runtimeCtx.Warnings, message))
	return pipelineNodeExecutionResult{
		Message:       message,
		Output:        map[string]any{"status": status},
		OutputSummary: status,
		Status:        "completed",
	}, nil
}

func (a *App) executeEndNode(node appcore.PipelineNode, _ *pipelineRuntimeContext) (pipelineNodeExecutionResult, error) {
	status := normalizePipelineProfileStatus(stringValue(mapValue(node.Config)["status"], "completed"))
	message := strings.TrimSpace(stringValue(mapValue(node.Config)["message"], ""))
	if message == "" {
		message = pipelineDefaultProfileMessage(status, 0)
	}
	return pipelineNodeExecutionResult{
		Message:       message,
		Output:        map[string]any{"status": status},
		OutputSummary: status,
		Status:        status,
	}, nil
}

func pipelineNextNodeID(node appcore.PipelineNode, edges []appcore.PipelineEdge, outcome string) (string, error) {
	if normalizePipelineNodeType(node.NodeType) == appcore.PipelineNodeTypeEnd {
		return "", nil
	}
	if normalizePipelineNodeType(node.NodeType) == appcore.PipelineNodeTypeBranch {
		for _, edge := range edges {
			if strings.TrimSpace(edge.Outcome) == strings.TrimSpace(outcome) {
				return strings.TrimSpace(edge.TargetNode), nil
			}
		}
		return "", fmt.Errorf("分支节点 %s 缺少 outcome=%s 的出边", node.ID, outcome)
	}
	if len(edges) == 0 {
		return "", nil
	}
	return strings.TrimSpace(edges[0].TargetNode), nil
}

func (a *App) preparePipelineProbeStage(node appcore.PipelineNode, runtimeCtx *pipelineRuntimeContext) (*pipelineProbeStageState, map[string]any, error) {
	snapshot := pipelineProbeSnapshotForNode(runtimeCtx, node)
	cfg, configWarnings := desktopConfigToProbeConfig(snapshot)
	cfg = applyDesktopExportConfig(cfg, snapshot, runtimeCtx.TaskID, runtimeCtx.Profile.Name)
	sources := pipelineProbeSourcesForNode(runtimeCtx, node)
	prepared := prepareDesktopSources(cfg, sources)
	if len(prepared.FatalErrors) > 0 {
		return nil, snapshot, errors.New(strings.Join(prepared.FatalErrors, "；"))
	}
	preparedSummary := summarizeSource(prepared.Text)
	prepared.Text = strings.Join(preparedSummary.Valid, "\n")
	if strings.TrimSpace(prepared.Text) == "" {
		message := "没有可用的 IP/CIDR/域名输入"
		if len(prepared.Warnings) > 0 {
			message = strings.Join(prepared.Warnings, "；")
		}
		return nil, snapshot, errors.New(message)
	}
	taskContext, portWarnings := probeTaskContextForPorts(cfg, prepared.SourcePorts)
	taskContext.ConfigSource = firstNonEmptyString(runtimeCtx.Payload.ConfigSource, "pipeline")
	prepared.Warnings = append(prepared.Warnings, portWarnings...)
	cfg.IPText = strings.Join(preparedSummary.Valid, ",")
	return &pipelineProbeStageState{
		Config:         cfg,
		ConfigWarnings: configWarnings,
		Prepared:       prepared,
		Source: SourceSummary{
			CandidateCount: preparedSummary.CandidateCount,
			DuplicateCount: preparedSummary.DuplicateCount,
			Duplicates:     preparedSummary.Duplicates,
			Invalid:        preparedSummary.Invalid,
			InvalidCount:   preparedSummary.InvalidCount + prepared.InvalidCount,
			RawLineCount:   preparedSummary.RawLineCount,
			UniqueCount:    preparedSummary.UniqueCount,
			Valid:          preparedSummary.Valid,
			ValidCount:     preparedSummary.ValidCount,
		},
		SourcePorts: prepared.SourcePorts,
		StartedAt:   time.Now(),
		TaskContext: taskContext,
		TestPorts:   pipelineTestPortsForIPs(preparedSummary.Valid, prepared.SourcePorts, cfg.TCPPort, cfg.PortPolicy),
		Warnings:    prepared.Warnings,
	}, snapshot, nil
}

func pipelineExistingProbeStage(runtimeCtx *pipelineRuntimeContext, requiredStage string) (*pipelineProbeStageState, error) {
	if runtimeCtx == nil || runtimeCtx.ProbeStage == nil {
		return nil, errors.New("缺少上游测速阶段输出")
	}
	for _, stage := range runtimeCtx.ProbeStage.CompletedStages {
		if stage == requiredStage {
			return runtimeCtx.ProbeStage, nil
		}
	}
	return nil, fmt.Errorf("缺少上游 %s 阶段输出", requiredStage)
}

func pipelineTestPortsForIPs(ips []string, sourcePorts map[string]int, globalPort int, portPolicy string) map[string]int {
	groups := probecore.PortGroups(ips, sourcePorts, globalPort, portPolicy)
	result := make(map[string]int, len(ips))
	for _, group := range groups {
		port := group.Port
		if port <= 0 {
			port = globalPort
		}
		for _, ip := range group.IPs {
			result[strings.TrimSpace(ip)] = port
		}
	}
	return result
}

func pipelineRowsFromRawResults(raw []utils.CloudflareIPData, sourcePorts map[string]int, testPorts map[string]int, fallbackPort int) []ProbeRow {
	rows := make([]ProbeRow, 0, len(raw))
	for _, item := range raw {
		ip := item.IP.String()
		testPort := testPorts[strings.TrimSpace(ip)]
		if testPort <= 0 {
			testPort = fallbackPort
		}
		rows = append(rows, probecore.ConvertProbeRow(item, sourcePorts[strings.TrimSpace(ip)], testPort))
	}
	return rows
}

func pipelineEnsureUploadSelection(runtimeCtx *pipelineRuntimeContext, node appcore.PipelineNode) (UploadSelectionResult, error) {
	if existing, ok := runtimeCtx.NodeOutputs[node.ID].(UploadSelectionResult); ok {
		runtimeCtx.LastUploadSelection = &existing
		return existing, nil
	}
	sourceRows := pipelineRowsForNodeSource(runtimeCtx, stringValue(mapValue(node.Config)["source"], ""))
	if len(sourceRows) == 0 {
		return UploadSelectionResult{}, errors.New("缺少可筛选的测速结果")
	}
	metric := "average"
	if runtimeCtx.ProbeResult != nil {
		metric = runtimeCtx.ProbeResult.Config.DownloadSpeedMetric
	}
	selectionSnapshot := pipelineSelectionSnapshotForNode(runtimeCtx, node)
	selection, err := BuildUploadSelection(selectionSnapshot, sourceRows, metric)
	if err != nil {
		return UploadSelectionResult{}, err
	}
	if topN, ok := pipelineTopNOverride(node); ok && topN > 0 {
		selection.FilteredRows = pipelineLimitProbeRows(selection.FilteredRows, topN, metric)
		selection.CloudflareRows = pipelineLimitProbeRows(selection.FilteredRows, topN, metric)
		selection.GitHubRows = pipelineLimitProbeRows(selection.FilteredRows, topN, metric)
	}
	if runtimeCtx.NodeOutputs == nil {
		runtimeCtx.NodeOutputs = map[string]any{}
	}
	runtimeCtx.NodeOutputs[node.ID] = selection
	runtimeCtx.LastUploadSelection = &selection
	runtimeCtx.FilteredRows = append([]ProbeRow{}, selection.FilteredRows...)
	return selection, nil
}

func pipelineRowsForNodeSource(runtimeCtx *pipelineRuntimeContext, source string) []ProbeRow {
	switch strings.ToLower(strings.TrimSpace(source)) {
	case "filtered_rows":
		if len(runtimeCtx.FilteredRows) > 0 {
			return append([]ProbeRow{}, runtimeCtx.FilteredRows...)
		}
		if selection, ok := lastUploadSelection(runtimeCtx); ok && len(selection.FilteredRows) > 0 {
			return append([]ProbeRow{}, selection.FilteredRows...)
		}
	case "probe_results":
		if runtimeCtx.ProbeResult != nil && len(runtimeCtx.ProbeResult.Results) > 0 {
			return append([]ProbeRow{}, runtimeCtx.ProbeResult.Results...)
		}
	default:
		if len(runtimeCtx.FilteredRows) > 0 {
			return append([]ProbeRow{}, runtimeCtx.FilteredRows...)
		}
		if selection, ok := lastUploadSelection(runtimeCtx); ok && len(selection.FilteredRows) > 0 {
			return append([]ProbeRow{}, selection.FilteredRows...)
		}
		if runtimeCtx.ProbeResult != nil && len(runtimeCtx.ProbeResult.Results) > 0 {
			return append([]ProbeRow{}, runtimeCtx.ProbeResult.Results...)
		}
	}
	return nil
}

func lastUploadSelection(runtimeCtx *pipelineRuntimeContext) (UploadSelectionResult, bool) {
	if runtimeCtx != nil && runtimeCtx.LastUploadSelection != nil {
		return *runtimeCtx.LastUploadSelection, true
	}
	return UploadSelectionResult{}, false
}

func pipelineProbeSnapshotForNode(runtimeCtx *pipelineRuntimeContext, node appcore.PipelineNode) map[string]any {
	snapshot := sanitizeDesktopConfigSnapshot(deepCloneMap(runtimeCtx.ConfigSnapshot))
	nodeConfig := mapValue(node.Config)
	probe := mapValue(snapshot["probe"])
	concurrency := mapValue(probe["concurrency"])
	thresholds := mapValue(probe["thresholds"])
	stageLimits := mapValue(firstNonNil(probe["stage_limits"], probe["stageLimits"]))
	timeouts := mapValue(probe["timeouts"])

	switch normalizePipelineNodeAction(node.Action) {
	case appcore.PipelineNodeActionProbeTCP, appcore.PipelineNodeActionProbeTrace, appcore.PipelineNodeActionProbeDownload:
		probe["strategy"] = "full"
		probe["disable_download"] = false
	}
	if value, ok := nodeConfig["concurrency_stage1"]; ok {
		concurrency["stage1"] = intValue(value, 200)
	}
	if value, ok := nodeConfig["concurrency_stage2"]; ok {
		concurrency["stage2"] = intValue(value, 30)
	}
	if value, ok := nodeConfig["concurrency_stage3"]; ok {
		concurrency["stage3"] = intValue(value, 1)
	}
	if value, ok := nodeConfig["tcp_port"]; ok {
		probe["tcp_port"] = intValue(value, 443)
	}
	if value, ok := nodeConfig["port_policy"]; ok {
		probe["port_policy"] = strings.TrimSpace(stringValue(value, ""))
	}
	if value, ok := nodeConfig["ping_times"]; ok {
		probe["ping_times"] = intValue(value, 4)
	}
	if value, ok := nodeConfig["min_delay_ms"]; ok {
		probe["min_delay_ms"] = intValue(value, 0)
	}
	if value, ok := nodeConfig["timeout_stage1_ms"]; ok {
		timeouts["stage1_ms"] = intValue(value, 1000)
	}
	if value, ok := nodeConfig["timeout_stage2_ms"]; ok {
		timeouts["stage2_ms"] = intValue(value, 1000)
	}
	if value, ok := nodeConfig["timeout_stage3_ms"]; ok {
		timeouts["stage3_ms"] = intValue(value, 10000)
	}
	if value, ok := nodeConfig["download_speed_metric"]; ok {
		probe["download_speed_metric"] = strings.TrimSpace(stringValue(value, ""))
	}
	if value, ok := nodeConfig["download_count"]; ok {
		count := intValue(value, 0)
		if count > 0 {
			probe["download_count"] = count
			if _, hasStage3Limit := nodeConfig["stage3_limit"]; !hasStage3Limit {
				stageLimits["stage3"] = count
			}
		}
	}
	if value, ok := nodeConfig["stage3_limit"]; ok {
		count := intValue(value, 0)
		if count > 0 {
			stageLimits["stage3"] = count
		}
	}
	if value, ok := nodeConfig["print_num"]; ok {
		probe["print_num"] = intValue(value, 0)
	}
	if value, ok := nodeConfig["download_get_concurrency"]; ok {
		probe["download_get_concurrency"] = intValue(value, 4)
	}
	if value, ok := nodeConfig["download_time_seconds"]; ok {
		probe["download_time_seconds"] = intValue(value, 10)
	}
	if value, ok := nodeConfig["download_warmup_seconds"]; ok {
		probe["download_warmup_seconds"] = intValue(value, 5)
	}
	if value, ok := nodeConfig["download_speed_sample_interval_ms"]; ok {
		probe["download_speed_sample_interval_ms"] = intValue(value, 500)
	}
	if value, ok := nodeConfig["download_buffer_kb"]; ok {
		probe["download_buffer_kb"] = intValue(value, 256)
	}
	if value, ok := nodeConfig["download_http_protocol"]; ok {
		probe["download_http_protocol"] = strings.TrimSpace(stringValue(value, ""))
	}
	if value, ok := nodeConfig["url"]; ok {
		probe["url"] = strings.TrimSpace(stringValue(value, ""))
	}
	if value, ok := nodeConfig["max_loss_rate"]; ok {
		probe["max_loss_rate"] = floatValue(value, 0)
	}
	if value, ok := nodeConfig["max_tcp_latency_ms"]; ok {
		if value == nil {
			thresholds["max_tcp_latency_ms"] = nil
		} else {
			thresholds["max_tcp_latency_ms"] = intValue(value, 0)
		}
	}
	if value, ok := nodeConfig["max_trace_latency_ms"]; ok {
		if value == nil {
			thresholds["max_http_latency_ms"] = nil
		} else {
			thresholds["max_http_latency_ms"] = intValue(value, 0)
		}
	}
	if value, ok := nodeConfig["min_download_mbps"]; ok {
		thresholds["min_download_mbps"] = floatValue(value, 0)
	}
	if value, ok := nodeConfig["trace_url"]; ok {
		probe["trace_url"] = strings.TrimSpace(stringValue(value, ""))
	}
	if value, ok := nodeConfig["trace_colo_mode"]; ok {
		probe["trace_colo_mode"] = strings.TrimSpace(stringValue(value, ""))
	}
	if value, ok := nodeConfig["source_colo_filter_phase"]; ok {
		probe["source_colo_filter_phase"] = strings.TrimSpace(stringValue(value, ""))
	}
	if value, ok := nodeConfig["httping_status_code"]; ok {
		probe["httping_status_code"] = intValue(value, 0)
	}
	if value, ok := nodeConfig["httping_cf_colo"]; ok {
		probe["httping_cf_colo"] = strings.TrimSpace(stringValue(value, ""))
	}
	if value, ok := nodeConfig["httping_cf_colo_mode"]; ok {
		probe["httping_cf_colo_mode"] = strings.TrimSpace(stringValue(value, ""))
	}

	probe["concurrency"] = concurrency
	probe["thresholds"] = thresholds
	probe["stage_limits"] = stageLimits
	probe["timeouts"] = timeouts
	snapshot["probe"] = probe
	return sanitizeDesktopConfigSnapshot(snapshot)
}

func pipelineProbeSourcesForNode(runtimeCtx *pipelineRuntimeContext, node appcore.PipelineNode) []DesktopSource {
	nodeConfig := mapValue(node.Config)
	sourceMode := strings.ToLower(strings.TrimSpace(stringValue(nodeConfig["source_mode"], "inherit")))

	var sources []DesktopSource
	if sourceMode == "custom" {
		sources = desktopSourcesFromAny(nodeConfig["sources"])
	} else if runtimeCtx != nil && runtimeCtx.SelectedSources != nil {
		sources = cloneDesktopSources(runtimeCtx.SelectedSources)
	} else {
		sources = desktopSourcesFromAny(runtimeCtx.ConfigSnapshot["sources"])
	}
	if len(sources) == 0 {
		return nil
	}

	overridden := make([]DesktopSource, 0, len(sources))
	sourceIPLimit, hasSourceIPLimit := nodeConfig["source_ip_limit"]
	sourceIPMode, hasSourceIPMode := nodeConfig["source_ip_mode"]
	sourceColoFilter, hasSourceColoFilter := nodeConfig["source_colo_filter"]
	sourceColoFilterMode, hasSourceColoFilterMode := nodeConfig["source_colo_filter_mode"]

	for _, source := range sources {
		next := source
		if hasSourceIPLimit {
			limit := intValue(sourceIPLimit, next.IPLimit)
			if limit > 0 {
				next.IPLimit = limit
			}
		}
		if hasSourceIPMode {
			next.IPMode = strings.TrimSpace(stringValue(sourceIPMode, next.IPMode))
		}
		if hasSourceColoFilter {
			next.ColoFilter = stringValue(sourceColoFilter, next.ColoFilter)
		}
		if hasSourceColoFilterMode {
			next.ColoFilterMode = strings.TrimSpace(stringValue(sourceColoFilterMode, next.ColoFilterMode))
		}
		overridden = append(overridden, next)
	}
	return overridden
}

func pipelineSourceGroupSourcesForNode(runtimeCtx *pipelineRuntimeContext, node appcore.PipelineNode) ([]DesktopSource, error) {
	if runtimeCtx == nil {
		return nil, nil
	}
	nodeConfig := mapValue(node.Config)
	profileID := strings.TrimSpace(stringValue(nodeConfig["source_profile_id"], ""))
	selectionMode := strings.ToLower(strings.TrimSpace(stringValue(nodeConfig["source_selection"], appcore.PipelineSourceSelectionEnabled)))
	allSources, err := pipelineSourceGroupAllSources(runtimeCtx, profileID)
	if err != nil {
		return nil, err
	}
	if selectionMode != appcore.PipelineSourceSelectionCustom {
		enabled := make([]DesktopSource, 0, len(allSources))
		for _, source := range allSources {
			if source.Enabled {
				enabled = append(enabled, source)
			}
		}
		return enabled, nil
	}
	sourceIDs := stringSliceValue(nodeConfig["source_ids"])
	selectedIDs := make(map[string]struct{}, len(sourceIDs))
	for _, sourceID := range sourceIDs {
		if strings.TrimSpace(sourceID) != "" {
			selectedIDs[strings.TrimSpace(sourceID)] = struct{}{}
		}
	}
	selected := make([]DesktopSource, 0, len(allSources))
	for _, source := range allSources {
		if _, ok := selectedIDs[strings.TrimSpace(source.ID)]; ok {
			selected = append(selected, source)
		}
	}
	return selected, nil
}

func pipelineSourceGroupAllSources(runtimeCtx *pipelineRuntimeContext, profileID string) ([]DesktopSource, error) {
	if strings.TrimSpace(profileID) == "" {
		return desktopSourcesFromAny(runtimeCtx.ConfigSnapshot["sources"]), nil
	}
	store, err := loadSourceProfileStore()
	if err != nil {
		return nil, fmt.Errorf("读取输入组档案失败：%w", err)
	}
	for _, profile := range store.Items {
		if strings.TrimSpace(profile.ID) == profileID {
			return cloneDesktopSources(profile.Sources), nil
		}
	}
	return nil, fmt.Errorf("输入组档案 %s 不存在。", profileID)
}

func pipelineSelectionSnapshotForNode(runtimeCtx *pipelineRuntimeContext, node appcore.PipelineNode) map[string]any {
	snapshot := sanitizeDesktopConfigSnapshot(deepCloneMap(runtimeCtx.ConfigSnapshot))
	nodeConfig := mapValue(node.Config)
	upload := mapValue(snapshot["upload"])
	sharedFilter := mapValue(upload["shared_filter"])
	cloudflare := mapValue(upload["cloudflare"])
	github := mapValue(upload["github"])

	filterKeys := []string{
		"status",
		"ip_version",
		"max_loss_rate",
		"max_tcp_latency_ms",
		"max_trace_latency_ms",
		"min_download_mbps",
		"colo_allow",
		"colo_deny",
	}
	hasFilterOverride := false
	for _, key := range filterKeys {
		if _, ok := nodeConfig[key]; ok {
			hasFilterOverride = true
			break
		}
	}
	if hasFilterOverride {
		sharedFilter["enabled"] = true
	}
	if value, ok := nodeConfig["status"]; ok {
		sharedFilter["status"] = stringValue(value, "passed")
	}
	if value, ok := nodeConfig["ip_version"]; ok {
		sharedFilter["ip_version"] = stringValue(value, "any")
	}
	if value, ok := nodeConfig["max_loss_rate"]; ok {
		sharedFilter["max_loss_rate"] = floatValue(value, 0)
	}
	if value, ok := nodeConfig["max_tcp_latency_ms"]; ok {
		sharedFilter["max_tcp_latency_ms"] = intValue(value, 0)
	}
	if value, ok := nodeConfig["max_trace_latency_ms"]; ok {
		sharedFilter["max_trace_latency_ms"] = intValue(value, 0)
	}
	if value, ok := nodeConfig["min_download_mbps"]; ok {
		sharedFilter["min_download_mbps"] = floatValue(value, 0)
	}
	if value, ok := nodeConfig["colo_allow"]; ok {
		sharedFilter["colo_allow"] = stringValue(value, "")
	}
	if value, ok := nodeConfig["colo_deny"]; ok {
		sharedFilter["colo_deny"] = stringValue(value, "")
	}
	if topN, ok := pipelineTopNOverride(node); ok {
		switch normalizePipelineNodeAction(node.Action) {
		case appcore.PipelineNodeActionDeliverDNS:
			cloudflare["top_n"] = topN
		case appcore.PipelineNodeActionDeliverGitHub:
			github["top_n"] = topN
		default:
			cloudflare["top_n"] = topN
			github["top_n"] = topN
		}
	}

	upload["shared_filter"] = sharedFilter
	upload["cloudflare"] = cloudflare
	upload["github"] = github
	snapshot["upload"] = upload
	return sanitizeDesktopConfigSnapshot(snapshot)
}

func pipelineDNSSnapshotForNode(runtimeCtx *pipelineRuntimeContext, node appcore.PipelineNode) map[string]any {
	snapshot := pipelineSelectionSnapshotForNode(runtimeCtx, node)
	nodeConfig := mapValue(node.Config)
	cloudflare := mapValue(snapshot["cloudflare"])

	if value, ok := nodeConfig["record_name"]; ok {
		recordName := strings.TrimSpace(stringValue(value, ""))
		if recordName != "" {
			cloudflare["record_name"] = recordName
		}
	}
	if value, ok := nodeConfig["record_type"]; ok {
		recordType := strings.ToUpper(strings.TrimSpace(stringValue(value, cloudflareRecordTypeA)))
		if recordType == cloudflareRecordTypeAll {
			cloudflare["record_type"] = cloudflareRecordTypeAll
		} else if recordType == cloudflareRecordTypeAAAA {
			cloudflare["record_type"] = cloudflareRecordTypeAAAA
		} else {
			cloudflare["record_type"] = cloudflareRecordTypeA
		}
	}
	if value, ok := nodeConfig["ttl"]; ok {
		ttl := intValue(value, 0)
		if ttl > 0 {
			cloudflare["ttl"] = ttl
		}
	}
	cloudflare["proxied"] = false
	if value, ok := nodeConfig["comment"]; ok {
		cloudflare["comment"] = stringValue(value, "")
	}

	snapshot["cloudflare"] = cloudflare
	return sanitizeDesktopConfigSnapshot(snapshot)
}

func pipelineTopNOverride(node appcore.PipelineNode) (int, bool) {
	nodeConfig := mapValue(node.Config)
	value, ok := nodeConfig["top_n"]
	if !ok {
		return 0, false
	}
	topN := intValue(value, 0)
	if topN < 0 {
		topN = 0
	}
	return topN, true
}

func pipelineLimitProbeRows(rows []ProbeRow, topN int, metric string) []ProbeRow {
	if len(rows) == 0 {
		return nil
	}
	if topN <= 0 || len(rows) <= topN {
		return append([]ProbeRow{}, rows...)
	}
	selected := probecore.SelectTopProbeRowsByMetric(append([]ProbeRow{}, rows...), topN, metric)
	return append([]ProbeRow{}, selected...)
}

func pipelineProfileFailureStatus(action string, status string) string {
	if normalizePipelineNodeAction(action) == appcore.PipelineNodeActionDeliverDNS {
		return "dns_failed"
	}
	status = normalizePipelineProfileStatus(status)
	if status == "completed" {
		return "failed"
	}
	return status
}

func normalizePipelineProfileStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "cancelled":
		return "cancelled"
	case "dns_failed":
		return "dns_failed"
	case "failed":
		return "failed"
	case "manual_review":
		return "manual_review"
	case "partial":
		return "partial"
	case "skipped":
		return "skipped"
	default:
		return "completed"
	}
}

func pipelineNodeFallbackMessage(node appcore.PipelineNode, status string) string {
	switch normalizePipelineNodeType(node.NodeType) {
	case appcore.PipelineNodeTypeSource:
		return "输入源组已完成。"
	case appcore.PipelineNodeTypeBranch:
		return "分支节点已完成。"
	case appcore.PipelineNodeTypeDeliver:
		if status == "skipped" {
			return "投递节点已跳过。"
		}
		return "投递节点已完成。"
	case appcore.PipelineNodeTypeEnd:
		return "流程已结束。"
	case appcore.PipelineNodeTypeFilter:
		return "筛选节点已完成。"
	case appcore.PipelineNodeTypeRecovery:
		return "恢复节点已完成。"
	default:
		return "节点已完成。"
	}
}

func pipelineDefaultProfileMessage(status string, resultCount int) string {
	switch normalizePipelineProfileStatus(status) {
	case "dns_failed":
		return "DNS 推送失败。"
	case "failed":
		return "流程执行失败。"
	case "manual_review":
		return "流程已结束，等待人工复核。"
	case "partial":
		return "流程部分完成。"
	case "skipped":
		return "流程已跳过。"
	default:
		return fmt.Sprintf("策略完成，可用结果 %d 条。", resultCount)
	}
}

func pipelineRuntimeResultCount(runtimeCtx *pipelineRuntimeContext, node appcore.PipelineNode) int {
	return len(pipelineRowsForNodeSource(runtimeCtx, stringValue(mapValue(node.Config)["source"], "")))
}

func pipelineResultCount(probeResult *ProbeRunResult, filteredRows []ProbeRow) int {
	if len(filteredRows) > 0 {
		return len(filteredRows)
	}
	if probeResult == nil {
		return 0
	}
	return len(probeResult.Results)
}

func pipelineTargetFromProfile(profile PipelineProfile, templateID string) PipelineTarget {
	return PipelineTarget{
		ConfigSnapshot: deepCloneMap(profile.ConfigSnapshot),
		CreatedAt:      profile.CreatedAt,
		DNSPushPolicy:  profile.DNSPushPolicy,
		Domain:         profile.Domain,
		Enabled:        profile.Enabled,
		ID:             profile.ID,
		Name:           profile.Name,
		Region:         profile.Region,
		TemplateID:     firstNonEmptyString(strings.TrimSpace(templateID), appcore.DefaultPipelineTemplateID),
		UpdatedAt:      profile.UpdatedAt,
	}
}

func normalizePipelineNodeAction(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch normalized {
	case appcore.PipelineNodeActionSelectSources, "source_group", "select_source":
		return appcore.PipelineNodeActionSelectSources
	case appcore.PipelineNodeActionFilterSources:
		return appcore.PipelineNodeActionFilterSources
	case appcore.PipelineNodeActionProbeTCP:
		return appcore.PipelineNodeActionProbeTCP
	case appcore.PipelineNodeActionProbeTrace:
		return appcore.PipelineNodeActionProbeTrace
	case appcore.PipelineNodeActionProbeDownload:
		return appcore.PipelineNodeActionProbeDownload
	case appcore.PipelineNodeActionFilterResults, "filter_candidates":
		return appcore.PipelineNodeActionFilterResults
	case appcore.PipelineNodeActionBranchHasResults, "has_results":
		return appcore.PipelineNodeActionBranchHasResults
	case appcore.PipelineNodeActionDeliverDNS, "dns_push":
		return appcore.PipelineNodeActionDeliverDNS
	case appcore.PipelineNodeActionDeliverGitHub, "github_export":
		return appcore.PipelineNodeActionDeliverGitHub
	case appcore.PipelineNodeActionRecoveryMark, "mark_manual_review":
		return appcore.PipelineNodeActionRecoveryMark
	case appcore.PipelineNodeActionCheckOutput:
		return appcore.PipelineNodeActionCheckOutput
	case appcore.PipelineNodeActionEnd, "completed", "manual_review":
		return appcore.PipelineNodeActionEnd
	default:
		if normalized == "" {
			return appcore.PipelineNodeActionProbeTCP
		}
		return normalized
	}
}

func normalizePipelineNodeType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case appcore.PipelineNodeTypeSource:
		return appcore.PipelineNodeTypeSource
	case appcore.PipelineNodeTypeFilter:
		return appcore.PipelineNodeTypeFilter
	case appcore.PipelineNodeTypeBranch:
		return appcore.PipelineNodeTypeBranch
	case appcore.PipelineNodeTypeDeliver:
		return appcore.PipelineNodeTypeDeliver
	case appcore.PipelineNodeTypeRecovery:
		return appcore.PipelineNodeTypeRecovery
	case appcore.PipelineNodeTypeEnd:
		return appcore.PipelineNodeTypeEnd
	default:
		return appcore.PipelineNodeTypeProbe
	}
}

func (a *App) pushPipelineProfileDNS(snapshot map[string]any, probeResult ProbeRunResult) DesktopCommandResult {
	selection, err := BuildUploadSelection(snapshot, probeResult.Results, probeResult.Config.DownloadSpeedMetric)
	if err != nil {
		return desktopCommandResult("DNS_CONFIG_INVALID", nil, err.Error(), false, nil, nil)
	}
	recordType := stringValue(mapValue(snapshot["cloudflare"])["record_type"], cloudflareRecordTypeA)
	rows := filterRowsForCloudflareRecordType(selection.CloudflareRows, recordType)
	if len(rows) == 0 {
		return desktopCommandResult("DNS_INPUT_EMPTY", map[string]any{
			"summary": map[string]any{},
		}, "本策略筛选后没有可推送到 Cloudflare 的 IP。", false, nil, selection.Warnings)
	}
	return a.PushCloudflareDNSRecords(map[string]any{
		"config": snapshot,
		"ipsRaw": probeRowsIPList(rows),
	})
}

func (a *App) pipelineProfilesForRun(payload PipelineRunPayload) ([]PipelineProfile, []string, error) {
	warnings := []string{}
	storeProfiles := payload.Profiles
	if len(storeProfiles) == 0 && (len(payload.TargetIDs) > 0 || len(payload.Workspace.Targets) > 0 || strings.TrimSpace(payload.TemplateID) != "") {
		workspace, workspaceWarnings, err := pipelineWorkspaceForRunPayload(payload)
		warnings = append(warnings, workspaceWarnings...)
		if err != nil {
			return nil, warnings, err
		}
		storeProfiles = pipelineProfilesFromWorkspaceSelection(workspace, payload.TemplateID, payload.TargetIDs)
	}
	if len(storeProfiles) == 0 {
		store, storeWarnings, err := loadPipelineProfileStoreOrDefault()
		warnings = append(warnings, storeWarnings...)
		if err != nil {
			return nil, warnings, err
		}
		storeProfiles = store.Items
	}
	selectedIDs := make(map[string]struct{}, len(payload.ProfileIDs))
	for _, id := range payload.ProfileIDs {
		id = strings.TrimSpace(id)
		if id != "" {
			selectedIDs[id] = struct{}{}
		}
	}
	profiles := make([]PipelineProfile, 0, len(storeProfiles))
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

func loadPipelineProfileStoreOrDefault() (pipelineProfileStore, []string, error) {
	workspace, warnings, err := loadPipelineWorkspaceOrDefault()
	if err != nil {
		return pipelineProfileStore{}, warnings, err
	}
	store := pipelineProfileStoreFromWorkspace(workspace)
	if len(store.Items) > 0 {
		return normalizePipelineProfileStoreForSave(store), warnings, nil
	}
	return store, warnings, nil
}

func loadPipelineWorkspaceOrDefault() (pipelineWorkspace, []string, error) {
	workspace, migrated, err := loadPipelineWorkspace()
	if err != nil {
		return workspace, nil, err
	}
	if len(workspace.Targets) > 0 && len(workspace.Templates) > 0 {
		workspace = normalizePipelineWorkspaceForSave(workspace)
		if migrated {
			if err := savePipelineWorkspace(workspace); err != nil {
				return workspace, []string{"已识别旧版策略数据，但写入新工作流文件失败。"}, nil
			}
			return workspace, []string{"已从 pipeline-profiles.json 自动迁移到 pipeline-workspace.json。"}, nil
		}
		return workspace, nil, nil
	}
	snapshot, snapshotErr := loadDesktopConfigSnapshotFromDisk()
	if snapshotErr != nil {
		if !errors.Is(snapshotErr, os.ErrNotExist) {
			return workspace, nil, snapshotErr
		}
		snapshot = defaultDesktopConfigSnapshot()
	}
	return defaultPipelineWorkspaceFromSnapshot(snapshot), nil, nil
}

func defaultPipelineWorkspaceFromSnapshot(snapshot map[string]any) pipelineWorkspace {
	return appcore.DefaultPipelineWorkspaceFromSnapshot(snapshot, pipelineWorkspaceSchemaVersion, time.Now().Format(time.RFC3339), sanitizeDesktopConfigSnapshot)
}

func normalizePipelineWorkspaceForSave(workspace pipelineWorkspace) pipelineWorkspace {
	return appcore.NormalizePipelineWorkspaceForSave(workspace, pipelineWorkspaceSchemaVersion, time.Now().Format(time.RFC3339), sanitizeDesktopConfigSnapshot, func(index int) string {
		return fmt.Sprintf("pipeline-template-%d", time.Now().UnixNano()+int64(index))
	}, func(index int) string {
		return fmt.Sprintf("pipeline-target-%d", time.Now().UnixNano()+int64(index))
	})
}

func pipelineProfileStoreFromWorkspace(workspace pipelineWorkspace) pipelineProfileStore {
	return appcore.LegacyPipelineProfileStoreFromWorkspace(workspace, pipelineProfilesSchemaVersion, time.Now().Format(time.RFC3339), sanitizeDesktopConfigSnapshot)
}

func applyLegacyProfileStoreToWorkspace(workspace pipelineWorkspace, store pipelineProfileStore) pipelineWorkspace {
	workspace = normalizePipelineWorkspaceForSave(workspace)
	next := appcore.PipelineWorkspaceFromProfileStore(store, pipelineWorkspaceSchemaVersion, time.Now().Format(time.RFC3339), sanitizeDesktopConfigSnapshot)
	if len(workspace.Templates) > 0 {
		next.Templates = workspace.Templates
		next.ActiveTemplateID = firstNonEmptyString(workspace.ActiveTemplateID, next.ActiveTemplateID)
	}
	existingTargets := make(map[string]pipelineTargetItem, len(workspace.Targets))
	for _, item := range workspace.Targets {
		existingTargets[item.ID] = item
	}
	for index := range next.Targets {
		if existing, ok := existingTargets[next.Targets[index].ID]; ok {
			next.Targets[index].TemplateID = firstNonEmptyString(existing.TemplateID, next.Targets[index].TemplateID, next.ActiveTemplateID, appcore.DefaultPipelineTemplateID)
			next.Targets[index].Tags = append([]string{}, existing.Tags...)
		} else if strings.TrimSpace(next.Targets[index].TemplateID) == "" {
			next.Targets[index].TemplateID = firstNonEmptyString(next.ActiveTemplateID, appcore.DefaultPipelineTemplateID)
		}
	}
	if strings.TrimSpace(store.ActiveProfileID) != "" {
		next.ActiveTargetID = strings.TrimSpace(store.ActiveProfileID)
	}
	return normalizePipelineWorkspaceForSave(next)
}

func defaultPipelineProfileStoreFromSnapshot(snapshot map[string]any) pipelineProfileStore {
	return appcore.DefaultPipelineProfileStoreFromSnapshot(snapshot, pipelineProfilesSchemaVersion, time.Now().Format(time.RFC3339), sanitizeDesktopConfigSnapshot)
}

func normalizePipelineProfileStoreForSave(store pipelineProfileStore) pipelineProfileStore {
	return appcore.NormalizePipelineProfileStoreForSave(store, pipelineProfilesSchemaVersion, time.Now().Format(time.RFC3339), sanitizeDesktopConfigSnapshot, func(index int) string {
		return fmt.Sprintf("pipeline-profile-%d", time.Now().UnixNano()+int64(index))
	})
}

func normalizePipelineRunPayload(payload PipelineRunPayload) PipelineRunPayload {
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

func pipelineProfileFromPayload(payload map[string]any) PipelineProfile {
	rawProfile := mapValue(firstNonNil(payload["profile"], payload["item"]))
	if len(rawProfile) == 0 {
		rawProfile = payload
	}
	profiles := appcore.PipelineProfilesFromAny([]any{rawProfile})
	if len(profiles) == 0 {
		return PipelineProfile{}
	}
	return profiles[0]
}

func pipelinePayloadHasEnabled(payload map[string]any) bool {
	if _, ok := payload["enabled"]; ok {
		return true
	}
	profile := mapValue(firstNonNil(payload["profile"], payload["item"]))
	_, ok := profile["enabled"]
	return ok
}

func pipelineTargetPayloadHasEnabled(payload map[string]any) bool {
	if _, ok := payload["enabled"]; ok {
		return true
	}
	target := mapValue(firstNonNil(payload["target"], payload["item"], payload["profile"]))
	_, ok := target["enabled"]
	return ok
}

func pipelineWorkspaceFromPayload(payload map[string]any) pipelineWorkspace {
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

func pipelineWorkspaceFromAny(value any) pipelineWorkspace {
	return appcore.PipelineWorkspaceFromAny(value)
}

func pipelineTemplateFromPayload(payload map[string]any) PipelineTemplate {
	rawTemplate := mapValue(firstNonNil(payload["template"], payload["item"]))
	if len(rawTemplate) == 0 {
		rawTemplate = payload
	}
	templates := appcore.PipelineTemplatesFromAny([]any{rawTemplate})
	if len(templates) == 0 {
		return PipelineTemplate{}
	}
	return templates[0]
}

func pipelineTargetFromPayload(payload map[string]any) PipelineTarget {
	rawTarget := mapValue(firstNonNil(payload["target"], payload["item"]))
	if len(rawTarget) == 0 {
		rawTarget = payload
	}
	targets := appcore.PipelineTargetsFromAny([]any{rawTarget})
	if len(targets) == 0 {
		profile := pipelineProfileFromPayload(payload)
		return PipelineTarget{
			ConfigSnapshot: profile.ConfigSnapshot,
			CreatedAt:      profile.CreatedAt,
			DNSPushPolicy:  profile.DNSPushPolicy,
			Domain:         profile.Domain,
			Enabled:        profile.Enabled,
			ID:             profile.ID,
			Name:           profile.Name,
			Region:         profile.Region,
			TemplateID:     firstNonEmptyString(strings.TrimSpace(stringValue(firstNonNil(payload["template_id"], payload["templateId"]), "")), appcore.DefaultPipelineTemplateID),
			UpdatedAt:      profile.UpdatedAt,
		}
	}
	return targets[0]
}

func pipelineWorkspaceForRunPayload(payload PipelineRunPayload) (pipelineWorkspace, []string, error) {
	if len(payload.Workspace.Templates) > 0 || len(payload.Workspace.Targets) > 0 {
		return normalizePipelineWorkspaceForSave(payload.Workspace), nil, nil
	}
	workspace, warnings, err := loadPipelineWorkspaceOrDefault()
	if err != nil {
		return pipelineWorkspace{}, warnings, err
	}
	return workspace, warnings, nil
}

func pipelineProfilesFromWorkspaceSelection(workspace pipelineWorkspace, templateID string, targetIDs []string) []PipelineProfile {
	selectedIDs := make(map[string]struct{}, len(targetIDs))
	for _, id := range targetIDs {
		id = strings.TrimSpace(id)
		if id != "" {
			selectedIDs[id] = struct{}{}
		}
	}
	templateID = strings.TrimSpace(templateID)
	profiles := make([]PipelineProfile, 0, len(workspace.Targets))
	for _, target := range workspace.Targets {
		if len(selectedIDs) > 0 {
			if _, ok := selectedIDs[target.ID]; !ok {
				continue
			}
		}
		if templateID != "" && strings.TrimSpace(target.TemplateID) != templateID {
			continue
		}
		profiles = append(profiles, PipelineProfile{
			ConfigSnapshot: deepCloneMap(target.ConfigSnapshot),
			CreatedAt:      target.CreatedAt,
			DNSPushPolicy:  target.DNSPushPolicy,
			Domain:         target.Domain,
			Enabled:        true,
			ID:             target.ID,
			Name:           target.Name,
			Region:         target.Region,
			UpdatedAt:      target.UpdatedAt,
		})
	}
	return profiles
}

func pipelineTemplateForRunPayload(payload PipelineRunPayload) (pipelineTemplateItem, []string, error) {
	workspace, warnings, err := pipelineWorkspaceForRunPayload(payload)
	if err != nil {
		return pipelineTemplateItem{}, warnings, err
	}
	templateID := strings.TrimSpace(payload.TemplateID)
	if templateID == "" {
		templateID = firstNonEmptyString(workspace.ActiveTemplateID, appcore.DefaultPipelineTemplateID)
	}
	for _, template := range workspace.Templates {
		if strings.TrimSpace(template.ID) != templateID {
			continue
		}
		if err := appcore.ValidatePipelineTemplate(template); err != nil {
			return pipelineTemplateItem{}, warnings, err
		}
		return template, warnings, nil
	}
	if len(workspace.Templates) == 1 && templateID == "" {
		if err := appcore.ValidatePipelineTemplate(workspace.Templates[0]); err != nil {
			return pipelineTemplateItem{}, warnings, err
		}
		return workspace.Templates[0], warnings, nil
	}
	return pipelineTemplateItem{}, warnings, fmt.Errorf("未找到工作流模板 %s", templateID)
}

func pipelineSnapshotForRun(profile PipelineProfile, taskID string) map[string]any {
	snapshot := sanitizeDesktopConfigSnapshot(deepCloneMap(profile.ConfigSnapshot))
	exportCfg := mapValue(snapshot["export"])
	template := strings.TrimSpace(stringValue(firstNonNil(exportCfg["file_name_template"], exportCfg["fileNameTemplate"]), ""))
	if template == "" {
		exportCfg["file_name_template"] = "result-{profile}-{task_id}.csv"
	}
	snapshot["export"] = exportCfg
	cloudflare := mapValue(snapshot["cloudflare"])
	if strings.TrimSpace(profile.Domain) != "" {
		cloudflare["record_name"] = strings.TrimSpace(profile.Domain)
	}
	snapshot["cloudflare"] = cloudflare
	_ = taskID
	return snapshot
}

func pipelineProfileTaskID(pipelineID string, profile PipelineProfile, index int) string {
	safeID := probecore.SanitizeTemplateFileName(profile.ID)
	if safeID == "" {
		safeID = fmt.Sprintf("profile-%d", index+1)
	}
	return fmt.Sprintf("%s-%02d-%s", probecore.SanitizeTemplateFileName(pipelineID), index+1, safeID)
}

func pipelineProbeMetadata(payload DesktopProbePayload) map[string]any {
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

func pipelineProfileEventPayload(profile PipelineProfile, taskID string, extra map[string]any) map[string]any {
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

func (e *pipelineEventEmitter) emit(event string, payload map[string]any) {
	if e == nil || e.app == nil {
		return
	}
	if payload == nil {
		payload = map[string]any{}
	}
	if _, ok := payload["pipeline_id"]; !ok {
		payload["pipeline_id"] = e.pipelineID
	}
	e.seq++
	e.app.emitProbeEvent(desktopProbeEventEnvelope{
		Event:         event,
		Payload:       payload,
		SchemaVersion: guiSchemaVersion,
		Seq:           e.seq,
		TaskID:        e.pipelineID,
		TS:            time.Now().Format(time.RFC3339),
	})
}

func (a *App) claimPipeline(pipelineID string) (bool, string) {
	a.pipelineMu.Lock()
	defer a.pipelineMu.Unlock()
	if a.currentPipelineID != "" {
		return false, a.currentPipelineID
	}
	if currentTaskID := a.currentProbeRuntimeTaskID(); currentTaskID != "" {
		return false, currentTaskID
	}
	a.currentPipelineID = pipelineID
	a.currentPipelineCancel = false
	a.pipelineResults = map[string]appcore.PipelineRunResult{}
	return true, pipelineID
}

func (a *App) clearPipeline(pipelineID string) {
	a.pipelineMu.Lock()
	defer a.pipelineMu.Unlock()
	if a.currentPipelineID == pipelineID {
		a.currentPipelineID = ""
		a.currentPipelineCancel = false
	}
}

func (a *App) canStartProbeForPipeline(ownerPipelineID string) bool {
	a.pipelineMu.Lock()
	defer a.pipelineMu.Unlock()
	return a.currentPipelineID == "" || (ownerPipelineID != "" && ownerPipelineID == a.currentPipelineID)
}

func (a *App) activePipelineID() string {
	a.pipelineMu.Lock()
	defer a.pipelineMu.Unlock()
	return a.currentPipelineID
}

func (a *App) hasActivePipelineTask() bool {
	a.pipelineMu.Lock()
	defer a.pipelineMu.Unlock()
	return a.currentPipelineID != ""
}

func (a *App) isPipelineCancelRequested(pipelineID string) bool {
	a.pipelineMu.Lock()
	defer a.pipelineMu.Unlock()
	return a.currentPipelineID == pipelineID && a.currentPipelineCancel
}

func (a *App) rememberPipelineResult(result PipelineRunResult) {
	a.pipelineMu.Lock()
	defer a.pipelineMu.Unlock()
	a.pipelineResults = map[string]appcore.PipelineRunResult{
		result.PipelineID: result,
	}
}

func (a *App) currentProbeRuntimeTaskID() string {
	a.ensureProbeControl()
	a.probeControlMu.Lock()
	defer a.probeControlMu.Unlock()
	if strings.TrimSpace(a.currentTaskID) != "" {
		return a.currentTaskID
	}
	if strings.TrimSpace(a.pausedTaskID) != "" || a.pauseRequested {
		return a.pausedTaskID
	}
	return ""
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func pipelineDomainFromSnapshot(snapshot map[string]any) string {
	cloudflare := mapValue(snapshot["cloudflare"])
	return strings.TrimSpace(stringValue(firstNonNil(cloudflare["record_name"], cloudflare["recordName"]), ""))
}

func deepCloneMap(input map[string]any) map[string]any {
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
