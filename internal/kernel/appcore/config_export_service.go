package appcore

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (s *Service) invokeConfigExport(payloadJSON string) CommandResult {
	payload, err := decodeCommandObject(payloadJSON)
	if err != nil {
		return NewCommandResult("CONFIG_EXPORT_INVALID", nil, err.Error(), false, nil, nil)
	}
	snapshot, result := s.configSnapshotForExport(payload)
	if !result.OK {
		return result
	}
	_, sourceProfiles, profilesResult := s.loadSourceProfileContext()
	if !profilesResult.OK {
		return NewCommandResult("CONFIG_EXPORT_SOURCE_PROFILE_FAILED", nil, profilesResult.Message, false, nil, nil)
	}
	body := map[string]any{
		"app_version":     s.archiveAppVersion(),
		"config_snapshot": snapshot,
		"exported_at":     s.now().Format(time.RFC3339),
		"source_profiles": sourceProfiles,
		"schema_version":  ConfigSchemaVersion,
		"storage":         s.archiveStorageState(),
	}
	raw, err := json.MarshalIndent(body, "", "  ")
	if err != nil {
		return NewCommandResult("CONFIG_EXPORT_SERIALIZE_FAILED", nil, err.Error(), false, nil, nil)
	}
	targetPath := strings.TrimSpace(stringValue(firstNonNil(payload["target_path"], payload["targetPath"], payload["path"]), ""))
	if targetPath != "" {
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return NewCommandResult("CONFIG_EXPORT_WRITE_FAILED", nil, err.Error(), false, nil, nil)
		}
		if err := WriteFileAtomic(targetPath, raw, 0o600); err != nil {
			return NewCommandResult("CONFIG_EXPORT_WRITE_FAILED", nil, err.Error(), false, nil, nil)
		}
	}
	return NewCommandResult("CONFIG_EXPORT_OK", map[string]any{
		"content": string(raw),
		"path":    targetPath,
	}, "完整配置已导出。", true, nil, []string{"导出的配置包含完整 Cloudflare API Token，请仅保存到可信位置。"})
}

func (s *Service) invokeConfigBackup(payloadJSON string) CommandResult {
	payload, err := decodeCommandObject(payloadJSON)
	if err != nil {
		return NewCommandResult("CONFIG_BACKUP_INVALID", nil, err.Error(), false, nil, nil)
	}
	snapshot, result := s.configSnapshotForExport(payload)
	if !result.OK {
		return NewCommandResult("CONFIG_BACKUP_READ_FAILED", nil, result.Message, false, nil, nil)
	}
	now := s.now()
	body := map[string]any{
		"backed_up_at":    now.Format(time.RFC3339),
		"config_snapshot": snapshot,
		"schema_version":  ConfigSchemaVersion,
	}
	raw, err := json.MarshalIndent(body, "", "  ")
	if err != nil {
		return NewCommandResult("CONFIG_BACKUP_SERIALIZE_FAILED", nil, err.Error(), false, nil, nil)
	}
	targetDir := s.StorageLayout().BackupsRoot()
	targetPath := filepath.Join(targetDir, fmt.Sprintf("config-%s.json", now.Format("20060102-150405")))
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return NewCommandResult("CONFIG_BACKUP_WRITE_FAILED", nil, err.Error(), false, nil, nil)
	}
	if err := WriteFileAtomic(targetPath, raw, 0o600); err != nil {
		return NewCommandResult("CONFIG_BACKUP_WRITE_FAILED", nil, err.Error(), false, nil, nil)
	}
	return NewCommandResult("CONFIG_BACKUP_OK", map[string]any{"path": targetPath}, "当前配置已备份。", true, nil, nil)
}

func (s *Service) configSnapshotForExport(payload map[string]any) (map[string]any, CommandResult) {
	snapshot := mapValue(firstNonNil(payload["config_snapshot"], payload["configSnapshot"]))
	if len(snapshot) == 0 {
		loaded, err := s.LoadConfig()
		if err != nil {
			return nil, NewCommandResult("CONFIG_EXPORT_READ_FAILED", nil, err.Error(), false, nil, nil)
		}
		return loaded.Snapshot, CommandResult{OK: true}
	}
	s.mu.RLock()
	policy := s.options.ConfigPolicy
	s.mu.RUnlock()
	return sanitizeServiceConfig(snapshot, policy), CommandResult{OK: true}
}
