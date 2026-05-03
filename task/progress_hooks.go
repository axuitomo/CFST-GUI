package task

type DownloadSpeedSample struct {
	Stage           string
	IP              string
	CurrentSpeedMBs float64
	AverageSpeedMBs float64
	BytesRead       int64
	ElapsedMS       int64
	Colo            string
}

var (
	LatencyProgressHook      func(processed, passed, failed, total int)
	HeadProgressHook         func(processed, passed, failed, total int)
	TraceProgressHook        func(processed, passed, failed, total int)
	DownloadProgressHook     func(processed, qualified, total int)
	DownloadSpeedSampleHook  func(sample DownloadSpeedSample)
	ProbePauseHook           func(stage, ip string)
	DownloadSpeedSampleStage = "stage3_get"
)

func CheckProbePause(stage, ip string) {
	if ProbePauseHook != nil {
		ProbePauseHook(stage, ip)
	}
}
