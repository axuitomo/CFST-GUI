package mobileapi

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/axuitomo/CFST-GUI/internal/appcore"
)

type recordingEventSink struct {
	events []string
}

func (sink *recordingEventSink) OnProbeEvent(eventJSON string) {
	sink.events = append(sink.events, eventJSON)
}

func decodeCommandForTest(t *testing.T, raw string) appcore.CommandResult {
	t.Helper()
	var result appcore.CommandResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("decode command result: %v", err)
	}
	return result
}

func commandDataForTest(t *testing.T, result appcore.CommandResult) map[string]any {
	t.Helper()
	data, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatalf("command data = %#v, want object", result.Data)
	}
	return data
}

func TestServiceInitAndCoreInvokeUsePrivateStorage(t *testing.T) {
	baseDir := t.TempDir()
	seedReadyColoDictionaryForTest(t, baseDir)
	service := NewService()

	initialized := decodeCommandForTest(t, service.Init(baseDir))
	if !initialized.OK || initialized.Code != "MOBILE_INIT_OK" {
		t.Fatalf("Init = %#v", initialized)
	}
	initData := commandDataForTest(t, initialized)
	if initData["base_dir"] != baseDir || initData["config_path"] != filepath.Join(baseDir, "mobile-config.json") {
		t.Fatalf("Init data = %#v", initData)
	}

	loaded := decodeCommandForTest(t, service.Invoke("config.load", "{}"))
	if !loaded.OK || loaded.Code != "CONFIG_READY" {
		t.Fatalf("config.load = %#v", loaded)
	}
	if commandDataForTest(t, loaded)["configPath"] != filepath.Join(baseDir, "mobile-config.json") {
		t.Fatalf("config.load data = %#v", loaded.Data)
	}
}

func TestServiceStorageSetRemainsPrivateDirectoryNoop(t *testing.T) {
	baseDir := t.TempDir()
	seedReadyColoDictionaryForTest(t, baseDir)
	service := NewService()
	decodeCommandForTest(t, service.Init(baseDir))

	result := decodeCommandForTest(t, service.Invoke("storage.set", `{"storage_uri":"content://tree/documents"}`))
	if !result.OK || result.Code != "STORAGE_SET_DEPRECATED" {
		t.Fatalf("storage.set = %#v", result)
	}
	storage, ok := commandDataForTest(t, result)["storage"].(map[string]any)
	if !ok || storage["backend"] != "private" || storage["storage_uri"] != "" {
		t.Fatalf("storage = %#v", storage)
	}
}

func TestServiceRecordExportPublishesAndroidPlatformEvent(t *testing.T) {
	baseDir := t.TempDir()
	seedReadyColoDictionaryForTest(t, baseDir)
	service := NewService()
	decodeCommandForTest(t, service.Init(baseDir))
	sink := &recordingEventSink{}
	service.SetEventSink(sink)

	result := decodeCommandForTest(t, service.Invoke("probe.record_export", `{"task_id":"android-export","source_path":"private/result.csv","target_uri":"content://exports/result.csv","written":3,"ok":true}`))
	if !result.OK || result.Code != "ANDROID_EXPORT_OK" {
		t.Fatalf("probe.record_export = %#v", result)
	}
	if len(sink.events) != 1 {
		t.Fatalf("events = %#v, want one", sink.events)
	}
	var event appcore.ProbeEvent
	if err := json.Unmarshal([]byte(sink.events[0]), &event); err != nil {
		t.Fatal(err)
	}
	if event.Event != "probe.export_completed" || event.TaskID != "android-export" {
		t.Fatalf("event = %#v", event)
	}
}

func TestRecoverServicePanicReturnsEnvelope(t *testing.T) {
	service := NewService()
	var resultJSON string
	func() {
		defer service.recoverServicePanic(&resultJSON, "test.command")
		panic("boom")
	}()
	result := decodeCommandForTest(t, resultJSON)
	if result.OK || result.Code != "COMMAND_PANIC" {
		t.Fatalf("code/ok = %q/%v, want COMMAND_PANIC/false", result.Code, result.OK)
	}
	if !strings.Contains(result.Message, "boom") {
		t.Fatalf("message = %q, want panic detail", result.Message)
	}
}

func TestInvokeKeepsUnknownCommandMessageEscaped(t *testing.T) {
	service := NewService()
	command := "bad\"cmd\\path\nline"
	raw := service.Invoke(command, "{}")
	if !json.Valid([]byte(raw)) {
		t.Fatalf("Invoke returned invalid JSON: %s", raw)
	}
	result := decodeCommandForTest(t, raw)
	if result.OK || result.Code != "COMMAND_UNKNOWN" {
		t.Fatalf("code/ok = %q/%v, want COMMAND_UNKNOWN/false", result.Code, result.OK)
	}
	want := "unknown command: " + strings.ToLower(strings.TrimSpace(command))
	if result.Message != want {
		t.Fatalf("message = %q, want %q", result.Message, want)
	}
}

func TestInvokeKeepsPayloadDecoderErrorEscaped(t *testing.T) {
	service := NewService()
	raw := service.Invoke("unknown.command", "{\"note\":\"line1\nline2\"}")
	if !json.Valid([]byte(raw)) {
		t.Fatalf("Invoke returned invalid JSON: %s", raw)
	}
	result := decodeCommandForTest(t, raw)
	if result.OK || result.Code != "COMMAND_PAYLOAD_INVALID" {
		t.Fatalf("code/ok = %q/%v, want COMMAND_PAYLOAD_INVALID/false", result.Code, result.OK)
	}
	if !strings.Contains(result.Message, "invalid character") || !strings.Contains(result.Message, "in string") {
		t.Fatalf("message = %q, want Go decoder detail", result.Message)
	}
}
