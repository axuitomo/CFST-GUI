package probecore

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/utils"
)

// TestRunProbeStagesEmitsGranularObservations 覆盖新增的三类阶段观测事件：
// 指标分布、逐 IP 抽样、阶段间截断流失。
func TestRunProbeStagesEmitsGranularObservations(t *testing.T) {
	type observation struct {
		event  string
		fields map[string]any
	}
	events := make([]observation, 0, 8)
	collect := func(event string, fields map[string]any) {
		events = append(events, observation{event: event, fields: fields})
	}
	find := func(reason, stage string) map[string]any {
		for _, item := range events {
			if item.event != "stage.detail" {
				continue
			}
			if item.fields["reason"] == reason && (stage == "" || item.fields["stage"] == stage) {
				return item.fields
			}
		}
		return nil
	}

	withHeadDelay := func(ip string, delay, headDelay time.Duration, speedMB float64) utils.CloudflareIPData {
		item := probeCoreTestData(ip, delay, speedMB)
		item.HeadDelay = headDelay
		item.Colo = "HKG"
		return item
	}

	_, err := RunProbeStages(StageWorkflowRequest{
		Config: StageWorkflowConfig{
			DownloadSpeedMetric: utils.DownloadSpeedMetricAverage,
			PrintNum:            1,
			Stage2Limit:         1,
			Stage3Limit:         5,
			TCPPort:             443,
		},
		Source: SourceSummary{CandidateCount: 3, ValidCount: 3},
	}, StageWorkflowAdapter{
		EmitEvent: collect,
		RunTCP: func() (utils.PingDelaySet, error) {
			return utils.PingDelaySet{
				withHeadDelay("1.1.1.1", 10*time.Millisecond, 15*time.Millisecond, 1),
				withHeadDelay("1.1.1.2", 20*time.Millisecond, 25*time.Millisecond, 2),
				withHeadDelay("1.1.1.3", 30*time.Millisecond, 35*time.Millisecond, 3),
			}, nil
		},
		RunTrace: func(input utils.PingDelaySet) utils.PingDelaySet {
			return input
		},
		RunDownload: func(input utils.PingDelaySet) utils.DownloadSpeedSet {
			// 返回两条，配合 PrintNum=1 触发结果截断。
			return utils.DownloadSpeedSet{
				withHeadDelay("1.1.1.1", 10*time.Millisecond, 15*time.Millisecond, 30),
				withHeadDelay("1.1.1.2", 20*time.Millisecond, 25*time.Millisecond, 10),
			}
		},
	})
	if err != nil {
		t.Fatalf("RunProbeStages() error = %v", err)
	}

	tcpDistribution := find("stage_distribution", StageTCP)
	if tcpDistribution == nil {
		t.Fatalf("missing %s distribution event: %#v", StageTCP, events)
	}
	metrics, _ := tcpDistribution["metrics"].(map[string]any)
	if metrics["p50"] != 20.0 || metrics["min"] != 10.0 || metrics["max"] != 30.0 {
		t.Fatalf("%s metrics = %#v, want p50=20 min=10 max=30", StageTCP, metrics)
	}

	traceDistribution := find("stage_distribution", StageTrace)
	if traceDistribution == nil {
		t.Fatalf("missing %s distribution event: %#v", StageTrace, events)
	}
	if count := traceDistribution["counts"].(map[string]any)["count"]; count != 1 {
		t.Fatalf("%s distribution count = %v, want 1 (Stage2Limit)", StageTrace, count)
	}

	sample := find("stage_sample", StageTCP)
	if sample == nil {
		t.Fatalf("missing %s sample event: %#v", StageTCP, events)
	}
	samples := sample["samples"].([]map[string]any)
	if len(samples) != 3 || len(samples) > stageSampleLimit {
		t.Fatalf("%s samples = %d, want 3 and <= %d", StageTCP, len(samples), stageSampleLimit)
	}

	stage2Limit := find("stage_limit_dropped", StageTrace)
	if stage2Limit == nil {
		t.Fatalf("missing stage2 truncation event: %#v", events)
	}
	counts := stage2Limit["counts"].(map[string]any)
	if counts["dropped"] != 2 || counts["input"] != 3 || counts["kept"] != 1 {
		t.Fatalf("stage2 truncation counts = %#v, want input=3 kept=1 dropped=2", counts)
	}
	if stage2Limit["limit_kind"] != "stage2_limit" || stage2Limit["limit"] != 1 {
		t.Fatalf("stage2 truncation fields = %#v, want limit_kind=stage2_limit limit=1", stage2Limit)
	}

	resultLimit := find("stage_limit_dropped", "result")
	if resultLimit == nil {
		t.Fatalf("missing result truncation event: %#v", events)
	}
	if dropped := resultLimit["counts"].(map[string]any)["dropped"]; dropped != 1 {
		t.Fatalf("result truncation dropped = %v, want 1", dropped)
	}
}

// TestRunProbeStagesWithoutEmitEvent 确认未接入日志时新增观测路径不改变工作流行为。
func TestRunProbeStagesWithoutEmitEvent(t *testing.T) {
	result, err := RunProbeStages(StageWorkflowRequest{
		Config: StageWorkflowConfig{DisableDownload: true, TCPPort: 443},
		Source: SourceSummary{CandidateCount: 2, ValidCount: 2},
	}, StageWorkflowAdapter{
		RunTCP: func() (utils.PingDelaySet, error) {
			return utils.PingDelaySet{
				probeCoreTestData("1.1.1.1", 10*time.Millisecond, 1),
				probeCoreTestData("1.1.1.2", 20*time.Millisecond, 1),
			}, nil
		},
		RunTrace: func(input utils.PingDelaySet) utils.PingDelaySet { return input },
	})
	if err != nil {
		t.Fatalf("RunProbeStages() error = %v", err)
	}
	if len(result.RawResults) != 2 {
		t.Fatalf("raw results = %d, want 2", len(result.RawResults))
	}
}

// TestStageObservationsReachDebugLogFile 确认新增观测真的写进 cfip-log.txt 的 JSONL 行，
// 而不只是走了回调。
func TestStageObservationsReachDebugLogFile(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "cfip-log.txt")
	logger := utils.NewDebugLogger()
	if _, err := logger.Configure(true, logPath); err != nil {
		t.Fatalf("Configure() error = %v", err)
	}
	defer func() { _ = logger.Close() }()

	_, err := RunProbeStages(StageWorkflowRequest{
		Config: StageWorkflowConfig{DisableDownload: true, Stage2Limit: 1, TCPPort: 443},
		Source: SourceSummary{CandidateCount: 3, ValidCount: 3},
	}, StageWorkflowAdapter{
		EmitEvent: logger.Event,
		RunTCP: func() (utils.PingDelaySet, error) {
			return utils.PingDelaySet{
				probeCoreTestData("1.1.1.1", 10*time.Millisecond, 1),
				probeCoreTestData("1.1.1.2", 20*time.Millisecond, 1),
				probeCoreTestData("1.1.1.3", 30*time.Millisecond, 1),
			}, nil
		},
		RunTrace: func(input utils.PingDelaySet) utils.PingDelaySet { return input },
	})
	if err != nil {
		t.Fatalf("RunProbeStages() error = %v", err)
	}
	if err := logger.Flush(); err != nil {
		t.Fatalf("Flush() error = %v", err)
	}

	raw, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read debug log: %v", err)
	}
	reasons := make(map[string]bool)
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		if line == "" {
			continue
		}
		entry := map[string]any{}
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("debug log line is not JSON: %q", line)
		}
		if entry["event"] == "stage.detail" {
			reasons[entry["reason"].(string)] = true
		}
	}
	for _, want := range []string{"stage_distribution", "stage_sample", "stage_limit_dropped"} {
		if !reasons[want] {
			t.Fatalf("debug log missing %q reason, got %#v\n%s", want, reasons, raw)
		}
	}
}
