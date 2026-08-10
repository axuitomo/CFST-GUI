package utils

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// DebugLogger owns the complete logging context for one probe service.
type DebugLogger struct {
	mu        sync.Mutex
	enabled   bool
	output    io.Writer
	file      *os.File
	taskID    string
	mode      string
	format    string
	verbosity string
	console   io.Writer
	clock     func() time.Time
}

func NewDebugLogger() *DebugLogger {
	return &DebugLogger{
		output:    io.Discard,
		mode:      DebugLogModeStructured,
		format:    DefaultDebugLogFormat,
		verbosity: DebugLogVerbosityDetailed,
		console:   os.Stdout,
		clock:     time.Now,
	}
}

func (logger *DebugLogger) SetEnabled(enabled bool) {
	if logger == nil {
		return
	}
	logger.mu.Lock()
	logger.enabled = enabled
	logger.mu.Unlock()
}

func (logger *DebugLogger) Configure(enabled bool, path string, options ...string) (string, error) {
	if logger == nil {
		return "", nil
	}
	logger.mu.Lock()
	defer logger.mu.Unlock()

	if err := logger.closeLocked(); err != nil {
		return "", err
	}
	logger.enabled = enabled
	mode, format, verbosity := "", "", ""
	if len(options) > 0 {
		mode = options[0]
	}
	if len(options) > 1 {
		format = options[1]
	}
	if len(options) > 2 {
		verbosity = options[2]
	}
	logger.mode = normalizeDebugLogMode(mode)
	logger.format = normalizeDebugLogFormat(format)
	logger.verbosity = normalizeDebugLogVerbosity(verbosity)
	if !enabled {
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
	logger.file = file
	logger.output = io.MultiWriter(file, logger.console)
	return path, nil
}

func (logger *DebugLogger) Close() error {
	if logger == nil {
		return nil
	}
	logger.mu.Lock()
	defer logger.mu.Unlock()
	return logger.closeLocked()
}

func (logger *DebugLogger) closeLocked() error {
	logger.output = io.Discard
	logger.taskID = ""
	logger.mode = DebugLogModeStructured
	logger.format = DefaultDebugLogFormat
	logger.verbosity = DebugLogVerbosityDetailed
	if logger.file == nil {
		return nil
	}
	err := logger.file.Close()
	logger.file = nil
	return err
}

func (logger *DebugLogger) SetContext(taskID string) {
	if logger == nil {
		return
	}
	logger.mu.Lock()
	logger.taskID = strings.TrimSpace(taskID)
	logger.mu.Unlock()
}

func (logger *DebugLogger) Debugf(format string, args ...any) {
	logger.Event("debug.message", map[string]any{
		"level":   "debug",
		"message": fmt.Sprintf(format, args...),
	})
}

func (logger *DebugLogger) Event(event string, fields map[string]any) {
	if logger == nil {
		return
	}
	logger.mu.Lock()
	defer logger.mu.Unlock()
	if !logger.enabled || logger.output == nil || !shouldWriteDebugEvent(event, logger.verbosity) {
		return
	}
	clock := logger.clock
	if clock == nil {
		clock = time.Now
	}
	entry := map[string]any{
		"event": strings.TrimSpace(event),
		"level": "info",
		"ts":    clock().Format(time.RFC3339Nano),
	}
	if entry["event"] == "" {
		entry["event"] = "debug.event"
	}
	if logger.taskID != "" {
		entry["task_id"] = logger.taskID
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
	line := renderDebugLogLine(entry, logger.mode, logger.format)
	_, _ = logger.output.Write(append(line, '\n'))
}
