package app

import (
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/appcore"
)

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
	result.TargetResults = slices.Clone(result.Results)
}

func pipelineTemplateHasAction(template pipelineTemplateItem, action string) bool {
	action = strings.TrimSpace(action)
	for _, node := range template.Nodes {
		if appcore.NormalizePipelineNodeAction(node.Action) == action {
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
