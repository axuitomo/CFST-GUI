package probecore

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/task"
	"github.com/axuitomo/CFST-GUI/internal/utils"
)

func TestRunProbeStagesFastSkipsDownloadAndUsesSourceTotal(t *testing.T) {
	var beforeStages []string
	var afterStages []string
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
	if !reflect.DeepEqual(beforeStages, []string{StageTCP, StageTrace}) {
		t.Fatalf("before stages = %#v, want tcp+trace", beforeStages)
	}
	if !reflect.DeepEqual(afterStages, []string{StageTCP, StageTrace}) {
		t.Fatalf("after stages = %#v, want tcp+trace", afterStages)
	}
	if result.Summary.Total != 3 || result.Summary.Passed != 1 || result.Summary.Failed != 2 {
		t.Fatalf("summary = %#v, want total 3 passed 1 failed 2", result.Summary)
	}
	if len(result.Results) != 1 || result.Results[0].TestPort != 2053 {
		t.Fatalf("results = %#v, want one row with test port 2053", result.Results)
	}
}

func TestRunProbeStagesFullAppliesStage3LimitAndPrintLimit(t *testing.T) {
	oldDisable := task.Disable
	task.Disable = false
	t.Cleanup(func() {
		task.Disable = oldDisable
	})

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
	if !reflect.DeepEqual(result.CompletedStages, []string{StageTCP}) {
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
	if !reflect.DeepEqual(result.CompletedStages, []string{StageTCP}) {
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
