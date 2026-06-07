package app

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/appcore"
)

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

func pipelineProfileFromPayload(payload map[string]any) (PipelineProfile, error) {
	rawProfile := mapValue(firstNonNil(payload["profile"], payload["item"]))
	if len(rawProfile) == 0 {
		rawProfile = payload
	}
	profiles, err := appcore.ParsePipelineProfiles([]any{rawProfile})
	if err != nil {
		return PipelineProfile{}, err
	}
	if len(profiles) == 0 {
		return PipelineProfile{}, nil
	}
	return profiles[0], nil
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

func pipelineWorkspaceFromPayload(payload map[string]any) (pipelineWorkspace, error) {
	rawWorkspace := firstNonNil(payload["workspace"], payload["pipeline_workspace"], payload["pipelineWorkspace"])
	workspace, err := appcore.ParsePipelineWorkspace(rawWorkspace)
	if err != nil {
		return pipelineWorkspace{}, err
	}
	if len(workspace.Templates) == 0 && len(workspace.Targets) == 0 {
		templates, err := appcore.ParsePipelineTemplates(firstNonNil(payload["templates"], payload["pipeline_templates"], payload["pipelineTemplates"]))
		if err != nil {
			return pipelineWorkspace{}, err
		}
		targets, err := appcore.ParsePipelineTargets(firstNonNil(payload["targets"], payload["pipeline_targets"], payload["pipelineTargets"]))
		if err != nil {
			return pipelineWorkspace{}, err
		}
		workspace.Templates = templates
		workspace.Targets = targets
		workspace.ActiveTemplateID = strings.TrimSpace(stringValue(firstNonNil(payload["active_template_id"], payload["activeTemplateId"]), ""))
		workspace.ActiveTargetID = strings.TrimSpace(stringValue(firstNonNil(payload["active_target_id"], payload["activeTargetId"]), ""))
		workspace.SchemaVersion = strings.TrimSpace(stringValue(firstNonNil(payload["schema_version"], payload["schemaVersion"]), ""))
		workspace.UpdatedAt = strings.TrimSpace(stringValue(firstNonNil(payload["updated_at"], payload["updatedAt"]), ""))
	}
	return workspace, nil
}

func pipelineWorkspaceFromAny(value any) pipelineWorkspace {
	return appcore.PipelineWorkspaceFromAny(value)
}

func pipelineTemplateFromPayload(payload map[string]any) (PipelineTemplate, error) {
	rawTemplate := mapValue(firstNonNil(payload["template"], payload["item"]))
	if len(rawTemplate) == 0 {
		rawTemplate = payload
	}
	templates, err := appcore.ParsePipelineTemplates([]any{rawTemplate})
	if err != nil {
		return PipelineTemplate{}, err
	}
	if len(templates) == 0 {
		return PipelineTemplate{}, nil
	}
	return templates[0], nil
}

func pipelineTargetFromPayload(payload map[string]any) (PipelineTarget, error) {
	rawTarget := mapValue(firstNonNil(payload["target"], payload["item"]))
	if len(rawTarget) == 0 {
		rawTarget = payload
	}
	targets, err := appcore.ParsePipelineTargets([]any{rawTarget})
	if err != nil {
		return PipelineTarget{}, err
	}
	if len(targets) == 0 {
		profile, err := pipelineProfileFromPayload(payload)
		if err != nil {
			return PipelineTarget{}, err
		}
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
		}, nil
	}
	return targets[0], nil
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
			Enabled:        target.Enabled,
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
