package appcore

import (
	"context"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/probecore"
	"github.com/axuitomo/CFST-GUI/internal/task"
	"github.com/axuitomo/CFST-GUI/internal/utils"
)

func probeSampleForTest() utils.PingDelaySet {
	return utils.PingDelaySet{{PingData: &utils.PingData{
		Delay: 10 * time.Millisecond, IP: &net.IPAddr{IP: net.ParseIP("1.1.1.1")}, Received: 4, Sended: 4,
	}}}
}

func probePayloadForCoreTest(taskID string, exportDir string) ProbePayload {
	config := map[string]any{"probe": map[string]any{"print_num": 10, "strategy": "fast", "tcp_port": 443}}
	if exportDir != "" {
		config["export"] = map[string]any{
			"file_name": "result.csv", "overwrite": "replace_on_start", "target_dir": exportDir,
		}
	}
	return ProbePayload{
		AndroidExportURI: "content://exports/result.csv",
		Config:           config,
		Sources: []Source{{
			Content: "1.1.1.1", Enabled: true, ID: "inline", IPMode: "traverse", Kind: "inline", Name: "inline",
		}},
		TaskID: taskID,
	}
}

func probeEventForTest(t *testing.T, events []ProbeEvent, name string) ProbeEvent {
	t.Helper()
	for _, event := range events {
		if event.Event == name {
			return event
		}
	}
	t.Fatalf("event %q not found in %#v", name, events)
	return ProbeEvent{}
}

func TestRunProbeCompletedEventKeepsPrivatePathUntilAndroidExportFinishes(t *testing.T) {
	events := make([]ProbeEvent, 0, 8)
	service := NewService(ServiceOptions{
		EventSink: EventSinkFunc(func(_ context.Context, event ProbeEvent) { events = append(events, event) }),
		ProbeHooks: ProbeExecutionHooks{
			RunTCP:   func(*task.Engine) (utils.PingDelaySet, error) { return probeSampleForTest(), nil },
			RunTrace: func(_ *task.Engine, input utils.PingDelaySet) utils.PingDelaySet { return input },
		},
		Storage: StorageLayout{Root: t.TempDir()},
	})
	exportDir := filepath.Join(t.TempDir(), "private")
	result, err := service.RunProbe(probePayloadForCoreTest("task-export-uri", exportDir))
	if err != nil {
		t.Fatal(err)
	}
	event := probeEventForTest(t, events, "probe.completed")
	if got := event.Payload["target_path"]; got != result.OutputFile || got != filepath.Join(exportDir, "result.csv") {
		t.Fatalf("target_path = %#v, result = %#v", got, result)
	}
	if got := event.Payload["android_export_uri"]; got != "content://exports/result.csv" {
		t.Fatalf("android_export_uri = %#v", got)
	}
	if pending, ok := event.Payload["android_export_pending"].(bool); !ok || !pending {
		t.Fatalf("android_export_pending = %#v, want true", event.Payload["android_export_pending"])
	}
}

func TestRunProbeCompletedEventIncludesTraceDiagnostics(t *testing.T) {
	events := make([]ProbeEvent, 0, 8)
	service := NewService(ServiceOptions{
		EventSink: EventSinkFunc(func(_ context.Context, event ProbeEvent) { events = append(events, event) }),
		ProbeHooks: ProbeExecutionHooks{
			RunTCP: func(*task.Engine) (utils.PingDelaySet, error) { return probeSampleForTest(), nil },
			RunTrace: func(engine *task.Engine, _ utils.PingDelaySet) utils.PingDelaySet {
				engine.EmitTraceDiagnostic(task.TraceDiagnostic{IP: "1.1.1.1", Reason: "rate_limited", StatusCode: 429})
				return nil
			},
		},
		Storage: StorageLayout{Root: t.TempDir()},
	})
	payload := probePayloadForCoreTest("task-trace-diagnostics", "")
	payload.AndroidExportURI = ""
	result, err := service.RunProbe(payload)
	if err != nil {
		t.Fatal(err)
	}
	if result.FailureStage != "stage2_trace" {
		t.Fatalf("failure stage = %q", result.FailureStage)
	}
	event := probeEventForTest(t, events, "probe.completed")
	diagnostics, ok := event.Payload["trace_diagnostics"].(map[string]any)
	if !ok {
		t.Fatalf("trace_diagnostics = %#v", event.Payload["trace_diagnostics"])
	}
	reasons, ok := diagnostics["reason_counts"].(map[string]int)
	if !ok || reasons["rate_limited"] != 1 {
		t.Fatalf("reason_counts = %#v", diagnostics["reason_counts"])
	}
}

func TestProbeSpeedEventIncludesMeasurementMetadata(t *testing.T) {
	events := make([]ProbeEvent, 0, 1)
	service := NewService(ServiceOptions{
		Storage:   StorageLayout{Root: t.TempDir()},
		EventSink: EventSinkFunc(func(_ context.Context, event ProbeEvent) { events = append(events, event) }),
	})
	service.emitProbeSpeed("speed-task", task.DownloadSpeedSample{
		AverageReady: true, BodyRead: true, BytesRead: 4096, CurrentReady: true, ElapsedMS: 250, IP: "1.1.1.1", Stage: "stage3_get",
	})
	payload := probeEventForTest(t, events, "probe.speed").Payload
	if payload["measured_bytes"] != int64(0) || payload["measured_elapsed_ms"] != int64(0) {
		t.Fatalf("measurement metadata = %#v", payload)
	}
	if payload["average_ready"] != true || payload["current_ready"] != true || payload["body_read"] != true || payload["transfer_complete"] != false {
		t.Fatalf("speed readiness metadata = %#v", payload)
	}
}

func TestProbeProgressThrottlesSameStage(t *testing.T) {
	events := make([]ProbeEvent, 0, 4)
	service := NewService(ServiceOptions{
		Storage:   StorageLayout{Root: t.TempDir()},
		EventSink: EventSinkFunc(func(_ context.Context, event ProbeEvent) { events = append(events, event) }),
	})
	service.runtime.ConfigureProgressThrottle(time.Hour)
	service.emitProbeProgress("progress-task", "stage1_tcp", 0, 0, 0, 10)
	service.emitProbeProgress("progress-task", "stage1_tcp", 2, 1, 1, 10)
	service.emitProbeProgress("progress-task", "stage1_tcp", 3, 2, 1, 10)
	service.emitProbeProgress("progress-task", "stage2_trace", 2, 1, 1, 10)
	service.emitProbeProgress("progress-task", "stage2_trace", 3, 2, 1, 10)
	service.emitProbeProgress("progress-task", "stage2_trace", 10, 8, 2, 10)
	if len(events) != 3 || events[0].Payload["stage"] != "stage1_tcp" || events[1].Payload["stage"] != "stage2_trace" || events[2].Payload["processed"] != 10 {
		t.Fatalf("progress events = %#v", events)
	}
}

func TestRunTCPWithMCISReusesMetricsWithoutTCPProbe(t *testing.T) {
	service := &Service{}
	config := task.DefaultConfig()
	config.IPText = ""
	engine := task.NewEngine(config, task.Hooks{})
	candidates := map[string]probecore.MCISCandidate{
		"2001:db8::1": {
			IP: "2001:db8::1", Prefix: "2001:db8::/64", Colo: "HKG",
			ConnectMS: 10, TLSMS: 20, TTFBMS: 30, TotalMS: 40,
			PrefixSamples: 8, PrefixOK: 8, PrefixFail: 0,
		},
	}

	result, err := service.runTCPWithMCIS(engine, candidates, []string{"2001:db8::1"}, true)
	if err != nil {
		t.Fatalf("runTCPWithMCIS() error = %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("result count = %d, want 1 reused candidate", len(result))
	}
	item := result[0]
	if item.Delay != 40*time.Millisecond || item.Colo != "HKG" || item.Sended != 8 || item.Received != 8 {
		t.Fatalf("reused measurement = %#v", item)
	}
	if item.MCISConnectMS != 10 || item.MCISTLSMS != 20 || item.MCISTTFBMS != 30 || item.MCISTotalMS != 40 || item.MCISPrefix != "2001:db8::/64" {
		t.Fatalf("MICS metrics = %#v", item)
	}
}
