package app

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/appcore"
	"github.com/axuitomo/CFST-GUI/internal/probecore"
)

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
	cleared := false
	if a.currentPipelineID == pipelineID {
		a.currentPipelineID = ""
		a.currentPipelineCancel = false
		cleared = true
	}
	a.pipelineMu.Unlock()
	if cleared {
		a.triggerRuntimeCleanupAfterTask()
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
