package utils

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	debugLogMu     sync.Mutex
	debugLogOutput io.Writer = io.Discard
	debugLogFile   *os.File
)

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
	debugLogOutput = io.MultiWriter(os.Stdout, file)
	log.SetOutput(debugLogOutput)
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	return path, nil
}

func CloseDebugLog() error {
	debugLogMu.Lock()
	defer debugLogMu.Unlock()
	return closeDebugLogLocked()
}

func Debugf(format string, args ...interface{}) {
	if !Debug {
		return
	}

	message := fmt.Sprintf(format, args...)
	if !strings.HasSuffix(message, "\n") {
		message += "\n"
	}

	debugLogMu.Lock()
	defer debugLogMu.Unlock()
	if debugLogOutput == nil {
		return
	}

	_, _ = fmt.Fprintf(debugLogOutput, "%s %s", time.Now().Format(time.RFC3339), message)
}

func closeDebugLogLocked() error {
	debugLogOutput = io.Discard
	if debugLogFile == nil {
		return nil
	}
	err := debugLogFile.Close()
	debugLogFile = nil
	return err
}
