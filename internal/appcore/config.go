package appcore

import (
	"encoding/json"
	"errors"
	"os"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/configvalue"
)

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
	snapshot = sanitize(snapshot)
	body := map[string]any{
		"config_snapshot": snapshot,
		"saved_at":        time.Now().Format(time.RFC3339),
		"schema_version":  schemaVersion,
	}
	raw, err := json.MarshalIndent(body, "", "  ")
	if err != nil {
		return err
	}
	return WriteFileAtomic(path, raw, 0o600)
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
