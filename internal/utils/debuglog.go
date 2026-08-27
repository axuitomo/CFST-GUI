package utils

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	DebugLogModeFreeform      = "freeform"
	DebugLogModeStructured    = "structured"
	DebugLogVerbosityDetailed = "detailed"
	DebugLogVerbositySimple   = "simple"
	DefaultDebugLogFormat     = "{ts} [{level}] {event} task={task_id} stage={stage} {message}"
	redactedValue             = "<redacted>"
)

var bearerTokenPattern = regexp.MustCompile(`(?i)\b(bearer|token)\s+([A-Za-z0-9._~+/=-]{8,})`)
var telegramBotTokenPattern = regexp.MustCompile(`[0-9]{5,16}:[A-Za-z0-9_-]{20,}`)
var debugLogPlaceholderPattern = regexp.MustCompile(`\{([A-Za-z0-9_.-]+)\}`)
var defaultErrorLogRotation = LogRotationConfig{MaxFileSize: 10 * 1024 * 1024, MaxFileCount: 10, RotateOnStart: false, FlushInterval: 0, BufferSize: 0}

func AppendErrorLog(path, event string, fields map[string]any) error {
	return AppendErrorLogWithRotation(path, event, fields, defaultErrorLogRotation)
}

// AppendErrorLogWithRotation appends one structured error and rotates the shared log format.
func AppendErrorLogWithRotation(path, event string, fields map[string]any, config LogRotationConfig) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	entry := map[string]any{
		"event": strings.TrimSpace(event),
		"level": "error",
		"ts":    time.Now().Format(time.RFC3339Nano),
	}
	if entry["event"] == "" {
		entry["event"] = "error"
	}
	for key, value := range fields {
		normalizedKey := strings.TrimSpace(key)
		if normalizedKey == "" || normalizedKey == "ts" || normalizedKey == "event" {
			continue
		}
		entry[normalizedKey] = sanitizeDebugValue(normalizedKey, value)
	}
	raw, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	if dir := filepath.Dir(path); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	if info, statErr := os.Stat(path); statErr == nil && config.MaxFileSize > 0 && info.Size() >= config.MaxFileSize {
		rotator := NewLogRotator(config)
		archive := fmt.Sprintf("%s.%s.txt", strings.TrimSuffix(path, filepath.Ext(path)), time.Now().Format("20060102-150405"))
		if err := os.Rename(path, archive); err != nil {
			return err
		}
		if err := rotator.CleanupExpired(filepath.Dir(path), strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))); err != nil {
			return err
		}
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.Write(append(raw, '\n'))
	return err
}

func sanitizeDebugValue(key string, value any) any {
	if value == nil {
		return nil
	}
	if isSensitiveDebugKey(key) {
		return redactedValue
	}

	switch typed := value.(type) {
	case error:
		return sanitizeDebugString(key, typed.Error())
	case string:
		return sanitizeDebugString(key, typed)
	case []string:
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			result = append(result, sanitizeDebugString(key, item))
		}
		return result
	case []any:
		result := make([]any, 0, len(typed))
		for _, item := range typed {
			result = append(result, sanitizeDebugValue(key, item))
		}
		return result
	case map[string]any:
		result := make(map[string]any, len(typed))
		for childKey, childValue := range typed {
			result[childKey] = sanitizeDebugValue(childKey, childValue)
		}
		return result
	case map[string]string:
		result := make(map[string]string, len(typed))
		for childKey, childValue := range typed {
			result[childKey] = fmt.Sprint(sanitizeDebugValue(childKey, childValue))
		}
		return result
	default:
		return typed
	}
}

func sanitizeDebugString(key, value string) string {
	if value == "" {
		return value
	}
	if isSensitiveDebugKey(key) {
		return redactedValue
	}
	return RedactSensitiveText(value)
}

// RedactSensitiveText removes credentials that may be embedded in errors or URLs.
func RedactSensitiveText(value string) string {
	value = bearerTokenPattern.ReplaceAllString(value, `$1 `+redactedValue)
	value = telegramBotTokenPattern.ReplaceAllString(value, redactedValue)
	return redactDebugURLQuery(value)
}

func isSensitiveDebugKey(key string) bool {
	normalized := strings.ToLower(strings.TrimSpace(key))
	if normalized == "" {
		return false
	}
	normalized = strings.ReplaceAll(normalized, "-", "_")
	sensitiveParts := []string{
		"api_token",
		"authorization",
		"cookie",
		"password",
		"secret",
		"set_cookie",
	}
	for _, part := range sensitiveParts {
		if strings.Contains(normalized, part) {
			return true
		}
	}
	if normalized == "token" || strings.HasSuffix(normalized, "_token") || strings.HasPrefix(normalized, "token_") {
		return true
	}
	return false
}

func isSensitiveDebugQueryKey(key string) bool {
	normalized := strings.ToLower(strings.TrimSpace(key))
	if normalized == "" {
		return false
	}
	for _, part := range []string{"token", "secret", "password", "authorization", "auth", "signature", "api_key", "apikey"} {
		if strings.Contains(normalized, part) {
			return true
		}
	}
	return false
}

func redactDebugURLQuery(value string) string {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.RawQuery == "" {
		return value
	}

	query := parsed.Query()
	changed := false
	for key := range query {
		if isSensitiveDebugQueryKey(key) {
			query.Set(key, redactedValue)
			changed = true
		}
	}
	if !changed {
		return value
	}
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func normalizeDebugLogMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case DebugLogModeFreeform:
		return DebugLogModeFreeform
	default:
		return DebugLogModeStructured
	}
}

func normalizeDebugLogFormat(format string) string {
	format = strings.TrimSpace(format)
	if format == "" {
		return DefaultDebugLogFormat
	}
	return format
}

func normalizeDebugLogVerbosity(verbosity string) string {
	switch strings.ToLower(strings.TrimSpace(verbosity)) {
	case DebugLogVerbositySimple:
		return DebugLogVerbositySimple
	default:
		return DebugLogVerbosityDetailed
	}
}

func shouldWriteDebugEvent(event string, verbosity string) bool {
	if normalizeDebugLogVerbosity(verbosity) != DebugLogVerbositySimple {
		return true
	}
	switch strings.TrimSpace(event) {
	case "probe.start", "stage.complete", "probe.export", "probe.complete", "probe.failed":
		return true
	default:
		return false
	}
}

func renderDebugLogLine(entry map[string]any, mode string, format string) []byte {
	if normalizeDebugLogMode(mode) == DebugLogModeFreeform {
		return []byte(renderFreeformDebugLog(entry, format))
	}

	line, err := json.Marshal(entry)
	if err != nil {
		line, _ = json.Marshal(map[string]any{
			"error":   err.Error(),
			"event":   "debug.encode_failed",
			"level":   "error",
			"message": "failed to encode debug log entry",
			"ts":      time.Now().Format(time.RFC3339Nano),
		})
	}
	return line
}

func renderFreeformDebugLog(entry map[string]any, format string) string {
	format = normalizeDebugLogFormat(format)
	return debugLogPlaceholderPattern.ReplaceAllStringFunc(format, func(token string) string {
		matches := debugLogPlaceholderPattern.FindStringSubmatch(token)
		if len(matches) != 2 {
			return ""
		}
		return debugLogValueToString(entry[matches[1]])
	})
}

func debugLogValueToString(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case fmt.Stringer:
		return typed.String()
	case error:
		return typed.Error()
	case bool:
		return fmt.Sprintf("%t", typed)
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return fmt.Sprint(typed)
	default:
		raw, err := json.Marshal(typed)
		if err != nil {
			return fmt.Sprint(typed)
		}
		return string(raw)
	}
}
