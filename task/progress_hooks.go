package task

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
	DownloadSpeedSampleStage = "stage3_get"
)

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
