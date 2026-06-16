package task

import "sync"

type DownloadSpeedSample struct {
	Stage             string
	IP                string
	CurrentSpeedMBs   float64
	CurrentReady      bool
	AverageSpeedMBs   float64
	AverageReady      bool
	BodyRead          bool
	BytesRead         int64
	ElapsedMS         int64
	Colo              string
	SampleBytes       int64
	SampleElapsedMS   int64
	MeasuredBytes     int64
	MeasuredElapsedMS int64
	TransferComplete  bool
	Attempt           int
}

type StageRejectEvent struct {
	Error   string
	IP      string
	Message string
	Reason  string
	Stage   string
}

var (
	LatencyProgressHook      func(processed, passed, failed, total int)
	HeadProgressHook         func(processed, passed, failed, total int)
	TraceProgressHook        func(processed, passed, failed, total int)
	TraceInterruptHook       func(stage, ip string, interrupt func()) func()
	DownloadProgressHook     func(processed, qualified, total int)
	DownloadSpeedSampleHook  func(sample DownloadSpeedSample)
	DownloadInterruptHook    func(stage, ip string, interrupt func()) func()
	ProbePauseHook           func(stage, ip string)
	ProbeCancelHook          func(stage, ip string) bool
	StageRejectHook          func(event StageRejectEvent)
	DownloadSpeedSampleStage = "stage3_get"
)

var stageRejectHookMu sync.Mutex

func SetStageRejectHook(hook func(event StageRejectEvent)) {
	stageRejectHookMu.Lock()
	StageRejectHook = hook
	stageRejectHookMu.Unlock()
}

func CheckProbePause(stage, ip string) {
	if ProbePauseHook != nil {
		ProbePauseHook(stage, ip)
	}
}

func IsProbeCanceled(stage, ip string) bool {
	if ProbeCancelHook == nil {
		return false
	}
	return ProbeCancelHook(stage, ip)
}

func ReportStageReject(event StageRejectEvent) {
	stageRejectHookMu.Lock()
	hook := StageRejectHook
	stageRejectHookMu.Unlock()
	if hook == nil {
		return
	}
	hook(event)
}
