package app

import (
	"encoding/json"
	"strings"

	"github.com/axuitomo/CFST-GUI/internal/kernel/appcore"
)

func (a *App) Invoke(command string, payloadJSON string) string {
	if strings.EqualFold(strings.TrimSpace(command), droppedSourcesCommand) {
		return encodeInvokeResult(a.consumeDroppedSourcesResult())
	}
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
