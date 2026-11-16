package mobileapi

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/axuitomo/CFST-GUI/internal/appcore"
)

func TestBridgeTraceOnlyRecordsMetadata(t *testing.T) {
	service := NewService()
	baseDir := t.TempDir()
	seedReadyColoDictionaryForTest(t, baseDir)
	if result := decodeCommandForTest(t, service.Init(baseDir)); !result.OK {
		t.Fatalf("Init = %#v", result)
	}
	path := filepath.Join(baseDir, "logs", "bridge-debug.log")
	readEntries := func() []map[string]any {
		t.Helper()
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(raw), "fake-") {
			t.Fatalf("sensitive content reached trace: %s", raw)
		}
		var entries []map[string]any
		for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
			var entry map[string]any
			if err := json.Unmarshal([]byte(line), &entry); err != nil {
				t.Fatal(err)
			}
			entries = append(entries, entry)
		}
		return entries
	}
	for _, request := range []struct{ command, payload string }{
		{"task.get", `{"api_token":"fake-api-token","password":"fake-password","token":"fake-token","url":"https://example.invalid/?token=fake-url-token"}`},
		{"unknown.command", `{`},
		{"probe.record_export", `{"task_id":"audit-export","ok":false,"message":"fake-response-secret https://example.invalid/?token=fake-response-token"}`},
	} {
		before := len(readEntries())
		result := decodeCommandForTest(t, service.Invoke(request.command, request.payload))
		entries := readEntries()
		if len(entries) != before+1 {
			t.Fatalf("%s wrote %d traces, want 1", request.command, len(entries)-before)
		}
		entry := entries[len(entries)-1]
		if entry["command"] != request.command || entry["code"] != result.Code || entry["ok"] != result.OK {
			t.Fatalf("wrong outcome: %#v", entry)
		}
		if request.command == "probe.record_export" && !strings.Contains(result.Message, "fake-response-secret") {
			t.Fatal("command response was changed")
		}
	}
	fields := map[string]any{
		"event": "security.check", "source": "android", "command": "config.save", "code": "CONFIG_SAVE_OK", "ok": true,
		"task_id": "audit-task", "seq": 7, "duration_ms": 12,
		"api_token": "fake-token", "password": "fake-password", "token": "fake-github-token",
		"payload": map[string]any{"secret": "fake-nested-secret"}, "result": `{"secret":"fake-result-secret"}`,
		"message": "https://example.invalid/?token=fake-query-token", "error": "fake-error", "note": "fake-note",
	}
	before := len(readEntries())
	if result := decodeCommandForTest(t, service.Invoke("bridge.trace", encodeJSON(fields))); !result.OK {
		t.Fatalf("bridge.trace = %#v", result)
	}
	entries := readEntries()
	if len(entries) != before+1 {
		t.Fatal("bridge.trace traced itself")
	}
	last := entries[len(entries)-1]
	for _, key := range []string{"payload", "result", "message", "error", "note", "api_token", "password", "token"} {
		if _, exists := last[key]; exists {
			t.Errorf("unapproved field %s survived", key)
		}
	}
	if last["source"] != "android" || last["task_id"] != "audit-task" || last["seq"] != float64(7) {
		t.Fatalf("safe metadata lost: %#v", last)
	}
	fields["unsupported"] = make(chan int)
	service.trace("security.check", fields)
	if len(readEntries()) != len(entries)+1 {
		t.Fatal("unapproved object prevented metadata trace")
	}

	service.trace("https://example.invalid/fake-event", map[string]any{
		"source": "https://example.invalid/fake-source", "command": "line\nbreak", "task_id": strings.Repeat("a", 129),
	})
	entries = readEntries()
	last = entries[len(entries)-1]
	if last["event"] != "bridge.trace" || last["source"] != nil || last["command"] != nil || last["task_id"] != nil {
		t.Fatalf("free text survived identifier validation: %#v", last)
	}

	sink := &recordingEventSink{}
	service.SetEventSink(sink)
	service.deliverProbeEvent(appcore.ProbeEvent{Event: "probe.progress", TaskID: "audit-task", Seq: 42, Payload: map[string]any{"secret": "fake-event-secret"}})
	entries = readEntries()
	last = entries[len(entries)-1]
	if last["event"] != "event.out" || last["source_event"] != "probe.progress" || last["seq"] != float64(42) {
		t.Fatalf("event metadata = %#v", last)
	}
	if len(sink.events) != 1 || !strings.Contains(sink.events[0], "fake-event-secret") {
		t.Fatal("event delivery was changed")
	}
}
