package appcore

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/configvalue"
	"github.com/axuitomo/CFST-GUI/internal/probecore"
)

func (s *Service) invokeConfig(command, payloadJSON string) CommandResult {
	switch command {
	case "config.load":
		if _, err := decodeCommandPayload[struct{}](payloadJSON); err != nil {
			return NewCommandResult("COMMAND_PAYLOAD_INVALID", nil, err.Error(), false, nil, nil)
		}
		loaded, err := s.LoadConfig()
		if err != nil {
			var repositoryErr *ConfigRepositoryError
			if errors.As(err, &repositoryErr) && repositoryErr.Operation == "parse" {
				return NewCommandResult("CONFIG_PARSE_FAILED", nil, err.Error(), false, nil, nil)
			}
			return NewCommandResult("CONFIG_READ_FAILED", nil, err.Error(), false, nil, nil)
		}
		warnings := make([]string, 0)
		if loaded.CompatInfo.IgnoredTrailingContent {
			warnings = append(warnings, "检测到配置文件尾部存在残留内容，已自动忽略。建议重新保存配置。")
		}
		warnings = append(warnings, s.configSnapshotWarnings(loaded.Snapshot)...)
		profiles := s.LoadSourceProfiles()
		if !profiles.OK {
			warnings = append(warnings, profiles.Message)
		}
		data := map[string]any{
			"configPath":      loaded.Path,
			"config_snapshot": loaded.Snapshot,
			"draft_status":    s.DraftStatus().Payload(),
			"source_profiles": profiles.Data,
			"storage":         s.archiveStorageState(),
		}
		code, message := "CONFIG_READ_OK", "配置已加载。"
		if !loaded.Existed {
			code, message = "CONFIG_READY", "配置文件尚未创建，已加载默认配置。"
		}
		return NewCommandResult(code, data, message, true, nil, warnings)
	case "config.save":
		payload, err := decodeCommandObject(payloadJSON)
		if err != nil {
			return NewCommandResult("CONFIG_INVALID", nil, err.Error(), false, nil, nil)
		}
		snapshot := mapValue(firstNonNil(payload["config_snapshot"], payload["configSnapshot"]))
		if len(snapshot) == 0 {
			return NewCommandResult("CONFIG_INVALID", nil, "缺少 config_snapshot。", false, nil, nil)
		}
		snapshot, err = s.SaveConfig(snapshot)
		if err != nil {
			return NewCommandResult("CONFIG_WRITE_FAILED", nil, err.Error(), false, nil, nil)
		}
		if err := s.DiscardDraft(); err != nil {
			return NewCommandResult("CONFIG_WRITE_FAILED", nil, fmt.Sprintf("配置已保存，但清理草稿失败：%v", err), false, nil, nil)
		}
		profiles := s.LoadSourceProfiles()
		warnings := s.configSnapshotWarnings(snapshot)
		if !profiles.OK {
			warnings = append(warnings, profiles.Message)
		}
		schedulerStatus, schedulerErr := s.RefreshSchedulerStatus(snapshot)
		if schedulerErr != nil {
			warnings = append(warnings, fmt.Sprintf("刷新定时任务状态失败：%v", schedulerErr))
		}
		s.mu.RLock()
		savedHook := s.options.ConfigSavedHook
		s.mu.RUnlock()
		if savedHook != nil {
			savedHook(cloneServiceMap(snapshot))
		}
		return NewCommandResult("CONFIG_SAVE_OK", map[string]any{
			"configPath":       s.StorageLayout().ConfigPath(),
			"config_snapshot":  snapshot,
			"draft_status":     s.DraftStatus().Payload(),
			"scheduler_status": schedulerStatus,
			"source_profiles":  profiles.Data,
			"storage":          s.archiveStorageState(),
		}, "配置已保存。", true, nil, warnings)
	default:
		return NewCommandResult("COMMAND_UNKNOWN", nil, fmt.Sprintf("unknown command: %s", strings.TrimSpace(command)), false, nil, nil)
	}
}

func (s *Service) configSnapshotWarnings(snapshot map[string]any) []string {
	s.mu.RLock()
	options := s.options.ProbeConfigOptions
	s.mu.RUnlock()
	options.Now = s.now()
	_, warnings := probecore.ConfigSnapshotToProbeConfig(snapshot, options)
	return warnings
}

type ConfigLoadResult struct {
	CompatInfo JSONCompatInfo
	Existed    bool
	Path       string
	SavedAt    time.Time
	Snapshot   map[string]any
}

type ConfigRepositoryError struct {
	Operation string
	Err       error
}

func (err *ConfigRepositoryError) Error() string { return err.Err.Error() }
func (err *ConfigRepositoryError) Unwrap() error { return err.Err }

func (s *Service) LoadConfig() (ConfigLoadResult, error) {
	s.mu.RLock()
	layout := s.options.Storage
	policy := s.options.ConfigPolicy
	s.mu.RUnlock()
	return loadServiceConfig(layout.ConfigPath(), policy)
}

func (s *Service) SaveConfig(snapshot map[string]any) (map[string]any, error) {
	s.mu.RLock()
	layout := s.options.Storage
	policy := s.options.ConfigPolicy
	clock := s.options.Clock
	s.mu.RUnlock()
	snapshot = sanitizeServiceConfig(snapshot, policy)
	if err := writeConfigSnapshotV2At(layout.ConfigPath(), snapshot, func(value map[string]any) map[string]any {
		return sanitizeServiceConfig(value, policy)
	}, clock()); err != nil {
		return nil, err
	}
	return snapshot, nil
}

func (s *Service) LoadDraft() (ConfigLoadResult, error) {
	s.mu.RLock()
	layout := s.options.Storage
	policy := s.options.ConfigPolicy
	s.mu.RUnlock()
	return loadServiceConfig(layout.DraftPath(), policy)
}

func (s *Service) SaveDraft(snapshot map[string]any) (map[string]any, error) {
	s.mu.RLock()
	layout := s.options.Storage
	policy := s.options.ConfigPolicy
	clock := s.options.Clock
	s.mu.RUnlock()
	snapshot = sanitizeServiceConfig(snapshot, policy)
	if err := writeConfigSnapshotV2At(layout.DraftPath(), snapshot, func(value map[string]any) map[string]any {
		return sanitizeServiceConfig(value, policy)
	}, clock()); err != nil {
		return nil, err
	}
	return snapshot, nil
}

func (s *Service) DiscardDraft() error {
	path := s.StorageLayout().DraftPath()
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func loadServiceConfig(path string, policy ConfigPolicy) (ConfigLoadResult, error) {
	result := ConfigLoadResult{Path: path}
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			result.Snapshot = sanitizeServiceConfig(nil, policy)
			return result, nil
		}
		return result, &ConfigRepositoryError{Operation: "read", Err: err}
	}
	result.Existed = true
	var saved map[string]any
	compatInfo, err := UnmarshalJSONCompat(raw, &saved)
	if err != nil {
		return result, &ConfigRepositoryError{Operation: "parse", Err: err}
	}
	result.CompatInfo = compatInfo
	if savedAt := stringValue(saved["saved_at"], ""); savedAt != "" {
		result.SavedAt, _ = time.Parse(time.RFC3339, savedAt)
	}
	if snapshot := mapValue(saved["config_snapshot"]); len(snapshot) > 0 {
		result.Snapshot = sanitizeServiceConfig(snapshot, policy)
	} else {
		result.Snapshot = sanitizeServiceConfig(saved, policy)
	}
	return result, nil
}

func sanitizeServiceConfig(snapshot map[string]any, policy ConfigPolicy) map[string]any {
	if len(snapshot) == 0 && policy.DefaultSnapshot != nil {
		snapshot = policy.DefaultSnapshot()
	}
	if snapshot == nil {
		snapshot = map[string]any{}
	}
	if policy.SanitizeSnapshot != nil {
		return policy.SanitizeSnapshot(snapshot)
	}
	return snapshot
}

func LoadConfigSnapshotFromDisk(path string, defaultSnapshot func() map[string]any, sanitize func(map[string]any) map[string]any) (map[string]any, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return sanitize(defaultSnapshot()), nil
		}
		return nil, err
	}
	var saved map[string]any
	if _, err := UnmarshalJSONCompat(raw, &saved); err != nil {
		return nil, err
	}
	if snapshot := mapValue(saved["config_snapshot"]); len(snapshot) > 0 {
		return sanitize(snapshot), nil
	}
	return sanitize(saved), nil
}

func WriteConfigSnapshot(path string, snapshot map[string]any, schemaVersion string, sanitize func(map[string]any) map[string]any) error {
	return writeConfigSnapshotAt(path, snapshot, schemaVersion, sanitize, time.Now())
}

func writeConfigSnapshotAt(path string, snapshot map[string]any, schemaVersion string, sanitize func(map[string]any) map[string]any, now time.Time) error {
	snapshot = sanitize(snapshot)
	body := map[string]any{
		"config_snapshot": snapshot,
		"saved_at":        now.Format(time.RFC3339),
		"schema_version":  schemaVersion,
	}
	raw, err := json.MarshalIndent(body, "", "  ")
	if err != nil {
		return err
	}
	return WriteFileAtomic(path, raw, 0o600)
}

func WriteConfigSnapshotV2(path string, snapshot map[string]any, sanitize func(map[string]any) map[string]any) error {
	return writeConfigSnapshotV2At(path, snapshot, sanitize, time.Now())
}

func writeConfigSnapshotV2At(path string, snapshot map[string]any, sanitize func(map[string]any) map[string]any, now time.Time) error {
	if err := backupLegacyConfigOnce(path); err != nil {
		return err
	}
	return writeConfigSnapshotAt(path, snapshot, ConfigSchemaVersion, sanitize, now)
}

func backupLegacyConfigOnce(path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	var envelope struct {
		SchemaVersion string `json:"schema_version"`
	}
	if json.Unmarshal(raw, &envelope) == nil && envelope.SchemaVersion == ConfigSchemaVersion {
		return nil
	}
	backupPath := path + ".v1.bak"
	if _, err := os.Stat(backupPath); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return WriteFileAtomic(backupPath, raw, 0o600)
}

func mapValue(value any) map[string]any {
	return configvalue.Map(value)
}

func firstNonNil(values ...any) any {
	return configvalue.FirstNonNil(values...)
}

func firstPresent(source map[string]any, keys ...string) (any, bool) {
	for _, key := range keys {
		if value, ok := source[key]; ok && value != nil {
			return value, true
		}
	}
	return nil, false
}

func stringValue(value any, fallback string) string {
	return configvalue.String(value, fallback)
}
