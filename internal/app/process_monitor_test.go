package app

import (
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/probecore"
	"github.com/axuitomo/CFST-GUI/internal/utils"
)

func preserveProcessMonitorForTest(t *testing.T) {
	t.Helper()
	oldExecutable := processMonitorExecutable
	oldPID := processMonitorPID
	oldNow := processMonitorNow
	oldStart := processMonitorStart
	oldAllowed := processMonitorAllowed
	oldLogMonitorNow := logMonitorNow
	oldLogMonitorProcessAlive := logMonitorProcessAlive
	t.Cleanup(func() {
		processMonitorExecutable = oldExecutable
		processMonitorPID = oldPID
		processMonitorNow = oldNow
		processMonitorStart = oldStart
		processMonitorAllowed = oldAllowed
		logMonitorNow = oldLogMonitorNow
		logMonitorProcessAlive = oldLogMonitorProcessAlive
	})
}

func TestConfigureDesktopProcessMonitorStartsHeartbeatAndSidecarWithExpectedArgs(t *testing.T) {
	preserveProcessMonitorForTest(t)
	isolateStorageForTest(t)
	app := NewApp()
	t.Cleanup(app.disableDesktopProcessMonitor)

	now := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	processMonitorNow = func() time.Time { return now }
	processMonitorPID = func() int { return 4242 }
	processMonitorExecutable = func() (string, error) { return "/tmp/cfst-gui", nil }
	processMonitorAllowed = func() bool { return true }
	var gotExecutable string
	var gotArgs []string
	processMonitorStart = func(executable string, args []string) (*exec.Cmd, error) {
		gotExecutable = executable
		gotArgs = append([]string(nil), args...)
		return exec.Command("true"), nil
	}

	warnings := app.configureDesktopProcessMonitor(probecore.DefaultRuntimeLogConfig())
	if len(warnings) != 0 {
		t.Fatalf("warnings = %#v, want none", warnings)
	}
	if gotExecutable != "/tmp/cfst-gui" {
		t.Fatalf("executable = %q, want /tmp/cfst-gui", gotExecutable)
	}
	wantArgs := []string{
		"--log-monitor",
		"--parent-pid", "4242",
		"--log-dir", logDirectoryPath(),
		"--heartbeat", filepath.Join(logDirectoryPath(), "main-heartbeat.json"),
		"--retention-days", "7",
		"--stale-after", "10s",
	}
	if !reflect.DeepEqual(gotArgs, wantArgs) {
		t.Fatalf("monitor args = %#v, want %#v", gotArgs, wantArgs)
	}
	heartbeat, err := utils.ReadMainHeartbeat(filepath.Join(logDirectoryPath(), "main-heartbeat.json"))
	if err != nil {
		t.Fatalf("read heartbeat: %v", err)
	}
	if heartbeat.PID != 4242 || heartbeat.State != utils.MainHeartbeatStateRunning || heartbeat.LogDir != logDirectoryPath() {
		t.Fatalf("heartbeat = %#v", heartbeat)
	}
}

func TestLogMonitorStateRecordsHungRecoveredAndExit(t *testing.T) {
	preserveProcessMonitorForTest(t)

	dir := t.TempDir()
	heartbeatPath := filepath.Join(dir, "main-heartbeat.json")
	base := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	logMonitorNow = func() time.Time { return base.Add(11 * time.Second) }
	logMonitorProcessAlive = func(int) bool { return true }
	if err := utils.WriteMainHeartbeat(heartbeatPath, utils.NewMainHeartbeat(101, base, base, utils.MainHeartbeatStateRunning, dir)); err != nil {
		t.Fatal(err)
	}
	state := &logMonitorState{}
	options := logMonitorOptions{
		HeartbeatPath: heartbeatPath,
		LogDir:        dir,
		ParentPID:     101,
		StaleAfter:    10 * time.Second,
	}
	stop, err := state.check(options)
	if stop || err != nil {
		t.Fatalf("hung check stop=%v err=%v", stop, err)
	}

	logMonitorNow = func() time.Time { return base.Add(12 * time.Second) }
	if err := utils.WriteMainHeartbeat(heartbeatPath, utils.NewMainHeartbeat(101, base, base.Add(12*time.Second), utils.MainHeartbeatStateRunning, dir)); err != nil {
		t.Fatal(err)
	}
	stop, err = state.check(options)
	if stop || err != nil {
		t.Fatalf("recovered check stop=%v err=%v", stop, err)
	}

	logMonitorProcessAlive = func(int) bool { return false }
	if err := utils.WriteMainHeartbeat(heartbeatPath, utils.NewMainHeartbeat(101, base, base.Add(13*time.Second), utils.MainHeartbeatStateShutdown, dir)); err != nil {
		t.Fatal(err)
	}
	stop, err = state.check(options)
	if !stop || err != nil {
		t.Fatalf("exit check stop=%v err=%v", stop, err)
	}

	matches, err := filepath.Glob(filepath.Join(dir, "monitor-*.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 {
		t.Fatalf("monitor log files = %#v, want one", matches)
	}
	entries := readDebugLogEntries(t, matches[0])
	events := make([]string, 0, len(entries))
	for _, entry := range entries {
		events = append(events, stringValue(entry["event"], ""))
	}
	if !reflect.DeepEqual(events, []string{"main.hung", "main.recovered", "main.exited"}) {
		t.Fatalf("monitor events = %#v", events)
	}
}
