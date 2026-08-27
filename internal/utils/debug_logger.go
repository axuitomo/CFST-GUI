package utils

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type LogRotationConfig struct {
	MaxFileSize   int64
	MaxFileCount  int
	RotateOnStart bool
	FlushInterval time.Duration
	BufferSize    int
}

func defaultLogRotationConfig() LogRotationConfig {
	return LogRotationConfig{MaxFileSize: 10 * 1024 * 1024, MaxFileCount: 5, RotateOnStart: true, FlushInterval: 2 * time.Second, BufferSize: 64 * 1024}
}

type LogRotator struct{ config LogRotationConfig }

func NewLogRotator(config LogRotationConfig) *LogRotator {
	defaults := defaultLogRotationConfig()
	if config.MaxFileSize <= 0 {
		config.MaxFileSize = defaults.MaxFileSize
	}
	if config.MaxFileCount <= 0 {
		config.MaxFileCount = defaults.MaxFileCount
	}
	return &LogRotator{config: config}
}

func (r *LogRotator) CleanupExpired(logDir, prefix string) error {
	entries, err := os.ReadDir(logDir)
	if err != nil {
		return err
	}
	var paths []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasPrefix(entry.Name(), prefix+".") && strings.HasSuffix(entry.Name(), ".txt") && entry.Name() != prefix+".txt" {
			paths = append(paths, filepath.Join(logDir, entry.Name()))
		}
	}
	sort.Strings(paths)
	keep := r.config.MaxFileCount
	for len(paths) > keep {
		if err := os.Remove(paths[0]); err != nil && !os.IsNotExist(err) {
			return err
		}
		paths = paths[1:]
	}
	return nil
}

// DebugLogger owns the complete logging context for one probe service.
type DebugLogger struct {
	mu        sync.Mutex
	enabled   bool
	output    io.Writer
	file      *os.File
	buffer    *bufio.Writer
	fileSize  int64
	path      string
	rotation  LogRotationConfig
	rotator   *LogRotator
	flushStop chan struct{}
	flushDone chan struct{}
	taskID    string
	mode      string
	format    string
	verbosity string
	console   io.Writer
	clock     func() time.Time
}

func NewDebugLogger() *DebugLogger {
	config := defaultLogRotationConfig()
	return &DebugLogger{output: io.Discard, mode: DebugLogModeStructured, format: DefaultDebugLogFormat, verbosity: DebugLogVerbosityDetailed, console: os.Stdout, clock: time.Now, rotation: config, rotator: NewLogRotator(config)}
}

// SetRotationConfig updates rotation defaults for the next Configure call.
func (logger *DebugLogger) SetRotationConfig(config LogRotationConfig) {
	if logger == nil {
		return
	}
	logger.mu.Lock()
	defer logger.mu.Unlock()
	logger.rotation = config
	logger.rotator = NewLogRotator(config)
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
	if logger.rotation.RotateOnStart {
		if info, err := os.Stat(path); err == nil && info.Size() > 0 {
			_ = logger.rotateFileLocked(path)
		}
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return "", err
	}
	info, _ := file.Stat()
	logger.file, logger.path, logger.fileSize = file, path, 0
	if info != nil {
		logger.fileSize = info.Size()
	}
	logger.buffer = bufio.NewWriterSize(file, logger.rotation.BufferSize)
	logger.output = io.MultiWriter(logger.buffer, logger.console)
	logger.startFlushLoopLocked()
	return path, nil
}

func (logger *DebugLogger) startFlushLoopLocked() {
	logger.flushStop = make(chan struct{})
	logger.flushDone = make(chan struct{})
	stop, done, interval := logger.flushStop, logger.flushDone, logger.rotation.FlushInterval
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		defer close(done)
		for {
			select {
			case <-ticker.C:
				_ = logger.Flush()
			case <-stop:
				return
			}
		}
	}()
}

func (logger *DebugLogger) Flush() error {
	if logger == nil {
		return nil
	}
	logger.mu.Lock()
	defer logger.mu.Unlock()
	if logger.buffer == nil {
		return nil
	}
	return logger.buffer.Flush()
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
	if logger.buffer != nil {
		_ = logger.buffer.Flush()
	}
	if logger.flushStop != nil {
		close(logger.flushStop)
		logger.flushStop = nil
	}
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
	logger.buffer = nil
	return err
}

func (logger *DebugLogger) rotateFileLocked(path string) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	archive := fmt.Sprintf("%s.%s.txt", strings.TrimSuffix(path, filepath.Ext(path)), time.Now().Format("20060102-150405"))
	if err := os.Rename(path, archive); err != nil {
		return err
	}
	return logger.rotator.CleanupExpired(filepath.Dir(path), strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)))
}

func (logger *DebugLogger) Rotate() (string, error) {
	if logger == nil {
		return "", nil
	}
	logger.mu.Lock()
	defer logger.mu.Unlock()
	if logger.file == nil || logger.path == "" {
		return "", nil
	}
	if err := logger.buffer.Flush(); err != nil {
		return "", err
	}
	oldPath := logger.path
	if err := logger.file.Close(); err != nil {
		return "", err
	}
	logger.file = nil
	logger.buffer = nil
	archive := fmt.Sprintf("%s.%s.txt", strings.TrimSuffix(oldPath, filepath.Ext(oldPath)), time.Now().Format("20060102-150405"))
	if err := os.Rename(oldPath, archive); err != nil {
		return "", err
	}
	file, err := os.OpenFile(oldPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return "", err
	}
	logger.file = file
	logger.fileSize = 0
	logger.buffer = bufio.NewWriterSize(file, logger.rotation.BufferSize)
	logger.output = io.MultiWriter(logger.buffer, logger.console)
	_ = logger.rotator.CleanupExpired(filepath.Dir(oldPath), strings.TrimSuffix(filepath.Base(oldPath), filepath.Ext(oldPath)))
	return archive, nil
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
	logger.Event("debug.message", map[string]any{"level": "debug", "message": fmt.Sprintf(format, args...)})
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
	entry := map[string]any{"event": strings.TrimSpace(event), "level": "info", "ts": clock().Format(time.RFC3339Nano)}
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
	payload := append(line, '\n')
	_, _ = logger.output.Write(payload)
	logger.fileSize += int64(len(payload))
	if logger.fileSize >= logger.rotation.MaxFileSize {
		go logger.Rotate()
	}
}
