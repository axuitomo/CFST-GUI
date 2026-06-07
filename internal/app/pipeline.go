package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/appcore"
	"github.com/axuitomo/CFST-GUI/internal/utils"
)

type pipelineEventEmitter struct {
	app        *App
	pipelineID string
	seq        int
}

type pipelineRuntimeContext struct {
	ConfigSnapshot     map[string]any
	DNSResult          any
	FilteredRows       []ProbeRow
	NodeOutputs        map[string]any
	Payload            PipelineRunPayload
	ProbeStage         *pipelineProbeStageState
	ProbeStageSnapshot map[string]any
	ProbeResult        *ProbeRunResult
	Profile            PipelineProfile
	SchedulerOverrides appcore.PipelineRuntimeOverrides
	SelectedSources    []DesktopSource
	TaskID             string
	Target             PipelineTarget
	Template           pipelineTemplateItem
	Warnings           []string
}

func (ctx *pipelineRuntimeContext) nodeOutput(nodeID string) (any, bool) {
	output, ok := ctx.NodeOutputs[nodeID]
	return output, ok
}

func (ctx *pipelineRuntimeContext) putNodeOutput(nodeID string, output any) {
	ctx.NodeOutputs[nodeID] = output
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
	workspace, err := pipelineWorkspaceFromPayload(payload)
	if err != nil {
		return desktopCommandResult("PIPELINE_WORKSPACE_INVALID", nil, err.Error(), false, nil, nil)
	}
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
	template, err := pipelineTemplateFromPayload(payload)
	if err != nil {
		return desktopCommandResult("PIPELINE_WORKSPACE_INVALID", nil, err.Error(), false, nil, warnings)
	}
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
	target, err := pipelineTargetFromPayload(payload)
	if err != nil {
		return desktopCommandResult("PIPELINE_WORKSPACE_INVALID", nil, err.Error(), false, nil, warnings)
	}
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
	store, err := appcore.ParsePipelineProfileStore(rawStore)
	if err != nil {
		return desktopCommandResult("PIPELINE_PROFILE_INVALID", nil, err.Error(), false, nil, nil)
	}
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
	profile, err := pipelineProfileFromPayload(payload)
	if err != nil {
		return desktopCommandResult("PIPELINE_PROFILE_INVALID", nil, err.Error(), false, nil, nil)
	}
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
