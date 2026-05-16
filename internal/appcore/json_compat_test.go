package appcore

import (
	"encoding/json"
	"testing"
)

func TestUnmarshalJSONCompatIgnoresTrailingContent(t *testing.T) {
	var payload map[string]any
	info, err := UnmarshalJSONCompat([]byte("{\"config_snapshot\":{\"probe\":{\"routines\":12}}}4"), &payload)
	if err != nil {
		t.Fatalf("UnmarshalJSONCompat returned error: %v", err)
	}
	if !info.IgnoredTrailingContent {
		t.Fatalf("IgnoredTrailingContent = false, want true")
	}
	snapshot, ok := payload["config_snapshot"].(map[string]any)
	if !ok {
		t.Fatalf("config_snapshot missing: %#v", payload)
	}
	probe, ok := snapshot["probe"].(map[string]any)
	if !ok {
		t.Fatalf("probe missing: %#v", snapshot)
	}
	if got := probe["routines"]; got != float64(12) {
		t.Fatalf("probe.routines = %#v, want 12", got)
	}
}

func TestUnmarshalJSONCompatTrimsUTF8BOM(t *testing.T) {
	var payload map[string]any
	info, err := UnmarshalJSONCompat([]byte("\xef\xbb\xbf{\"ok\":true}"), &payload)
	if err != nil {
		t.Fatalf("UnmarshalJSONCompat returned error: %v", err)
	}
	if !info.TrimmedUTF8BOM {
		t.Fatalf("TrimmedUTF8BOM = false, want true")
	}
	if got, ok := payload["ok"].(bool); !ok || !got {
		raw, _ := json.Marshal(payload)
		t.Fatalf("payload = %s, want ok=true", raw)
	}
}
