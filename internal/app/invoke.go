package app

import (
	"encoding/json"

	"github.com/axuitomo/CFST-GUI/internal/appcore"
)

func (a *App) Invoke(command string, payloadJSON string) string {
	return encodeInvokeResult(a.core.Invoke(command, payloadJSON))
}

func encodeInvokeResult(result any) string {
	raw, err := json.Marshal(result)
	if err == nil {
		return string(raw)
	}
	fallback, _ := json.Marshal(appcore.NewCommandResult("COMMAND_ENCODE_FAILED", nil, err.Error(), false, nil, nil))
	return string(fallback)
}
