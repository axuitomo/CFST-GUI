package mobileapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

func NewService() *Service {
	service := &Service{}
	service.pauseCond = sync.NewCond(&service.stateMu)
	return service
}

func (s *Service) SetEventSink(sink EventSink) {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	s.eventSink = sink
}

func (s *Service) Init(baseDir string) string {
	baseDir = strings.TrimSpace(baseDir)
	if baseDir == "" {
		baseDir = defaultBaseDir()
	}
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return encodeCommand(commandResultFor("MOBILE_INIT_FAILED", nil, err.Error(), false, nil, nil))
	}
	s.stateMu.Lock()
	s.baseDir = baseDir
	s.stateMu.Unlock()
	return encodeCommand(commandResultFor("MOBILE_INIT_OK", map[string]any{
		"base_dir":    baseDir,
		"config_path": s.configPath(),
	}, "Android mobile API 已初始化。", true, nil, nil))
}

func (s *Service) basePath() string {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	if strings.TrimSpace(s.baseDir) != "" {
		return s.baseDir
	}
	return defaultBaseDir()
}

func defaultBaseDir() string {
	dir, err := os.UserConfigDir()
	if err != nil || strings.TrimSpace(dir) == "" {
		dir = "."
	}
	return filepath.Join(dir, "CFST-GUI", "mobile")
}

func (s *Service) configPath() string {
	return filepath.Join(s.basePath(), "mobile-config.json")
}

func (s *Service) debugLogPath() string {
	return filepath.Join(s.basePath(), "cfip-log.txt")
}

func (s *Service) exportPath(outputFile string) string {
	outputFile = strings.TrimSpace(outputFile)
	if outputFile == "" {
		outputFile = "result.csv"
	}
	if filepath.IsAbs(outputFile) {
		return outputFile
	}
	return filepath.Join(s.basePath(), "exports", outputFile)
}

func encodeJSON(value any) string {
	raw, err := json.Marshal(value)
	if err != nil {
		return `{"code":"JSON_ENCODE_FAILED","data":null,"message":"` + escapeJSONString(err.Error()) + `","ok":false,"schema_version":"` + schemaVersion + `","task_id":null,"warnings":[]}`
	}
	return string(raw)
}

func encodeCommand(result commandResult) string {
	return encodeJSON(result)
}

func commandResultFor(code string, data any, message string, ok bool, taskID *string, warnings []string) commandResult {
	if warnings == nil {
		warnings = []string{}
	}
	return commandResult{
		Code:          code,
		Data:          data,
		Message:       message,
		OK:            ok,
		SchemaVersion: schemaVersion,
		TaskID:        taskID,
		Warnings:      dedupeStrings(warnings),
	}
}

func escapeJSONString(value string) string {
	raw, _ := json.Marshal(value)
	return strings.Trim(string(raw), `"`)
}

func decodeObject(raw string) (map[string]any, error) {
	decoder := json.NewDecoder(bytes.NewBufferString(raw))
	decoder.UseNumber()
	var payload map[string]any
	if err := decoder.Decode(&payload); err != nil {
		return nil, err
	}
	if payload == nil {
		payload = map[string]any{}
	}
	return payload, nil
}

func decodeInto(raw string, target any) error {
	decoder := json.NewDecoder(bytes.NewBufferString(raw))
	decoder.UseNumber()
	return decoder.Decode(target)
}

func mapValue(value any) map[string]any {
	if typed, ok := value.(map[string]any); ok {
		return typed
	}
	return map[string]any{}
}

func firstNonNil(values ...any) any {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func intValue(value any, fallback int) int {
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
		if _, err := fmt.Sscanf(typed, "%d", &parsed); err == nil {
			return parsed
		}
	}
	return fallback
}

func floatValue(value any, fallback float64) float64 {
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
		if _, err := fmt.Sscanf(typed, "%f", &parsed); err == nil {
			return parsed
		}
	}
	return fallback
}

func boolValue(value any, fallback bool) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case int:
		return typed != 0
	case int64:
		return typed != 0
	case float64:
		return typed != 0
	case json.Number:
		parsed, err := typed.Int64()
		if err == nil {
			return parsed != 0
		}
	case string:
		normalized := strings.ToLower(strings.TrimSpace(typed))
		switch normalized {
		case "1", "true", "yes", "on":
			return true
		case "0", "false", "no", "off":
			return false
		}
	}
	return fallback
}

func stringValue(value any, fallback string) string {
	if value == nil {
		return fallback
	}
	if typed, ok := value.(string); ok {
		return typed
	}
	return fmt.Sprint(value)
}

func sourceTokens(raw string) []string {
	tokens := make([]string, 0)
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	for _, line := range lines {
		if idx := strings.IndexByte(line, '#'); idx >= 0 {
			line = line[:idx]
		}
		parts := strings.FieldsFunc(line, func(r rune) bool {
			return r == ',' || r == ';' || r == '\t' || r == ' ' || r == '\n'
		})
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part != "" {
				tokens = append(tokens, part)
			}
		}
	}
	return tokens
}

func normalizeIPToken(token string) (string, bool) {
	token = strings.TrimSpace(token)
	if token == "" {
		return "", false
	}
	if strings.Contains(token, "/") {
		ip, ipNet, err := net.ParseCIDR(token)
		if err != nil {
			return "", false
		}
		ones, _ := ipNet.Mask.Size()
		return fmt.Sprintf("%s/%d", ip.String(), ones), true
	}
	ip := net.ParseIP(token)
	if ip == nil {
		return "", false
	}
	return ip.String(), true
}

func dedupeStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}

func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}

func clampInt(value, minValue, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func nowRFC3339() string {
	return time.Now().Format(time.RFC3339)
}
