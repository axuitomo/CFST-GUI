package app

import (
	"encoding/json"
	"strings"
)

func (a *App) currentProbeRuntimeTaskID() string {
	a.ensureProbeControl()
	a.probeControlMu.Lock()
	defer a.probeControlMu.Unlock()
	if strings.TrimSpace(a.currentTaskID) != "" {
		return a.currentTaskID
	}
	if strings.TrimSpace(a.pausedTaskID) != "" || a.pauseRequested {
		return a.pausedTaskID
	}
	return ""
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func deepCloneMap(input map[string]any) map[string]any {
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
