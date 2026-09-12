package mobileapi

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/appcore"
	"github.com/axuitomo/CFST-GUI/internal/configvalue"
	"github.com/axuitomo/CFST-GUI/internal/utils"
)

// bridgeTracePayloadLimit caps how much of a raw payload/result is written to the
// trace file. Large payloads (task.results rows, colo dictionaries) are truncated
// so the trace stays readable and rotation pressure stays bounded.
const bridgeTracePayloadLimit = 2000

// bridgeTraceMaxFileSize / bridgeTraceMaxArchives bound disk usage of the trace
// file. The file rotates at MaxFileSize and at most MaxArchives rotated archives
// are kept alongside the active file.
const (
	bridgeTraceMaxFileSize = 4 * 1024 * 1024
	bridgeTraceMaxArchives = 3
)

// bridgeTraceMu serializes append + rotation across all Service instances in the
// process. Trace volume is low (one entry per bridge call / probe event), so a
// single process-wide mutex is simpler than per-Service state.
var bridgeTraceMu sync.Mutex

// trace records one JSONL bridge-trace entry to <runtimeDir>/logs/bridge-debug.log.
//
// The trace is deliberately gated on Init having set a real baseDir:
//   - On Android, CfstRuntime.ensureInitialized always calls Init, so traces land
//     under the app private runtime directory and are exportable via debug.export.
//   - Unit/contract tests that invoke the Service without Init produce no files,
//     keeping test runs free of side effects on the developer machine.
//
// trace is a best-effort, weak-failure path: any write error is swallowed and must
// never break the main command or event flow.
func (s *Service) trace(event string, fields map[string]any) {
	s.stateMu.Lock()
	baseDir := s.baseDir
	s.stateMu.Unlock()
	if strings.TrimSpace(baseDir) == "" {
		return
	}
	path := filepath.Join(baseDir, "logs", "bridge-debug.log")
	entry := map[string]any{
		"event": event,
		"level": "trace",
		"ts":    time.Now().Format(time.RFC3339Nano),
	}
	for key, value := range fields {
		if key == "event" || key == "level" || key == "ts" {
			continue
		}
		entry[key] = value
	}
	raw, err := json.Marshal(entry)
	if err != nil {
		return
	}
	line := utils.RedactSensitiveText(string(raw))

	bridgeTraceMu.Lock()
	defer bridgeTraceMu.Unlock()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	if info, statErr := os.Stat(path); statErr == nil && info.Size() >= bridgeTraceMaxFileSize {
		archive := fmt.Sprintf("%s.%s-%d.log", strings.TrimSuffix(path, filepath.Ext(path)), time.Now().Format("20060102-150405"), time.Now().UnixNano())
		_ = os.Rename(path, archive)
		cleanupBridgeTraceArchives(filepath.Dir(path), "bridge-debug")
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return
	}
	defer file.Close()
	if _, err := file.Write(append([]byte(line), '\n')); err != nil {
		return
	}
	_ = file.Sync()
}

// recordBridgeTrace is the "bridge.trace" command handler. It lets the Kotlin
// plugin and the frontend funnel their own layer observations into the single
// Go-owned trace sink, so all three producers write one file with one rotation
// and one redaction policy.
func (s *Service) recordBridgeTrace(payload map[string]any) string {
	event := strings.TrimSpace(configvalue.String(payload["event"], ""))
	if event == "" {
		event = "bridge.trace"
	}
	fields := make(map[string]any, len(payload))
	for key, value := range payload {
		if key == "event" {
			continue
		}
		fields[key] = value
	}
	s.trace(event, fields)
	return encodeCommand(appcore.NewCommandResult("BRIDGE_TRACE_OK", map[string]any{"event": event}, "桥接追踪已记录。", true, nil, nil))
}

func truncateJSONForTrace(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= bridgeTracePayloadLimit {
		return value
	}
	return value[:bridgeTracePayloadLimit] + fmt.Sprintf("…(截断 %d 字节)", len(value)-bridgeTracePayloadLimit)
}

func cleanupBridgeTraceArchives(dir, base string) {
	matches, err := filepath.Glob(filepath.Join(dir, base+".*.log"))
	if err != nil {
		return
	}
	if len(matches) <= bridgeTraceMaxArchives {
		return
	}
	for i := 0; i < len(matches)-bridgeTraceMaxArchives; i++ {
		_ = os.Remove(matches[i])
	}
}
