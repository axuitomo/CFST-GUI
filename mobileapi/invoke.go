package mobileapi

import (
	"fmt"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/appcore"
)

func (s *Service) Invoke(command string, payloadJSON string) string {
	command = strings.ToLower(strings.TrimSpace(command))
	if command == "bridge.trace" {
		payload, err := decodeObject(payloadJSON)
		if err != nil {
			return encodeCommand(appcore.NewCommandResult("COMMAND_PAYLOAD_INVALID", nil, err.Error(), false, nil, nil))
		}
		return s.recordBridgeTrace(payload)
	}
	started := time.Now()
	var result appcore.CommandResult
	defer func() {
		fields := map[string]any{
			"command": command, "code": result.Code, "ok": result.OK,
			"duration_ms": time.Since(started).Milliseconds(),
		}
		if result.TaskID != nil {
			fields["task_id"] = *result.TaskID
		}
		s.trace("invoke.out", fields)
	}()
	if coreResult, handled := s.core.TryInvoke(command, payloadJSON); handled {
		result = coreResult
		return encodeCommand(result)
	}
	payload, err := decodeObject(payloadJSON)
	if err != nil {
		result = appcore.NewCommandResult("COMMAND_PAYLOAD_INVALID", nil, err.Error(), false, nil, nil)
		return encodeCommand(result)
	}
	switch command {
	case "probe.record_export":
		result = s.recordAndroidExportResult(payload)
	default:
		result = appcore.NewCommandResult("COMMAND_UNKNOWN", nil, fmt.Sprintf("unknown command: %s", command), false, nil, nil)
	}
	return encodeCommand(result)
}
