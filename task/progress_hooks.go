package task

var (
	LatencyProgressHook  func(processed, passed, failed, total int)
	DownloadProgressHook func(processed, qualified, total int)
)
