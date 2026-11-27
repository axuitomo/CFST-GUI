package host

import (
	"encoding/json"
	"strings"

	"github.com/axuitomo/CFST-GUI/internal/kernel/appcore"
)

func (h *Host) Invoke(command string, payloadJSON string) string {
	if strings.EqualFold(strings.TrimSpace(command), DroppedSourcesCommand) {
		return encodeInvokeResult(h.consumeDroppedSourcesResult())
	}
	return encodeInvokeResult(h.Core.Invoke(command, payloadJSON))
}

func encodeInvokeResult(result any) string {
	raw, err := json.Marshal(result)
	if err == nil {
		return string(raw)
	}
	fallback, _ := json.Marshal(appcore.NewCommandResult("COMMAND_ENCODE_FAILED", nil, err.Error(), false, nil, nil))
	return string(fallback)
}
