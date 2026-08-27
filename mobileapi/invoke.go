package mobileapi

import (
	"fmt"
	"strings"

	"github.com/axuitomo/CFST-GUI/internal/appcore"
)

func (s *Service) Invoke(command string, payloadJSON string) string {
	command = strings.ToLower(strings.TrimSpace(command))
	if result, handled := s.core.TryInvoke(command, payloadJSON); handled {
		return encodeCommand(result)
	}
	if strings.TrimSpace(payloadJSON) == "" {
		payloadJSON = "{}"
	}
	payload, err := decodeObject(payloadJSON)
	if err != nil {
		return encodeCommand(appcore.NewCommandResult("COMMAND_PAYLOAD_INVALID", nil, err.Error(), false, nil, nil))
	}
	switch command {
	case "probe.record_export":
		return s.recordAndroidExportResult(payload)
	default:
		return encodeCommand(appcore.NewCommandResult("COMMAND_UNKNOWN", nil, fmt.Sprintf("unknown command: %s", command), false, nil, nil))
	}
}
