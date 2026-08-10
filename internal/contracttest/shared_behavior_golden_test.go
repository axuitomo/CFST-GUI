package contracttest

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/appcore"
	"github.com/axuitomo/CFST-GUI/internal/probecore"
)

type sharedBehaviorGolden struct {
	ConfigMigration struct {
		Legacy           string         `json:"legacy"`
		ExpectedSnapshot map[string]any `json:"expected_snapshot"`
	} `json:"config_migration"`
	Commands []struct {
		Command string `json:"command"`
		Payload string `json:"payload"`
		Code    string `json:"code"`
		OK      bool   `json:"ok"`
		TaskID  string `json:"task_id"`
	} `json:"commands"`
	PortGroups struct {
		IPs         []string              `json:"ips"`
		SourcePorts map[string]int        `json:"source_ports"`
		GlobalPort  int                   `json:"global_port"`
		Policy      string                `json:"policy"`
		Expected    []probecore.PortGroup `json:"expected"`
	} `json:"port_groups"`
	Upload struct {
		Snapshot              map[string]any       `json:"snapshot"`
		Rows                  []probecore.ProbeRow `json:"rows"`
		Metric                string               `json:"metric"`
		ExpectedFilteredIPs   []string             `json:"expected_filtered_ips"`
		ExpectedCloudflareIPs []string             `json:"expected_cloudflare_ips"`
		ExpectedGitHubIPs     []string             `json:"expected_github_ips"`
		ExpectedWarnings      []string             `json:"expected_warnings"`
	} `json:"upload"`
	Scheduler struct {
		Now             string                        `json:"now"`
		LastRun         string                        `json:"last_run"`
		Config          appcore.SchedulerTimingConfig `json:"config"`
		ExpectedNextRun string                        `json:"expected_next_run"`
	} `json:"scheduler"`
	Recovery struct {
		TaskID string `json:"task_id"`
		Events []struct {
			Event   string         `json:"event"`
			Payload map[string]any `json:"payload"`
		} `json:"events"`
		ExpectedStatus          string `json:"expected_status"`
		ExpectedStage           string `json:"expected_stage"`
		ExpectedSessionState    string `json:"expected_session_state"`
		ExpectedResumeCapable   bool   `json:"expected_resume_capable"`
		ExpectedRuntimeAttached bool   `json:"expected_runtime_attached"`
	} `json:"recovery"`
	EventSequence struct {
		TaskID string `json:"task_id"`
		Events []struct {
			Event   string         `json:"event"`
			Payload map[string]any `json:"payload"`
		} `json:"events"`
		ExpectedEvents    []string `json:"expected_events"`
		ExpectedSequences []int    `json:"expected_sequences"`
		ExpectedStatus    string   `json:"expected_status"`
		ExpectedTimestamp string   `json:"expected_timestamp"`
	} `json:"event_sequence"`
}

func TestSharedBehaviorGolden(t *testing.T) {
	fixture := loadSharedBehaviorGolden(t)
	t.Run("config migration", func(t *testing.T) { assertConfigMigrationGolden(t, fixture) })
	t.Run("commands", func(t *testing.T) { assertCommandGolden(t, fixture) })
	t.Run("port groups", func(t *testing.T) { assertPortGroupsGolden(t, fixture) })
	t.Run("upload selection", func(t *testing.T) { assertUploadGolden(t, fixture) })
	t.Run("scheduler", func(t *testing.T) { assertSchedulerGolden(t, fixture) })
	t.Run("recovery", func(t *testing.T) { assertRecoveryGolden(t, fixture) })
	t.Run("event sequence", func(t *testing.T) { assertEventSequenceGolden(t, fixture) })
}

func loadSharedBehaviorGolden(t *testing.T) sharedBehaviorGolden {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "shared_behavior.golden.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fixture sharedBehaviorGolden
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	return fixture
}

func assertConfigMigrationGolden(t *testing.T, fixture sharedBehaviorGolden) {
	t.Helper()
	root := t.TempDir()
	path := filepath.Join(root, "desktop-config.json")
	if err := os.WriteFile(path, []byte(fixture.ConfigMigration.Legacy), 0o600); err != nil {
		t.Fatal(err)
	}
	service := appcore.NewService(appcore.ServiceOptions{
		Storage:      appcore.StorageLayout{Root: root, ConfigFileName: "desktop-config.json"},
		ConfigPolicy: appcore.ConfigPolicy{SanitizeSnapshot: func(snapshot map[string]any) map[string]any { return snapshot }},
	})
	loaded, err := service.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(loaded.Snapshot, fixture.ConfigMigration.ExpectedSnapshot) || !loaded.CompatInfo.IgnoredTrailingContent {
		t.Fatalf("loaded config = %#v, compat = %#v", loaded.Snapshot, loaded.CompatInfo)
	}
	if _, err := service.SaveConfig(loaded.Snapshot); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path + ".v1.bak"); err != nil {
		t.Fatalf("legacy backup: %v", err)
	}
}

func assertCommandGolden(t *testing.T, fixture sharedBehaviorGolden) {
	t.Helper()
	for _, testCase := range fixture.Commands {
		t.Run(testCase.Command+"/"+testCase.Code, func(t *testing.T) {
			service := appcore.NewService(appcore.ServiceOptions{Storage: appcore.StorageLayout{Root: t.TempDir()}})
			result := service.Invoke(testCase.Command, testCase.Payload)
			if result.Code != testCase.Code || result.OK != testCase.OK {
				t.Fatalf("result = %#v", result)
			}
			if testCase.TaskID != "" && (result.TaskID == nil || *result.TaskID != testCase.TaskID) {
				t.Fatalf("task id = %#v, want %q", result.TaskID, testCase.TaskID)
			}
			if result.SchemaVersion != appcore.CommandSchemaVersion || result.Warnings == nil {
				t.Fatalf("shared envelope = %#v", result)
			}
		})
	}
}

func assertPortGroupsGolden(t *testing.T, fixture sharedBehaviorGolden) {
	t.Helper()
	got := probecore.PortGroups(fixture.PortGroups.IPs, fixture.PortGroups.SourcePorts, fixture.PortGroups.GlobalPort, fixture.PortGroups.Policy)
	if !reflect.DeepEqual(got, fixture.PortGroups.Expected) {
		t.Fatalf("port groups = %#v, want %#v", got, fixture.PortGroups.Expected)
	}
}

func assertUploadGolden(t *testing.T, fixture sharedBehaviorGolden) {
	t.Helper()
	result, err := appcore.BuildUploadSelection(fixture.Upload.Snapshot, fixture.Upload.Rows, fixture.Upload.Metric)
	if err != nil {
		t.Fatal(err)
	}
	if got := probeRowIPs(result.FilteredRows); !reflect.DeepEqual(got, fixture.Upload.ExpectedFilteredIPs) {
		t.Fatalf("filtered ips = %#v", got)
	}
	if got := probeRowIPs(result.CloudflareRows); !reflect.DeepEqual(got, fixture.Upload.ExpectedCloudflareIPs) {
		t.Fatalf("cloudflare ips = %#v", got)
	}
	if got := probeRowIPs(result.GitHubRows); !reflect.DeepEqual(got, fixture.Upload.ExpectedGitHubIPs) {
		t.Fatalf("github ips = %#v", got)
	}
	if !slices.Equal(result.Warnings, fixture.Upload.ExpectedWarnings) {
		t.Fatalf("warnings = %#v", result.Warnings)
	}
}

func assertSchedulerGolden(t *testing.T, fixture sharedBehaviorGolden) {
	t.Helper()
	now, err := time.Parse(time.RFC3339, fixture.Scheduler.Now)
	if err != nil {
		t.Fatal(err)
	}
	var lastRun time.Time
	if fixture.Scheduler.LastRun != "" {
		lastRun, err = time.Parse(time.RFC3339, fixture.Scheduler.LastRun)
		if err != nil {
			t.Fatal(err)
		}
	}
	got := appcore.NextSchedulerRun(now, lastRun, fixture.Scheduler.Config)
	if got.Format(time.RFC3339) != fixture.Scheduler.ExpectedNextRun {
		t.Fatalf("next run = %s, want %s", got.Format(time.RFC3339), fixture.Scheduler.ExpectedNextRun)
	}
}

func assertRecoveryGolden(t *testing.T, fixture sharedBehaviorGolden) {
	t.Helper()
	now := time.Date(2026, time.August, 9, 12, 0, 0, 0, time.UTC)
	var snapshot appcore.TaskSnapshot
	for index, event := range fixture.Recovery.Events {
		next := appcore.TaskSnapshotFromEvent(fixture.Recovery.TaskID, event.Event, event.Payload, now.Add(time.Duration(index)*time.Second))
		snapshot = appcore.MergeTaskSnapshots(snapshot, next)
	}
	if snapshot.Status != fixture.Recovery.ExpectedStatus || snapshot.CurrentStage != fixture.Recovery.ExpectedStage || snapshot.SessionState != fixture.Recovery.ExpectedSessionState || snapshot.ResumeCapable != fixture.Recovery.ExpectedResumeCapable || snapshot.RuntimeAttached != fixture.Recovery.ExpectedRuntimeAttached {
		t.Fatalf("recovered snapshot = %#v", snapshot)
	}
}

func assertEventSequenceGolden(t *testing.T, fixture sharedBehaviorGolden) {
	t.Helper()
	now, err := time.Parse(time.RFC3339, fixture.EventSequence.ExpectedTimestamp)
	if err != nil {
		t.Fatal(err)
	}
	events := make([]appcore.ProbeEvent, 0, len(fixture.EventSequence.Events))
	service := appcore.NewService(appcore.ServiceOptions{
		Clock:   func() time.Time { return now },
		Storage: appcore.StorageLayout{Root: t.TempDir()},
		EventSink: appcore.EventSinkFunc(func(_ context.Context, event appcore.ProbeEvent) {
			events = append(events, event)
		}),
	})
	for _, event := range fixture.EventSequence.Events {
		if err := service.PublishProbeEvent(context.Background(), appcore.ProbeEvent{
			Event: event.Event, TaskID: fixture.EventSequence.TaskID, Payload: event.Payload,
		}); err != nil {
			t.Fatal(err)
		}
	}
	if len(events) != len(fixture.EventSequence.ExpectedEvents) || len(events) != len(fixture.EventSequence.ExpectedSequences) {
		t.Fatalf("events = %#v", events)
	}
	for index, event := range events {
		if event.Event != fixture.EventSequence.ExpectedEvents[index] || event.Seq != fixture.EventSequence.ExpectedSequences[index] {
			t.Fatalf("event[%d] = %#v", index, event)
		}
		if event.SchemaVersion != appcore.EventSchemaVersion || event.TaskID != fixture.EventSequence.TaskID || event.TS != fixture.EventSequence.ExpectedTimestamp {
			t.Fatalf("event envelope[%d] = %#v", index, event)
		}
	}
	result := service.GetTask(appcore.TaskQueryRequest{TaskID: fixture.EventSequence.TaskID})
	snapshot, ok := result.Data.(appcore.TaskSnapshot)
	if !result.OK || !ok || snapshot.Status != fixture.EventSequence.ExpectedStatus {
		t.Fatalf("terminal snapshot = %#v", result)
	}
}

func probeRowIPs(rows []probecore.ProbeRow) []string {
	result := make([]string, 0, len(rows))
	for _, row := range rows {
		result = append(result, row.IP)
	}
	return result
}
