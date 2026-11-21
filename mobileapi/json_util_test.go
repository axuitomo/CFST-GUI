package mobileapi

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/axuitomo/CFST-GUI/internal/appcore"
)

type brokenJSONValue struct{}

func (brokenJSONValue) MarshalJSON() ([]byte, error) {
	return nil, errors.New(`bad "quote" and \ backslash`)
}

func TestEncodeJSONFailureEscapesMessage(t *testing.T) {
	raw := encodeJSON(brokenJSONValue{})
	if !json.Valid([]byte(raw)) {
		t.Fatalf("encodeJSON fallback is not valid JSON: %s", raw)
	}
	var result appcore.CommandResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("decode fallback: %v", err)
	}
	if result.Code != "JSON_ENCODE_FAILED" || result.OK {
		t.Fatalf("code/ok = %q/%v, want JSON_ENCODE_FAILED/false", result.Code, result.OK)
	}
	if !strings.Contains(result.Message, `bad "quote" and \ backslash`) {
		t.Fatalf("message = %q, want the marshal error text", result.Message)
	}
	if result.SchemaVersion != appcore.CommandSchemaVersion {
		t.Fatalf("schema_version = %q, want %q", result.SchemaVersion, appcore.CommandSchemaVersion)
	}
	if len(result.Warnings) != 0 {
		t.Fatalf("warnings = %#v, want empty", result.Warnings)
	}
}
