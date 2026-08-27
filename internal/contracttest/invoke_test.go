package contracttest

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/axuitomo/CFST-GUI/internal/app"
	"github.com/axuitomo/CFST-GUI/internal/appcore"
	"github.com/axuitomo/CFST-GUI/mobileapi"
)

func TestSharedCommandResultMatchesDesktopAndMobileAdapters(t *testing.T) {
	commands := []struct {
		command string
		code    string
	}{
		{command: "config.load", code: "COMMAND_PAYLOAD_INVALID"},
		{command: "config.save", code: "CONFIG_INVALID"},
		{command: "draft.load", code: "COMMAND_PAYLOAD_INVALID"},
		{command: "draft.save", code: "DRAFT_INVALID"},
		{command: "draft.discard", code: "COMMAND_PAYLOAD_INVALID"},
		{command: "source_profiles.load", code: "COMMAND_PAYLOAD_INVALID"},
		{command: "source_profiles.save", code: "SOURCE_PROFILE_INVALID"},
		{command: "source_profiles.update_current", code: "SOURCE_PROFILE_INVALID"},
		{command: "source_profiles.save_store", code: "SOURCE_PROFILE_INVALID"},
		{command: "source_profiles.switch", code: "SOURCE_PROFILE_INVALID"},
		{command: "source_profiles.delete", code: "SOURCE_PROFILE_INVALID"},
		{command: "probe.pause", code: "COMMAND_PAYLOAD_INVALID"},
		{command: "probe.cancel", code: "COMMAND_PAYLOAD_INVALID"},
		{command: "probe.resume", code: "COMMAND_PAYLOAD_INVALID"},
		{command: "task.get", code: "COMMAND_PAYLOAD_INVALID"},
		{command: "task.list", code: "COMMAND_PAYLOAD_INVALID"},
		{command: "task.results", code: "COMMAND_PAYLOAD_INVALID"},
	}
	for _, testCase := range commands {
		t.Run(testCase.command, func(t *testing.T) {
			desktopResult := decodeCommandResult(t, app.NewApp().Invoke(testCase.command, `{`))
			mobileResult := decodeCommandResult(t, mobileapi.NewService().Invoke(testCase.command, `{`))
			if desktopResult.Code != testCase.code {
				t.Fatalf("desktop code = %q", desktopResult.Code)
			}
			if !reflect.DeepEqual(desktopResult, mobileResult) {
				t.Fatalf("shared command mismatch:\ndesktop: %#v\nmobile:  %#v", desktopResult, mobileResult)
			}
		})
	}
}

func decodeCommandResult(t *testing.T, raw string) appcore.CommandResult {
	t.Helper()
	var result appcore.CommandResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("decode command result: %v", err)
	}
	return result
}
