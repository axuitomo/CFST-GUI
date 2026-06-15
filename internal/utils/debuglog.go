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
	debugLogVerbosity               = DebugLogVerbosityDetailed
	debugLogConsoleOutput io.Writer = os.Stdout
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

func ConfigureDebugLog(enabled bool, path string, options ...string) (string, error) {
	debugLogMu.Lock()
	defer debugLogMu.Unlock()

	closeDebugLogLocked()
	log.SetOutput(os.Stderr)
	verbosity := ""
	if len(options) > 2 {
		verbosity = options[2]
	} else if len(options) > 0 {
		verbosity = options[len(options)-1]
	}
	debugLogVerbosity = normalizeDebugLogVerbosity(verbosity)

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
	debugLogOutput = file
	log.SetOutput(debugLogConsoleOutput)
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

	entryFields := map[string]any{
		"level": LogLevelInfo,
	}
	if taskID := currentDebugLogTaskID(); taskID != "" {
		entryFields["task_id"] = taskID
	}
	for key, value := range fields {
		entryFields[key] = value
	}
	entry := runtimeLogEntry(LogChannelDebug, LogLevelInfo, event, entryFields)

	debugLogMu.Lock()
	if debugLogOutput == nil || !shouldWriteDebugEvent(fmt.Sprint(entry["event"]), debugLogVerbosity) {
		debugLogMu.Unlock()
		return
	}

	line := renderDebugLogLine(entry)
	_, _ = debugLogOutput.Write(append(line, '\n'))
	if debugLogConsoleOutput != nil {
		_, _ = debugLogConsoleOutput.Write(append(line, '\n'))
	}
	debugLogMu.Unlock()
	_ = appendRuntimeLogEntry(entry)
}

func AppendErrorLog(path, event string, fields map[string]any) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	entry := runtimeLogEntry(LogChannelError, LogLevelError, event, fields)
	entry["level"] = LogLevelError
	raw, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	if err := writeJSONLBytes(path, raw, true); err != nil {
		return err
	}
	return appendRuntimeLogEntryWithFallback(filepath.Dir(path), entry)
}

func closeDebugLogLocked() error {
	debugLogOutput = io.Discard
	debugLogTaskID = ""
	debugLogVerbosity = DebugLogVerbosityDetailed
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

func currentDebugLogFilePath() string {
	debugLogMu.Lock()
	defer debugLogMu.Unlock()
	if debugLogFile == nil {
		return ""
	}
	return debugLogFile.Name()
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

func normalizeDebugLogMode(mode string) string {
	return DebugLogModeStructured
}

func normalizeDebugLogFormat(format string) string {
	return ""
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

func renderDebugLogLine(entry map[string]any) []byte {
	line, err := json.Marshal(entry)
	if err != nil {
		line, _ = json.Marshal(map[string]any{
			"channel":        LogChannelDebug,
			"data":           map[string]any{"error": err.Error()},
			"event":          "debug.encode_failed",
			"level":          LogLevelError,
			"message":        "failed to encode debug log entry",
			"schema_version": LogSchemaVersion,
			"ts":             time.Now().Format(time.RFC3339Nano),
		})
	}
	return line
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
