package probecore

import (
	"encoding/json"
	"fmt"
	"strings"
)

func firstExistingConfigSnapshotValue(source map[string]any, key string) any {
	value, _ := configSnapshotFieldValue(source, key)
	return value
}

func hasConfigSnapshotField(source map[string]any, key string) bool {
	_, ok := configSnapshotFieldValue(source, key)
	return ok
}

func configSnapshotFieldValue(source map[string]any, key string) (any, bool) {
	keys := append([]string{key}, configSnapshotFieldAliases[key]...)
	return lookupConfigSnapshotValue(source, keys...)
}

func lookupConfigSnapshotValue(source map[string]any, keys ...string) (any, bool) {
	for _, key := range keys {
		if value, exists := source[key]; exists {
			return value, true
		}
	}
	return nil, false
}

func configSnapshotMap(value any) map[string]any {
	if typed, ok := value.(map[string]any); ok {
		return typed
	}
	if value == nil {
		return map[string]any{}
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return map[string]any{}
	}
	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		return map[string]any{}
	}
	if result == nil {
		return map[string]any{}
	}
	return result
}

func configSnapshotSlice(value any) ([]any, bool) {
	if value == nil {
		return nil, false
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, false
	}
	var result []any
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, false
	}
	if result == nil {
		return []any{}, true
	}
	return result, true
}

func configSnapshotStringSlice(value any) []string {
	switch typed := value.(type) {
	case []string:
		return append([]string{}, typed...)
	case string:
		return splitConfigSnapshotStrings(typed)
	}
	items, ok := configSnapshotSlice(value)
	if !ok {
		return []string{}
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		result = append(result, splitConfigSnapshotStrings(configSnapshotStringValue(item, ""))...)
	}
	return result
}

func splitConfigSnapshotStrings(value string) []string {
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ';' || r == '，' || r == '；' || r == '、' || r == ' ' || r == '\t' || r == '\n' || r == '\r'
	})
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if text := strings.TrimSpace(part); text != "" {
			result = append(result, text)
		}
	}
	return result
}

func cloneConfigSnapshotValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneConfigSnapshotMap(typed)
	case []map[string]any:
		cloned := make([]map[string]any, len(typed))
		for index, item := range typed {
			cloned[index] = cloneConfigSnapshotMap(item)
		}
		return cloned
	case []string:
		return append([]string{}, typed...)
	case []any:
		cloned := make([]any, len(typed))
		for index, item := range typed {
			cloned[index] = cloneConfigSnapshotValue(item)
		}
		return cloned
	default:
		return typed
	}
}

func cloneConfigSnapshotMap(value map[string]any) map[string]any {
	cloned := make(map[string]any, len(value))
	for key, item := range value {
		cloned[key] = cloneConfigSnapshotValue(item)
	}
	return cloned
}

func firstConfigSnapshotNonNil(values ...any) any {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func configSnapshotIntValue(value any, fallback int) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case json.Number:
		parsed, err := typed.Int64()
		if err == nil {
			return int(parsed)
		}
	case string:
		var parsed int
		if _, err := fmt.Sscanf(strings.TrimSpace(typed), "%d", &parsed); err == nil {
			return parsed
		}
	}
	return fallback
}

func configSnapshotFloatValue(value any, fallback float64) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	case json.Number:
		parsed, err := typed.Float64()
		if err == nil {
			return parsed
		}
	case string:
		var parsed float64
		if _, err := fmt.Sscanf(strings.TrimSpace(typed), "%f", &parsed); err == nil {
			return parsed
		}
	}
	return fallback
}

func configSnapshotBoolValue(value any, fallback bool) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		switch strings.ToLower(strings.TrimSpace(typed)) {
		case "1", "true", "yes", "y", "on":
			return true
		case "0", "false", "no", "n", "off":
			return false
		}
	case float64:
		return typed != 0
	case int:
		return typed != 0
	}
	return fallback
}

func configSnapshotStringValue(value any, fallback string) string {
	switch typed := value.(type) {
	case string:
		return typed
	case fmt.Stringer:
		return typed.String()
	case nil:
		return fallback
	default:
		return fmt.Sprint(value)
	}
}
