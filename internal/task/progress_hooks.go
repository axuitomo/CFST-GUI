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

const DownloadSpeedSampleStage = "stage3_get"
