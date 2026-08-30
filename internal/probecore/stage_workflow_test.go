package probecore

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/utils"
)

func TestRunProbeStagesFastSkipsDownloadAndUsesSourceTotal(t *testing.T) {
	var beforeStages []string
	var afterStages []string
	traceTotal := 0
	result, err := RunProbeStages(StageWorkflowRequest{
		Config: StageWorkflowConfig{
			DisableDownload:     true,
			DownloadSpeedMetric: utils.DownloadSpeedMetricAverage,
			TCPPort:             2053,
		},
		Source: SourceSummary{
			CandidateCount: 3,
			ValidCount:     2,
		},
	}, StageWorkflowAdapter{
		ConfigureProgress: func(info StageInfo) {
			if info.Stage == StageTrace {
				traceTotal = info.Total
			}
		},
		EstimateTraceTotal: func(candidateCount int) int {
			if candidateCount != 2 {
				t.Fatalf("trace candidate count = %d, want 2", candidateCount)
			}
			return 1
		},
		BeforeStage: func(info StageInfo) error {
			beforeStages = append(beforeStages, info.Stage)
			return nil
		},
		AfterStage: func(info StageInfo) error {
			afterStages = append(afterStages, info.Stage)
			return nil
		},
		RunTCP: func() (utils.PingDelaySet, error) {
			return utils.PingDelaySet{
				probeCoreTestData("1.1.1.1", 10*time.Millisecond, 1),
				probeCoreTestData("1.1.1.2", 20*time.Millisecond, 1),
			}, nil
		},
		RunTrace: func(input utils.PingDelaySet) utils.PingDelaySet {
			return input[:1]
		},
	})
	if err != nil {
		t.Fatalf("RunProbeStages() error = %v", err)
	}
	if !slices.Equal(beforeStages, []string{StageTCP, StageTrace}) {
		t.Fatalf("before stages = %#v, want tcp+trace", beforeStages)
	}
	if !slices.Equal(afterStages, []string{StageTCP, StageTrace}) {
		t.Fatalf("after stages = %#v, want tcp+trace", afterStages)
	}
	if traceTotal != 1 {
		t.Fatalf("trace total = %d, want injected engine limit 1", traceTotal)
	}
	if result.Summary.Total != 3 || result.Summary.Passed != 1 || result.Summary.Failed != 2 {
		t.Fatalf("summary = %#v, want total 3 passed 1 failed 2", result.Summary)
	}
	if len(result.Results) != 1 || result.Results[0].TestPort != 2053 {
		t.Fatalf("results = %#v, want one row with test port 2053", result.Results)
	}
}

func TestRunProbeStagesSortsStage2ByRTTAndStage3ByHeadDelay(t *testing.T) {
	var traceInput utils.PingDelaySet
	var downloadInput utils.PingDelaySet
	tcpOutput := utils.PingDelaySet{
		probeCoreTestData("1.1.1.1", 30*time.Millisecond, 1),
		probeCoreTestData("1.1.1.2", 10*time.Millisecond, 1),
		probeCoreTestData("1.1.1.3", 20*time.Millisecond, 1),
	}
	_, err := RunProbeStages(StageWorkflowRequest{
		Config: StageWorkflowConfig{Stage2Limit: 2, Stage3Limit: 1, TCPPort: 443},
		Source: SourceSummary{CandidateCount: 3, ValidCount: 3},
	}, StageWorkflowAdapter{
		RunTCP: func() (utils.PingDelaySet, error) { return tcpOutput, nil },
		RunTrace: func(input utils.PingDelaySet) utils.PingDelaySet {
			traceInput = append(utils.PingDelaySet(nil), input...)
			traced := utils.PingDelaySet{input[1], input[0]}
			traced[0].HeadDelay = 5 * time.Millisecond
			traced[1].HeadDelay = 50 * time.Millisecond
			return traced
		},
		RunDownload: func(input utils.PingDelaySet) utils.DownloadSpeedSet {
			downloadInput = append(utils.PingDelaySet(nil), input...)
			return utils.DownloadSpeedSet(input)
		},
	})
	if err != nil {
		t.Fatalf("RunProbeStages() error = %v", err)
	}
	if got := probeWorkflowIPs(traceInput); !slices.Equal(got, []string{"1.1.1.2", "1.1.1.3"}) {
		t.Fatalf("trace input = %v, want RTT-sorted stage2 limit", got)
	}
	if got := probeWorkflowIPs(downloadInput); !slices.Equal(got, []string{"1.1.1.3"}) {
		t.Fatalf("download input = %v, want HeadDelay-sorted stage3 limit", got)
	}
	if got := probeWorkflowIPs(tcpOutput); !slices.Equal(got, []string{"1.1.1.1", "1.1.1.2", "1.1.1.3"}) {
		t.Fatalf("TCP output mutated = %v", got)
	}
}

func TestSortAndLimitHeadDelaySetPlacesMissingDelayLast(t *testing.T) {
	input := utils.PingDelaySet{
		probeCoreTestData("1.1.1.1", 1*time.Millisecond, 1),
		probeCoreTestData("1.1.1.2", 2*time.Millisecond, 1),
		probeCoreTestData("1.1.1.3", 3*time.Millisecond, 1),
	}
	input[1].HeadDelay = 20 * time.Millisecond
	input[2].HeadDelay = 10 * time.Millisecond

	result := SortAndLimitHeadDelaySet(input, 2)
	if got := probeWorkflowIPs(result); !slices.Equal(got, []string{"1.1.1.3", "1.1.1.2"}) {
		t.Fatalf("HeadDelay order = %v, want measured delays before missing delay", got)
	}
}

func probeWorkflowIPs(input utils.PingDelaySet) []string {
	result := make([]string, 0, len(input))
	for _, item := range input {
		result = append(result, item.IP.String())
	}
	return result
}

func TestRunProbeStagesFullAppliesStage3LimitAndPrintLimit(t *testing.T) {
	result, err := RunProbeStages(StageWorkflowRequest{
		Config: StageWorkflowConfig{
			DownloadSpeedMetric: utils.DownloadSpeedMetricAverage,
			PrintNum:            1,
			Stage3Limit:         2,
			TCPPort:             443,
		},
		Source: SourceSummary{
			CandidateCount: 4,
			ValidCount:     4,
		},
	}, StageWorkflowAdapter{
		RunTCP: func() (utils.PingDelaySet, error) {
			return utils.PingDelaySet{
				probeCoreTestData("1.1.1.1", 30*time.Millisecond, 1),
				probeCoreTestData("1.1.1.2", 20*time.Millisecond, 1),
				probeCoreTestData("1.1.1.3", 10*time.Millisecond, 1),
			}, nil
		},
		RunTrace: func(input utils.PingDelaySet) utils.PingDelaySet {
			return input
		},
		RunDownload: func(input utils.PingDelaySet) utils.DownloadSpeedSet {
			if len(input) != 2 {
				t.Fatalf("download input count = %d, want stage3 limit 2", len(input))
			}
			return utils.DownloadSpeedSet{
				probeCoreTestData("1.1.1.1", 30*time.Millisecond, 10),
				probeCoreTestData("1.1.1.2", 20*time.Millisecond, 50),
			}
		},
	})
	if err != nil {
		t.Fatalf("RunProbeStages() error = %v", err)
	}
	if result.Summary.Total != 2 || result.Summary.Passed != 1 || result.Summary.Failed != 1 {
		t.Fatalf("summary = %#v, want stage3 total 2 and print limited pass 1", result.Summary)
	}
	if len(result.RawResults) != 1 || len(result.Results) != 1 {
		t.Fatalf("raw/results count = %d/%d, want print limit 1/1", len(result.RawResults), len(result.Results))
	}
	if result.RawResults[0].IP.String() != result.Results[0].IP {
		t.Fatalf("raw/result alignment = %s/%s", result.RawResults[0].IP.String(), result.Results[0].IP)
	}
}

func TestRunProbeStagesStopsAfterEmptyTCPStage(t *testing.T) {
	var stages []string
	result, err := RunProbeStages(StageWorkflowRequest{
		Config: StageWorkflowConfig{TCPPort: 443},
		Source: SourceSummary{CandidateCount: 3, ValidCount: 3},
	}, StageWorkflowAdapter{
		BeforeStage: func(info StageInfo) error { stages = append(stages, info.Stage); return nil },
		RunTCP:      func() (utils.PingDelaySet, error) { return nil, nil },
		RunTrace:    func(utils.PingDelaySet) utils.PingDelaySet { t.Fatal("trace stage should not run"); return nil },
		RunDownload: func(utils.PingDelaySet) utils.DownloadSpeedSet { t.Fatal("download stage should not run"); return nil },
	})
	if err != nil {
		t.Fatalf("RunProbeStages() error = %v", err)
	}
	if !slices.Equal(stages, []string{StageTCP}) || !slices.Equal(result.CompletedStages, []string{StageTCP}) || result.CurrentStage != StageTCP {
		t.Fatalf("stages = %v, result = %#v", stages, result)
	}
	if result.Summary.Total != 3 || result.Summary.Passed != 0 || result.Summary.Failed != 3 {
		t.Fatalf("summary = %#v", result.Summary)
	}
	if !stageWorkflowWarningsContain(result.Warnings, "后续追踪与文件测速未执行") {
		t.Fatalf("warnings = %#v", result.Warnings)
	}
}

func TestRunProbeStagesStopsAfterEmptyTraceStage(t *testing.T) {
	var stages []string
	result, err := RunProbeStages(StageWorkflowRequest{
		Config: StageWorkflowConfig{TCPPort: 443},
		Source: SourceSummary{CandidateCount: 2, ValidCount: 2},
	}, StageWorkflowAdapter{
		BeforeStage: func(info StageInfo) error { stages = append(stages, info.Stage); return nil },
		RunTCP:      func() (utils.PingDelaySet, error) { return makeProbeSetForStageWorkflow(2), nil },
		RunTrace:    func(utils.PingDelaySet) utils.PingDelaySet { return nil },
		RunDownload: func(utils.PingDelaySet) utils.DownloadSpeedSet { t.Fatal("download stage should not run"); return nil },
	})
	if err != nil {
		t.Fatalf("RunProbeStages() error = %v", err)
	}
	if !slices.Equal(stages, []string{StageTCP, StageTrace}) || result.CurrentStage != StageTrace {
		t.Fatalf("stages = %v, result = %#v", stages, result)
	}
	if !stageWorkflowWarningsContain(result.Warnings, "后续文件测速未执行") {
		t.Fatalf("warnings = %#v", result.Warnings)
	}
}

func makeProbeSetForStageWorkflow(count int) utils.PingDelaySet {
	result := make(utils.PingDelaySet, 0, count)
	for range count {
		result = append(result, probeCoreTestData("1.1.1.1", 10*time.Millisecond, 1))
	}
	return result
}

func TestRunProbeStagesWarnsWhenTraceMissesTCPHits(t *testing.T) {
	result, err := RunProbeStages(StageWorkflowRequest{
		Config: StageWorkflowConfig{
			DisableDownload: true,
			TCPPort:         443,
		},
		Source: SourceSummary{CandidateCount: 1, ValidCount: 1},
	}, StageWorkflowAdapter{
		RunTCP: func() (utils.PingDelaySet, error) {
			return utils.PingDelaySet{probeCoreTestData("1.1.1.1", 10*time.Millisecond, 1)}, nil
		},
		RunTrace: func(input utils.PingDelaySet) utils.PingDelaySet {
			return nil
		},
	})
	if err != nil {
		t.Fatalf("RunProbeStages() error = %v", err)
	}
	if !stageWorkflowWarningsContain(result.Warnings, "追踪探测未命中") {
		t.Fatalf("warnings = %#v, want trace miss warning", result.Warnings)
	}
}

func TestRunProbeStagesPropagatesAdapterStageError(t *testing.T) {
	result, err := RunProbeStages(StageWorkflowRequest{
		Config:         StageWorkflowConfig{DisableDownload: true, TCPPort: 443},
		ConfigWarnings: []string{"config warning"},
		Source:         SourceSummary{CandidateCount: 1, ValidCount: 1},
	}, StageWorkflowAdapter{
		AfterStage: func(info StageInfo) error {
			if info.Stage == StageTCP {
				return errStageWorkflowStop
			}
			return nil
		},
		RunTCP: func() (utils.PingDelaySet, error) {
			return utils.PingDelaySet{probeCoreTestData("1.1.1.1", 10*time.Millisecond, 1)}, nil
		},
		RunTrace: func(input utils.PingDelaySet) utils.PingDelaySet {
			t.Fatal("RunTrace should not run after stage1 adapter error")
			return nil
		},
	})
	if err == nil || err.Error() != errStageWorkflowStop.Error() {
		t.Fatalf("err = %v, want adapter error", err)
	}
	if !slices.Equal(result.CompletedStages, []string{StageTCP}) {
		t.Fatalf("completed stages = %#v, want only TCP", result.CompletedStages)
	}
	if !stageWorkflowWarningsContain(result.Warnings, "config warning") {
		t.Fatalf("warnings = %#v, want config warning preserved", result.Warnings)
	}
}

func TestRunProbeStagesPropagatesTCPRunnerError(t *testing.T) {
	tcpErr := errors.New("ip pool failed")
	result, err := RunProbeStages(StageWorkflowRequest{
		Config:         StageWorkflowConfig{DisableDownload: true, TCPPort: 443},
		ConfigWarnings: []string{"config warning"},
		Source:         SourceSummary{CandidateCount: 1, ValidCount: 1},
	}, StageWorkflowAdapter{
		RunTCP: func() (utils.PingDelaySet, error) {
			return nil, tcpErr
		},
		RunTrace: func(input utils.PingDelaySet) utils.PingDelaySet {
			t.Fatal("RunTrace should not run after TCP runner error")
			return nil
		},
	})
	if !errors.Is(err, tcpErr) {
		t.Fatalf("err = %v, want TCP runner error", err)
	}
	if !slices.Equal(result.CompletedStages, []string{StageTCP}) {
		t.Fatalf("completed stages = %#v, want only TCP", result.CompletedStages)
	}
	if result.CurrentStage != StageTCP {
		t.Fatalf("current stage = %q, want TCP", result.CurrentStage)
	}
	if !stageWorkflowWarningsContain(result.Warnings, "config warning") {
		t.Fatalf("warnings = %#v, want config warning preserved", result.Warnings)
	}
}

var errStageWorkflowStop = errors.New("stage stopped")

func stageWorkflowWarningsContain(warnings []string, needle string) bool {
	for _, warning := range warnings {
		if strings.Contains(warning, needle) {
			return true
		}
	}
	return false
}
