package mobileapi

import (
	"encoding/json"
	"strings"

	"github.com/axuitomo/CFST-GUI/internal/appcore"
)

func encodeJSON(value any) string {
	raw, err := json.Marshal(value)
	if err != nil {
		return `{"code":"JSON_ENCODE_FAILED","data":null,"message":"` + escapeJSONString(err.Error()) + `","ok":false,"schema_version":"` + appcore.CommandSchemaVersion + `","task_id":null,"warnings":[]}`
	}
	return string(raw)
}

func encodeCommand(result appcore.CommandResult) string {
	return encodeJSON(result)
}

func escapeJSONString(value string) string {
	raw, _ := json.Marshal(value)
	return strings.Trim(string(raw), `"`)
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
