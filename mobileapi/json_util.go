package mobileapi

import (
	"encoding/json"
	"strings"

	"github.com/axuitomo/CFST-GUI/internal/kernel/appcore"
)

// jsonEncodeFailureFallback 是 encodeJSON 兜底响应的预生成字面量。
// 所有字段都经 json.Marshal 输出，不在源码里手工拼接 JSON，避免漏转义引号或反斜杠。
var jsonEncodeFailureFallback = buildJSONEncodeFailure()

func buildJSONEncodeFailure() string {
	raw, err := json.Marshal(appcore.NewCommandResult("JSON_ENCODE_FAILED", nil, "JSON 编码失败", false, nil, nil))
	if err != nil {
		return `{}`
	}
	return string(raw)
}

func encodeJSON(value any) string {
	raw, err := json.Marshal(value)
	if err == nil {
		return string(raw)
	}
	failure, failureErr := json.Marshal(appcore.NewCommandResult("JSON_ENCODE_FAILED", nil, err.Error(), false, nil, nil))
	if failureErr != nil {
		return jsonEncodeFailureFallback
	}
	return string(failure)
}

func encodeCommand(result appcore.CommandResult) string {
	return encodeJSON(result)
}

func decodeObject(raw string) (map[string]any, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		raw = "{}"
	}
	var payload map[string]any
	if _, err := appcore.UnmarshalJSONCompat([]byte(raw), &payload); err != nil {
		return nil, err
	}
	if payload == nil {
		payload = map[string]any{}
	}
	return payload, nil
}
