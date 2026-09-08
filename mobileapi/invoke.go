package mobileapi

import (
	"fmt"
	"strings"

	"github.com/axuitomo/CFST-GUI/internal/appcore"
)

func (s *Service) Invoke(command string, payloadJSON string) (out string) {
	command = strings.ToLower(strings.TrimSpace(command))
	s.trace("invoke.in", map[string]any{
		"command": command,
		"payload": truncateJSONForTrace(payloadJSON),
	})
	defer func() {
		s.trace("invoke.out", map[string]any{
			"command": command,
			"result":  truncateJSONForTrace(out),
		})
	}()
	if result, handled := s.core.TryInvoke(command, payloadJSON); handled {
		out = encodeCommand(result)
		return
	}
	if strings.TrimSpace(payloadJSON) == "" {
		payloadJSON = "{}"
	}
	payload, err := decodeObject(payloadJSON)
	if err != nil {
		out = encodeCommand(appcore.NewCommandResult("COMMAND_PAYLOAD_INVALID", nil, err.Error(), false, nil, nil))
		return
	}
	switch command {
	case "bridge.trace":
		out = s.recordBridgeTrace(payload)
	case "probe.record_export":
		out = s.recordAndroidExportResult(payload)
	default:
		out = encodeCommand(appcore.NewCommandResult("COMMAND_UNKNOWN", nil, fmt.Sprintf("unknown command: %s", command), false, nil, nil))
	}
	return
}
