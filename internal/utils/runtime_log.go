package utils

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	LogSchemaVersion = "cfst-log-v1"

	LogChannelRuntime = "runtime"
	LogChannelDebug   = "debug"
	LogChannelError   = "error"
	LogChannelMonitor = "monitor"

	LogLevelError = "error"
	LogLevelWarn  = "warn"
	LogLevelInfo  = "info"
	LogLevelDebug = "debug"

	RuntimeLogDurabilitySplit = "split"

	DefaultRuntimeLogLevel         = LogLevelError
	DefaultRuntimeLogRetentionDays = 7
	DefaultRuntimeLogDurability    = RuntimeLogDurabilitySplit
)

type runtimeLogConfig struct {
	configured     bool
	directory      string
	enabled        bool
	level          string
	retentionDays  int
	retentionClean bool
	durability     string
}

var (
	runtimeLogMu       sync.Mutex
	runtimeLogNow      = time.Now
	syncRuntimeLogFile = func(file *os.File) error {
		return file.Sync()
	}
	runtimeLog = runtimeLogConfig{
		enabled:       true,
		level:         DefaultRuntimeLogLevel,
		retentionDays: DefaultRuntimeLogRetentionDays,
		durability:    DefaultRuntimeLogDurability,
	}
)

func ConfigureRuntimeLog(enabled bool, directory string, level string, retentionDays int, options ...string) error {
	durability := DefaultRuntimeLogDurability
	if len(options) > 0 {
		durability = options[0]
	}
	runtimeLogMu.Lock()
	runtimeLog = runtimeLogConfig{
		configured:    true,
		directory:     strings.TrimSpace(directory),
		enabled:       enabled,
		level:         NormalizeLogLevel(level),
		retentionDays: NormalizeRuntimeLogRetentionDays(retentionDays),
		durability:    NormalizeRuntimeLogDurability(durability),
	}
	cfg := runtimeLog
	runtimeLogMu.Unlock()

	if cfg.directory == "" {
		return nil
	}
	if _, err := os.Stat(cfg.directory); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return cleanupRuntimeLogs(cfg.directory, cfg.retentionDays, runtimeLogNow())
}

func NormalizeRuntimeLogDurability(durability string) string {
	return RuntimeLogDurabilitySplit
}

func NormalizeLogLevel(level string) string {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case LogLevelError:
		return LogLevelError
	case LogLevelWarn:
		return LogLevelWarn
	case LogLevelInfo:
		return LogLevelInfo
	case LogLevelDebug:
		return LogLevelDebug
	default:
		return LogLevelError
	}
}

func IsKnownLogLevel(level string) bool {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case LogLevelError, LogLevelWarn, LogLevelInfo, LogLevelDebug:
		return true
	default:
		return false
	}
}

func NormalizeRuntimeLogRetentionDays(days int) int {
	if days < 1 {
		return DefaultRuntimeLogRetentionDays
	}
	if days > 365 {
		return 365
	}
	return days
}

func RuntimeLogFilePath(directory string, now time.Time) string {
	return filepath.Join(strings.TrimSpace(directory), "app-"+now.Format("2006-01-02")+".jsonl")
}

func MonitorLogFilePath(directory string, now time.Time) string {
	return filepath.Join(strings.TrimSpace(directory), "monitor-"+now.Format("2006-01-02")+".jsonl")
}

func AppendRuntimeLog(level, event string, fields map[string]any) error {
	return appendRuntimeLog("", level, event, fields)
}

func AppendRuntimeLogAlways(level, event string, fields map[string]any) error {
	cfg := currentRuntimeLogConfig("")
	if !cfg.enabled || cfg.directory == "" {
		return nil
	}
	entry := runtimeLogEntry(LogChannelRuntime, level, event, fields)
	return writeRuntimeLogEntry(cfg, entry)
}

func AppendMonitorLog(directory, event string, fields map[string]any) error {
	return AppendMonitorLogWithRetention(directory, DefaultRuntimeLogRetentionDays, event, fields)
}

func AppendMonitorLogWithRetention(directory string, retentionDays int, event string, fields map[string]any) error {
	directory = strings.TrimSpace(directory)
	if directory == "" {
		return nil
	}
	if err := cleanupRuntimeLogs(directory, retentionDays, runtimeLogNow()); err != nil {
		return err
	}
	entry := runtimeLogEntry(LogChannelMonitor, LogLevelInfo, event, fields)
	return writeJSONLLogEntry(MonitorLogFilePath(directory, runtimeLogNow()), entry, true)
}

func CleanupConfiguredRuntimeLogs(fallbackDirectory string) error {
	cfg := currentRuntimeLogConfig(fallbackDirectory)
	if cfg.directory == "" {
		return nil
	}
	if _, err := os.Stat(cfg.directory); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return cleanupRuntimeLogs(cfg.directory, cfg.retentionDays, runtimeLogNow())
}

func appendRuntimeLog(fallbackDirectory string, level, event string, fields map[string]any) error {
	cfg := currentRuntimeLogConfig(fallbackDirectory)
	if !cfg.enabled || cfg.directory == "" || !shouldWriteRuntimeLog(level, cfg.level) {
		return nil
	}
	entry := runtimeLogEntry(LogChannelRuntime, level, event, fields)
	return writeRuntimeLogEntry(cfg, entry)
}

func appendRuntimeLogEntry(entry map[string]any) error {
	return appendRuntimeLogEntryWithFallback("", entry)
}

func appendRuntimeLogEntryWithFallback(fallbackDirectory string, entry map[string]any) error {
	level := NormalizeLogLevel(debugLogValueToString(entry["level"]))
	cfg := currentRuntimeLogConfig(fallbackDirectory)
	if !cfg.enabled || cfg.directory == "" || !shouldWriteRuntimeLog(level, cfg.level) {
		return nil
	}
	return writeRuntimeLogEntry(cfg, entry)
}

func currentRuntimeLogConfig(fallbackDirectory string) runtimeLogConfig {
	runtimeLogMu.Lock()
	defer runtimeLogMu.Unlock()

	cfg := runtimeLog
	if strings.TrimSpace(cfg.directory) == "" {
		cfg.directory = strings.TrimSpace(fallbackDirectory)
	}
	if !cfg.configured {
		cfg.enabled = true
		cfg.level = DefaultRuntimeLogLevel
		cfg.retentionDays = DefaultRuntimeLogRetentionDays
		cfg.durability = DefaultRuntimeLogDurability
	}
	cfg.level = NormalizeLogLevel(cfg.level)
	cfg.retentionDays = NormalizeRuntimeLogRetentionDays(cfg.retentionDays)
	cfg.durability = NormalizeRuntimeLogDurability(cfg.durability)
	return cfg
}

func runtimeLogEntry(channel, level, event string, fields map[string]any) map[string]any {
	channel = normalizeLogChannel(channel)
	data := make(map[string]any)
	entry := map[string]any{
		"channel":        channel,
		"data":           data,
		"event":          strings.TrimSpace(event),
		"level":          NormalizeLogLevel(level),
		"schema_version": LogSchemaVersion,
		"ts":             runtimeLogNow().Format(time.RFC3339Nano),
	}
	if entry["event"] == "" {
		entry["event"] = defaultLogEvent(channel)
	}
	for key, value := range fields {
		normalizedKey := strings.TrimSpace(key)
		if normalizedKey == "" || normalizedKey == "ts" || normalizedKey == "event" || normalizedKey == "schema_version" || normalizedKey == "channel" {
			continue
		}
		if normalizedKey == "level" {
			entry["level"] = NormalizeLogLevel(debugLogValueToString(value))
			continue
		}
		if normalizedKey == "message" || normalizedKey == "task_id" || normalizedKey == "stage" {
			if text := strings.TrimSpace(debugLogValueToString(sanitizeDebugValue(normalizedKey, value))); text != "" {
				entry[normalizedKey] = text
			}
			continue
		}
		data[normalizedKey] = sanitizeDebugValue(normalizedKey, value)
	}
	return entry
}

func normalizeLogChannel(channel string) string {
	switch strings.ToLower(strings.TrimSpace(channel)) {
	case LogChannelDebug:
		return LogChannelDebug
	case LogChannelError:
		return LogChannelError
	case LogChannelMonitor:
		return LogChannelMonitor
	default:
		return LogChannelRuntime
	}
}

func defaultLogEvent(channel string) string {
	switch normalizeLogChannel(channel) {
	case LogChannelDebug:
		return "debug.event"
	case LogChannelError:
		return "error"
	case LogChannelMonitor:
		return "monitor.event"
	default:
		return "runtime.event"
	}
}

func writeRuntimeLogEntry(cfg runtimeLogConfig, entry map[string]any) error {
	now := runtimeLogNow()
	if err := os.MkdirAll(cfg.directory, 0o755); err != nil {
		return err
	}
	if err := cleanupRuntimeLogs(cfg.directory, cfg.retentionDays, now); err != nil {
		return err
	}
	raw, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	return writeJSONLBytes(RuntimeLogFilePath(cfg.directory, now), raw, shouldSyncRuntimeLog(entry, cfg.durability))
}

func writeJSONLLogEntry(path string, entry map[string]any, syncAfterWrite bool) error {
	raw, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	return writeJSONLBytes(path, raw, syncAfterWrite)
}

func writeJSONLBytes(path string, raw []byte, syncAfterWrite bool) error {
	if dir := filepath.Dir(path); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err = file.Write(append(raw, '\n')); err != nil {
		return err
	}
	if syncAfterWrite {
		return syncRuntimeLogFile(file)
	}
	return nil
}

func shouldSyncRuntimeLog(entry map[string]any, durability string) bool {
	if NormalizeRuntimeLogDurability(durability) != RuntimeLogDurabilitySplit {
		return false
	}
	switch NormalizeLogLevel(debugLogValueToString(entry["level"])) {
	case LogLevelError, LogLevelWarn:
		return true
	default:
		return false
	}
}

func shouldWriteRuntimeLog(entryLevel string, configuredLevel string) bool {
	entryRank, ok := runtimeLogLevelRank(NormalizeLogLevel(entryLevel))
	if !ok {
		entryRank, _ = runtimeLogLevelRank(LogLevelError)
	}
	configuredRank, ok := runtimeLogLevelRank(NormalizeLogLevel(configuredLevel))
	if !ok {
		configuredRank, _ = runtimeLogLevelRank(DefaultRuntimeLogLevel)
	}
	return entryRank <= configuredRank
}

func runtimeLogLevelRank(level string) (int, bool) {
	switch level {
	case LogLevelError:
		return 0, true
	case LogLevelWarn:
		return 1, true
	case LogLevelInfo:
		return 2, true
	case LogLevelDebug:
		return 3, true
	default:
		return 0, false
	}
}

func cleanupRuntimeLogs(directory string, retentionDays int, now time.Time) error {
	retentionDays = NormalizeRuntimeLogRetentionDays(retentionDays)
	cutoff := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, -(retentionDays - 1))
	for _, pattern := range []string{"app-*.jsonl", "monitor-*.jsonl"} {
		matches, err := filepath.Glob(filepath.Join(directory, pattern))
		if err != nil {
			return err
		}
		for _, path := range matches {
			logDate, ok := runtimeLogDateFromPath(path, now.Location())
			if !ok || !logDate.Before(cutoff) {
				continue
			}
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
	}
	return cleanupLegacyDebugLog(filepath.Join(directory, "cfip-log.txt"), cutoff)
}

func cleanupLegacyDebugLog(path string, cutoff time.Time) error {
	if path == currentDebugLogFilePath() {
		return nil
	}
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if info.IsDir() || !info.ModTime().Before(cutoff) {
		return nil
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func runtimeLogDateFromPath(path string, location *time.Location) (time.Time, bool) {
	name := filepath.Base(path)
	prefix := ""
	switch {
	case strings.HasPrefix(name, "app-"):
		prefix = "app-"
	case strings.HasPrefix(name, "monitor-"):
		prefix = "monitor-"
	default:
		return time.Time{}, false
	}
	if !strings.HasSuffix(name, ".jsonl") {
		return time.Time{}, false
	}
	rawDate := strings.TrimSuffix(strings.TrimPrefix(name, prefix), ".jsonl")
	parsed, err := time.ParseInLocation("2006-01-02", rawDate, location)
	if err != nil {
		return time.Time{}, false
	}
	return parsed, true
}
