package appcore

import (
	"encoding/json"
	"testing"
)

func TestServiceInvokeRoutesSharedCommandsAndRejectsInvalidPayload(t *testing.T) {
	service := newTaskServiceForTest(t)
	if _, ok, _ := service.runtime.Start("invoke-task"); !ok {
		t.Fatal("start invoke task")
	}
	result := service.Invoke("probe.pause", `{"task_id":"invoke-task"}`)
	if !result.OK || result.Code != "PROBE_PAUSE_REQUESTED" {
		t.Fatalf("pause result = %#v", result)
	}
	invalid := service.Invoke("task.get", `{`)
	if invalid.OK || invalid.Code != "COMMAND_PAYLOAD_INVALID" {
		t.Fatalf("invalid result = %#v", invalid)
	}
	unknown := service.Invoke("platform.window.show", `{`)
	if unknown.OK || unknown.Code != "COMMAND_UNKNOWN" {
		t.Fatalf("unknown result = %#v", unknown)
	}
}

func TestServiceTryInvokeLeavesPlatformCommandsForAdapters(t *testing.T) {
	service := newTaskServiceForTest(t)
	if result, handled := service.TryInvoke("platform.window.show", `{`); handled || result.Code != "" {
		t.Fatalf("TryInvoke = %#v, %v", result, handled)
	}
}

func TestServiceTryInvokeOwnsConfigCommands(t *testing.T) {
	var savedHookSnapshot map[string]any
	service := NewService(ServiceOptions{
		ArchiveStorageState: func() any { return map[string]any{"backend": "test"} },
		ConfigSavedHook:     func(snapshot map[string]any) { savedHookSnapshot = snapshot },
		Storage: StorageLayout{
			Root:               t.TempDir(),
			ConfigFileName:     "config.json",
			DraftFileName:      "draft.json",
			SourceProfilesFile: "source-profiles.json",
		},
		ConfigPolicy: ConfigPolicy{
			DefaultSnapshot: func() map[string]any { return map[string]any{"probe": map[string]any{"tcp_port": 443}} },
		},
	})

	loaded, handled := service.TryInvoke("config.load", `{}`)
	if !handled || !loaded.OK || loaded.Code != "CONFIG_READY" {
		t.Fatalf("config.load = %#v, handled=%v", loaded, handled)
	}
	snapshot := mapValue(mapValue(loaded.Data)["config_snapshot"])
	if _, ok := snapshot["probe"]; !ok {
		t.Fatalf("config.load snapshot = %#v, want default probe", snapshot)
	}
	if mapValue(mapValue(loaded.Data)["storage"])["backend"] != "test" {
		t.Fatalf("config.load storage = %#v", mapValue(loaded.Data)["storage"])
	}

	if _, err := service.SaveDraft(snapshot); err != nil {
		t.Fatalf("SaveDraft: %v", err)
	}
	saved, handled := service.TryInvoke("config.save", encodeTestJSON(map[string]any{"config_snapshot": snapshot}))
	if !handled || !saved.OK || saved.Code != "CONFIG_SAVE_OK" {
		t.Fatalf("config.save = %#v, handled=%v", saved, handled)
	}
	if status := mapValue(mapValue(saved.Data)["draft_status"]); boolValue(status["exists"], true) {
		t.Fatalf("config.save draft status = %#v, want discarded", status)
	}
	data := mapValue(saved.Data)
	if mapValue(data["storage"])["backend"] != "test" || data["scheduler_status"] == nil {
		t.Fatalf("config.save data = %#v", data)
	}
	if intValue(mapValue(savedHookSnapshot["probe"])["tcp_port"], 0) != 443 {
		t.Fatalf("config saved hook snapshot = %#v", savedHookSnapshot)
	}
}

func encodeTestJSON(value any) string {
	raw, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return string(raw)
}

func TestServiceTryInvokeOwnsTaskResultsCommand(t *testing.T) {
	service := newTaskServiceForTest(t)
	speed := 12.5
	if err := service.taskStore.WriteResults("invoke-results", []ProbeResultRow{{
		Address:      "1.1.1.1",
		DownloadMbps: &speed,
		ExportStatus: "exported",
		StageStatus:  "completed",
	}}); err != nil {
		t.Fatalf("WriteResults: %v", err)
	}
	result, handled := service.TryInvoke("task.results", encodeTestJSON(map[string]any{
		"task_id": "invoke-results",
		"filter":  "exported",
	}))
	if !handled || !result.OK || result.Code != "RESULT_FILE_LISTED" {
		t.Fatalf("task.results = %#v, handled=%v", result, handled)
	}
	data := mapValue(result.Data)
	if intValue(data["count"], 0) != 1 || len(data["results"].([]ProbeResultRow)) != 1 || data["source_kind"] != "persisted" {
		t.Fatalf("task.results data = %#v", data)
	}
}
