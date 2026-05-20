//go:build webui

package app

import (
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/axuitomo/CFST-GUI/utils"
)

func TestInvokeWebUIAppMethodRunDesktopProbeReturnsCompletedResult(t *testing.T) {
	oldTCP := desktopTCPProbeRunner
	oldTrace := desktopTraceProbeRunner
	oldDownload := desktopDownloadProbeRunner
	t.Cleanup(func() {
		desktopTCPProbeRunner = oldTCP
		desktopTraceProbeRunner = oldTrace
		desktopDownloadProbeRunner = oldDownload
	})

	desktopTCPProbeRunner = func() utils.PingDelaySet {
		return utils.PingDelaySet{
			{
				PingData: &utils.PingData{
					Delay:    10 * time.Millisecond,
					IP:       &net.IPAddr{IP: net.ParseIP("1.1.1.1")},
					Received: 4,
					Sended:   4,
				},
			},
		}
	}
	desktopTraceProbeRunner = func(input utils.PingDelaySet) utils.PingDelaySet {
		return input
	}
	desktopDownloadProbeRunner = func(input utils.PingDelaySet) utils.DownloadSpeedSet {
		return utils.DownloadSpeedSet(input)
	}

	app := NewApp()
	cfg := defaultProbeConfig()
	cfg.WriteOutput = false
	taskID := "webui-sync-task"
	payload := DesktopProbePayload{
		Config:  desktopConfigSnapshotForTest(cfg),
		Sources: []DesktopSource{{Content: "1.1.1.1", Enabled: true, ID: "source-1", Kind: "inline", Name: "inline", IPMode: "traverse"}},
		TaskID:  taskID,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal payload: %v", err)
	}

	resultValue, err := app.invokeWebUIAppMethod("RunDesktopProbe", map[string]any{"task_id": taskID}, raw)
	if err != nil {
		t.Fatalf("invokeWebUIAppMethod(RunDesktopProbe): %v", err)
	}
	result, ok := resultValue.(ProbeRunResult)
	if !ok {
		t.Fatalf("result type = %T, want ProbeRunResult", resultValue)
	}
	if len(result.Results) == 0 {
		t.Fatalf("RunDesktopProbe result = %#v, want probe rows", result)
	}
	if app.currentTaskID != "" {
		t.Fatalf("currentTaskID = %q, want cleared after sync completion", app.currentTaskID)
	}

	snapshotValue, err := app.invokeWebUIAppMethod("LoadTaskSnapshot", map[string]any{"task_id": taskID}, []byte(`{"task_id":"webui-sync-task"}`))
	if err != nil {
		t.Fatalf("invokeWebUIAppMethod(LoadTaskSnapshot): %v", err)
	}
	snapshotResult, ok := snapshotValue.(DesktopCommandResult)
	if !ok {
		t.Fatalf("snapshot result type = %T, want DesktopCommandResult", snapshotValue)
	}
	if !snapshotResult.OK || snapshotResult.Code != "TASK_SNAPSHOT" {
		t.Fatalf("LoadTaskSnapshot result = %#v, want TASK_SNAPSHOT", snapshotResult)
	}
}

func TestInvokeWebUIAppMethodStartDesktopProbeReturnsAccepted(t *testing.T) {
	oldTCP := desktopTCPProbeRunner
	oldTrace := desktopTraceProbeRunner
	oldDownload := desktopDownloadProbeRunner
	t.Cleanup(func() {
		desktopTCPProbeRunner = oldTCP
		desktopTraceProbeRunner = oldTrace
		desktopDownloadProbeRunner = oldDownload
	})

	tcpEntered := make(chan struct{})
	releaseProbe := make(chan struct{})
	desktopTCPProbeRunner = func() utils.PingDelaySet {
		close(tcpEntered)
		<-releaseProbe
		return utils.PingDelaySet{
			{
				PingData: &utils.PingData{
					Delay:    10 * time.Millisecond,
					IP:       &net.IPAddr{IP: net.ParseIP("1.1.1.1")},
					Received: 4,
					Sended:   4,
				},
			},
		}
	}
	desktopTraceProbeRunner = func(input utils.PingDelaySet) utils.PingDelaySet {
		return input
	}
	desktopDownloadProbeRunner = func(input utils.PingDelaySet) utils.DownloadSpeedSet {
		return utils.DownloadSpeedSet(input)
	}

	app := NewApp()
	cfg := defaultProbeConfig()
	cfg.WriteOutput = false
	taskID := "webui-async-task"
	payload := DesktopProbePayload{
		Config:  desktopConfigSnapshotForTest(cfg),
		Sources: []DesktopSource{{Content: "1.1.1.1", Enabled: true, ID: "source-1", Kind: "inline", Name: "inline", IPMode: "traverse"}},
		TaskID:  taskID,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal payload: %v", err)
	}

	resultValue, err := app.invokeWebUIAppMethod("StartDesktopProbe", map[string]any{"task_id": taskID}, raw)
	if err != nil {
		t.Fatalf("invokeWebUIAppMethod(StartDesktopProbe): %v", err)
	}
	result, ok := resultValue.(DesktopCommandResult)
	if !ok {
		t.Fatalf("result type = %T, want DesktopCommandResult", resultValue)
	}
	if !result.OK || result.Code != "PROBE_ACCEPTED" {
		t.Fatalf("StartDesktopProbe result = %#v, want PROBE_ACCEPTED", result)
	}

	select {
	case <-tcpEntered:
	case <-time.After(time.Second):
		t.Fatal("async webui probe did not enter TCP stage")
	}

	snapshotValue, err := app.invokeWebUIAppMethod("LoadTaskSnapshot", map[string]any{"task_id": taskID}, []byte(`{"task_id":"webui-async-task"}`))
	if err != nil {
		t.Fatalf("invokeWebUIAppMethod(LoadTaskSnapshot): %v", err)
	}
	snapshotResult, ok := snapshotValue.(DesktopCommandResult)
	if !ok {
		t.Fatalf("snapshot result type = %T, want DesktopCommandResult", snapshotValue)
	}
	if !snapshotResult.OK || snapshotResult.Code != "TASK_SNAPSHOT" {
		t.Fatalf("LoadTaskSnapshot result = %#v, want TASK_SNAPSHOT", snapshotResult)
	}

	close(releaseProbe)
	deadline := time.After(time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("webui async probe did not finish in time")
		default:
			if app.currentTaskID == "" {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
}
