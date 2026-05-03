package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

var (
	debugLogMu            sync.Mutex
	debugLogOutput        io.Writer = io.Discard
	debugLogFile          *os.File
	debugLogTaskID        string
	debugLogConsoleOutput io.Writer = os.Stdout
)

const redactedValue = "<redacted>"

var bearerTokenPattern = regexp.MustCompile(`(?i)\b(bearer|token)\s+([A-Za-z0-9._~+/=-]{8,})`)

func ConfigureDebugLog(enabled bool, path string) (string, error) {
	debugLogMu.Lock()
	defer debugLogMu.Unlock()

	closeDebugLogLocked()
	log.SetOutput(os.Stderr)

	if !enabled {
		debugLogOutput = io.Discard
		return "", nil
	}

	path = strings.TrimSpace(path)
	if path == "" {
		path = "cfip-log.txt"
	}

	if dir := filepath.Dir(path); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", err
		}
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return "", err
	}

	debugLogFile = file
	debugLogOutput = io.MultiWriter(file, debugLogConsoleOutput)
	log.SetOutput(debugLogOutput)
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	return path, nil
}

func CloseDebugLog() error {
	debugLogMu.Lock()
	defer debugLogMu.Unlock()
	return closeDebugLogLocked()
}

func SetDebugLogContext(taskID string) {
	debugLogMu.Lock()
	defer debugLogMu.Unlock()
	debugLogTaskID = strings.TrimSpace(taskID)
}

func Debugf(format string, args ...any) {
	if !Debug {
		return
	}

	DebugEvent("debug.message", map[string]any{
		"level":   "debug",
		"message": fmt.Sprintf(format, args...),
	})
}

func DebugEvent(event string, fields map[string]any) {
	if !Debug {
		return
	}

	entry := map[string]any{
		"event": strings.TrimSpace(event),
		"level": "info",
		"ts":    time.Now().Format(time.RFC3339Nano),
	}
	if entry["event"] == "" {
		entry["event"] = "debug.event"
	}
	if taskID := currentDebugLogTaskID(); taskID != "" {
		entry["task_id"] = taskID
	}
	for key, value := range fields {
		normalizedKey := strings.TrimSpace(key)
		if normalizedKey == "" || normalizedKey == "ts" || normalizedKey == "event" {
			continue
		}
		entry[normalizedKey] = sanitizeDebugValue(normalizedKey, value)
	}
	if level, ok := entry["level"].(string); !ok || strings.TrimSpace(level) == "" {
		entry["level"] = "info"
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

	debugLogMu.Lock()
	defer debugLogMu.Unlock()
	if debugLogOutput == nil {
		return
	}

	_, _ = debugLogOutput.Write(append(line, '\n'))
}

func closeDebugLogLocked() error {
	debugLogOutput = io.Discard
	debugLogTaskID = ""
	if debugLogFile == nil {
		return nil
	}
	err := debugLogFile.Close()
	debugLogFile = nil
	return err
}

func currentDebugLogTaskID() string {
	debugLogMu.Lock()
	defer debugLogMu.Unlock()
	return debugLogTaskID
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
	return redactDebugURLQuery(bearerTokenPattern.ReplaceAllString(value, `$1 `+redactedValue))
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
