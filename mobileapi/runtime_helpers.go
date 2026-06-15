package mobileapi

import (
	"encoding/json"
	"strings"
)

func (s *Service) taskEventMetadataFor(taskID string) map[string]any {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	metadata := s.taskEventMetadata[taskID]
	if len(metadata) == 0 {
		return nil
	}
	cloned := make(map[string]any, len(metadata))
	for key, value := range metadata {
		cloned[key] = value
	}
	return cloned
}

func decodeCommandResult(response string) commandResult {
	var command commandResult
	if err := json.Unmarshal([]byte(response), &command); err != nil {
		return commandResult{Code: "COMMAND_DECODE_FAILED", Message: err.Error(), OK: false}
	}
	return command
}

func mobileProbeRowsIPList(rows []probeRow) string {
	values := make([]string, 0, len(rows))
	for _, row := range rows {
		if ip := strings.TrimSpace(row.IP); ip != "" {
			values = append(values, ip)
		}
	}
	return strings.Join(values, "\n")
}

func mobileDeepCloneMap(input map[string]any) map[string]any {
	if input == nil {
		return map[string]any{}
	}
	raw, err := json.Marshal(input)
	if err != nil {
		return map[string]any{}
	}
	var output map[string]any
	if err := json.Unmarshal(raw, &output); err != nil {
		return map[string]any{}
	}
	return output
}
