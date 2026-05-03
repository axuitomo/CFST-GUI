package mobileapi

import "sync"

const schemaVersion = "cfst-gui-mobile-v1"

type EventSink interface {
	OnProbeEvent(eventJSON string)
}

type Service struct {
	runMu sync.Mutex

	stateMu         sync.Mutex
	baseDir         string
	eventSink       EventSink
	eventSeq        int
	currentTaskID   string
	cancelTaskID    string
	cancelRequested bool
}

type probeConfig struct {
	Strategy                           string  `json:"strategy"`
	Routines                           int     `json:"routines"`
	HeadRoutines                       int     `json:"headRoutines"`
	PingTimes                          int     `json:"pingTimes"`
	SkipFirstLatency                   bool    `json:"skipFirstLatencySample"`
	EventThrottleMS                    int     `json:"eventThrottleMs"`
	DownloadSpeedSampleIntervalSeconds int     `json:"downloadSpeedSampleIntervalSeconds"`
	HeadTestCount                      int     `json:"headTestCount"`
	TestCount                          int     `json:"testCount"`
	Stage1Limit                        int     `json:"stage1Limit"`
	Stage1TimeoutMS                    int     `json:"stage1TimeoutMs"`
	Stage2TimeoutMS                    int     `json:"stage2TimeoutMs"`
	Stage3Concurrency                  int     `json:"stage3Concurrency"`
	DownloadTimeSeconds                int     `json:"downloadTimeSeconds"`
	TCPPort                            int     `json:"tcpPort"`
	URL                                string  `json:"url"`
	TraceURL                           string  `json:"traceUrl"`
	UserAgent                          string  `json:"userAgent"`
	HostHeader                         string  `json:"hostHeader"`
	SNI                                string  `json:"sni"`
	Httping                            bool    `json:"httping"`
	HttpingStatusCode                  int     `json:"httpingStatusCode"`
	HttpingCFColo                      string  `json:"httpingCFColo"`
	MaxDelayMS                         int     `json:"maxDelayMS"`
	HeadMaxDelayMS                     int     `json:"headMaxDelayMS"`
	MinDelayMS                         int     `json:"minDelayMS"`
	MaxLossRate                        float64 `json:"maxLossRate"`
	MinSpeedMB                         float64 `json:"minSpeedMB"`
	PrintNum                           int     `json:"printNum"`
	IPFile                             string  `json:"ipFile"`
	IPText                             string  `json:"ipText"`
	OutputFile                         string  `json:"outputFile"`
	WriteOutput                        bool    `json:"writeOutput"`
	ExportAppend                       bool    `json:"exportAppend"`
	DisableDownload                    bool    `json:"disableDownload"`
	TestAll                            bool    `json:"testAll"`
	RetryMaxAttempts                   int     `json:"retryMaxAttempts"`
	RetryBackoffMS                     int     `json:"retryBackoffMs"`
	CooldownFailures                   int     `json:"cooldownFailures"`
	CooldownMS                         int     `json:"cooldownMs"`
	Debug                              bool    `json:"debug"`
	DebugCaptureAddress                string  `json:"debugCaptureAddress"`
}

type commandResult struct {
	Code          string   `json:"code"`
	Data          any      `json:"data"`
	Message       string   `json:"message"`
	OK            bool     `json:"ok"`
	SchemaVersion string   `json:"schema_version"`
	TaskID        *string  `json:"task_id"`
	Warnings      []string `json:"warnings"`
}

type sourceSummary struct {
	CandidateCount int      `json:"candidateCount"`
	DuplicateCount int      `json:"duplicateCount"`
	Duplicates     []string `json:"duplicates"`
	Invalid        []string `json:"invalid"`
	InvalidCount   int      `json:"invalidCount"`
	RawLineCount   int      `json:"rawLineCount"`
	UniqueCount    int      `json:"uniqueCount"`
	Valid          []string `json:"valid"`
	ValidCount     int      `json:"validCount"`
}

type probeRunResult struct {
	Config         probeConfig           `json:"config"`
	DurationMS     int64                 `json:"durationMs"`
	OutputFile     string                `json:"outputFile"`
	Results        []probeRow            `json:"results"`
	Source         sourceSummary         `json:"source"`
	SourceStatuses []desktopSourceStatus `json:"sourceStatuses"`
	StartedAt      string                `json:"startedAt"`
	Summary        probeSummary          `json:"summary"`
	Warnings       []string              `json:"warnings"`
	SchemaVersion  string                `json:"schemaVersion"`
}

type probeSummary struct {
	AverageDelayMS float64 `json:"averageDelayMs"`
	BestIP         string  `json:"bestIp"`
	BestSpeedMB    float64 `json:"bestSpeedMb"`
	Failed         int     `json:"failed"`
	Passed         int     `json:"passed"`
	Total          int     `json:"total"`
}

type probeRow struct {
	Colo            string  `json:"colo"`
	DelayMS         float64 `json:"delayMs"`
	DownloadSpeedMB float64 `json:"downloadSpeedMb"`
	IP              string  `json:"ip"`
	LossRate        float64 `json:"lossRate"`
	Received        int     `json:"received"`
	Sended          int     `json:"sended"`
	TraceDelayMS    float64 `json:"traceDelayMs"`
}

type desktopSource struct {
	ColoFilter       string `json:"colo_filter"`
	Content          string `json:"content"`
	Enabled          bool   `json:"enabled"`
	ID               string `json:"id"`
	IPLimit          int    `json:"ip_limit"`
	IPMode           string `json:"ip_mode"`
	Kind             string `json:"kind"`
	Label            string `json:"label"`
	LastFetchedAt    string `json:"last_fetched_at"`
	LastFetchedCount int    `json:"last_fetched_count"`
	Name             string `json:"name"`
	Path             string `json:"path"`
	StatusText       string `json:"status_text"`
	URL              string `json:"url"`
}

type desktopSourceStatus struct {
	ID               string `json:"id"`
	LastFetchedAt    string `json:"last_fetched_at"`
	LastFetchedCount int    `json:"last_fetched_count"`
	StatusText       string `json:"status_text"`
}

type desktopProbePayload struct {
	AndroidExportURI string          `json:"android_export_uri"`
	Config           map[string]any  `json:"config"`
	Sources          []desktopSource `json:"sources"`
	TaskID           string          `json:"task_id"`
}

type sourcePreviewPayload struct {
	Config       map[string]any `json:"config"`
	PersistState bool           `json:"persist_state"`
	PreviewLimit int            `json:"preview_limit"`
	Source       desktopSource  `json:"source"`
}
