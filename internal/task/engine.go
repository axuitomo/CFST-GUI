package task

import (
	"context"
	"net"
	"sync"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/colodict"
	"github.com/axuitomo/CFST-GUI/internal/httpcfg"
	"github.com/axuitomo/CFST-GUI/internal/httpclient"
	"github.com/axuitomo/CFST-GUI/internal/utils"
)

// Config is the complete mutable input for one probe engine. Engines copy the
// config on construction so callers may safely reuse and modify their input.
type Config struct {
	Routines               int
	TCPPort                int
	PingTimes              int
	SkipFirstLatencySample bool
	TCPConnectTimeout      time.Duration
	TestAll                bool
	IPFile                 string
	IPText                 string
	Httping                bool
	HttpingStatusCode      int
	HttpingCFColo          string
	HttpingCFColoMode      string
	HttpingCFColos         []string
	HeadRoutines           int
	HeadTestCount          int
	HeadMaxDelay           time.Duration
	HeadTimeout            time.Duration
	TraceURL               string
	TraceColoMode          string
	ColoDictionaryPath     string
	SourceColoFilters      SourceColoFilterMap
	URL                    string
	Timeout                time.Duration
	Disable                bool
	TestCount              int
	MinSpeed               float64
	MinSpeedMetric         string
	DownloadRoutines       int
	DownloadGetConcurrency int
	DownloadBufferKB       int
	DownloadHTTPProtocol   string
	DownloadSampleInterval time.Duration
	DownloadWarmupDuration time.Duration
	RetryMaxAttempts       int
	RetryBackoff           time.Duration
	CooldownFailures       int
	CooldownDuration       time.Duration
	UserAgent              string
	HostHeader             string
	SNI                    string
	RequestHeaders         string
	CaptureAddress         string
	InsecureSkipVerify     bool
	Debug                  bool
	InputMaxDelay          time.Duration
	InputMinDelay          time.Duration
	InputMaxLossRate       float32
	PrintNum               int
	Output                 string
	OutputAppend           bool
	OutputCSVEncoding      string
}

func DefaultConfig() Config {
	return Config{
		Routines:               defaultRoutines,
		TCPPort:                defaultPort,
		PingTimes:              defaultPingTimes,
		SkipFirstLatencySample: true,
		TCPConnectTimeout:      defaultTCPConnectTimeout,
		IPFile:                 defaultInputFile,
		HttpingStatusCode:      DefaultHTTPingStatusCode,
		HeadRoutines:           defaultHeadRoutines,
		HeadTestCount:          defaultHeadTestCount,
		HeadTimeout:            defaultHeadTimeout,
		TraceURL:               defaultTraceURL,
		TraceColoMode:          TraceColoModeStandard,
		URL:                    defaultURL,
		Timeout:                defaultTimeout,
		TestCount:              defaultTestNum,
		MinSpeed:               defaultMinSpeed,
		MinSpeedMetric:         utils.DownloadSpeedMetricAverage,
		DownloadRoutines:       defaultDownloadRoutines,
		DownloadGetConcurrency: defaultDownloadGetConcurrency,
		DownloadBufferKB:       defaultDownloadBufferKB,
		DownloadHTTPProtocol:   string(httpclient.ProtocolAuto),
		DownloadSampleInterval: defaultDownloadSpeedSampleInterval,
		DownloadWarmupDuration: defaultDownloadWarmupDuration,
		UserAgent:              httpcfg.DefaultUserAgent,
		InputMaxDelay:          utils.DefaultMaxDelay,
		InputMinDelay:          utils.DefaultMinDelay,
		InputMaxLossRate:       utils.DefaultMaxLossRate,
		PrintNum:               utils.DefaultPrintNum,
		Output:                 utils.DefaultOutput,
		OutputCSVEncoding:      utils.CSVEncodingUTF8,
	}
}

type Hooks struct {
	LatencyProgress     func(processed, passed, failed, total int)
	HeadProgress        func(processed, passed, failed, total int)
	TraceProgress       func(processed, passed, failed, total int)
	TraceDiagnostic     func(TraceDiagnostic)
	TraceInterrupt      func(stage, ip string, interrupt func()) func()
	DownloadProgress    func(processed, qualified, total int)
	DownloadSpeedSample func(sample DownloadSpeedSample)
	DownloadInterrupt   func(stage, ip string, interrupt func()) func()
	ProbePause          func(stage, ip string)
	ProbeCancel         func(stage, ip string) bool
	ProbeContext        func() context.Context
	DebugEvent          func(event string, payload map[string]any)
}

type Engine struct {
	config Config
	hooks  Hooks

	cooldownMu    sync.Mutex
	cooldownFails map[string]int
	coloCache     engineColoDictionaryCache

	traceProbe            func(*Engine, *netIPAddr) traceProbeResult
	downloadHandler       func(*Engine, *netIPAddr) (float64, string)
	downloadHandlerResult func(*Engine, *netIPAddr) downloadResult
}

type engineColoDictionaryCache struct {
	sync.Mutex
	path    string
	entries []colodict.ColoEntry
	modTime time.Time
	size    int64
}

// netIPAddr is an alias used to keep test injection fields private without
// leaking implementation-only function types into the public API.
type netIPAddr = net.IPAddr

func NewEngine(config Config, hooks Hooks) *Engine {
	config.HttpingCFColos = append([]string(nil), config.HttpingCFColos...)
	config.SourceColoFilters = CloneSourceColoFilterMap(config.SourceColoFilters)
	return &Engine{
		config:        normalizeConfig(config),
		hooks:         hooks,
		cooldownFails: make(map[string]int),
	}
}

func (e *Engine) Config() Config {
	config := e.config
	config.HttpingCFColos = append([]string(nil), e.config.HttpingCFColos...)
	config.SourceColoFilters = CloneSourceColoFilterMap(e.config.SourceColoFilters)
	return config
}

func (e *Engine) SetHooks(hooks Hooks) {
	e.hooks = hooks
}

func (e *Engine) FilterPingResults(input utils.PingDelaySet) utils.PingDelaySet {
	filter := utils.FilterConfig{
		MaxDelay: e.config.InputMaxDelay, MinDelay: e.config.InputMinDelay,
		MaxLossRate: e.config.InputMaxLossRate, DebugEvent: e.debugEvent,
	}
	return utils.FilterPingLossRate(utils.FilterPingDelay(input, filter), filter)
}

func (e *Engine) CSVWriter() utils.CSVWriter {
	return utils.CSVWriter{Path: e.config.Output, Append: e.config.OutputAppend, Encoding: e.config.OutputCSVEncoding}
}

func (e *Engine) EmitTraceDiagnostic(diagnostic TraceDiagnostic) {
	e.emitTraceDiagnostic(diagnostic)
}

func (e *Engine) CheckPause(stage, ip string) {
	e.checkPause(stage, ip)
}

func (e *Engine) ResetCooldownCounters() {
	e.cooldownMu.Lock()
	clear(e.cooldownFails)
	e.cooldownMu.Unlock()
}

func (e *Engine) checkPause(stage, ip string) {
	if e != nil && e.hooks.ProbePause != nil {
		e.hooks.ProbePause(stage, ip)
	}
}

func (e *Engine) isCanceled(stage, ip string) bool {
	return e != nil && e.hooks.ProbeCancel != nil && e.hooks.ProbeCancel(stage, ip)
}

func (e *Engine) context() context.Context {
	if e != nil && e.hooks.ProbeContext != nil {
		if ctx := e.hooks.ProbeContext(); ctx != nil {
			return ctx
		}
	}
	return context.Background()
}

func (e *Engine) debugEvent(event string, payload map[string]any) {
	if e != nil && e.hooks.DebugEvent != nil {
		e.hooks.DebugEvent(event, payload)
	}
}

func normalizeConfig(config Config) Config {
	defaults := DefaultConfig()
	if config.Routines <= 0 {
		config.Routines = defaults.Routines
	}
	if config.TCPPort <= 0 || config.TCPPort >= 65535 {
		config.TCPPort = defaults.TCPPort
	}
	if config.PingTimes <= 0 {
		config.PingTimes = defaults.PingTimes
	} else if config.PingTimes < MinPingTimes {
		config.PingTimes = MinPingTimes
	}
	if config.TCPConnectTimeout <= 0 {
		config.TCPConnectTimeout = defaults.TCPConnectTimeout
	}
	if config.IPFile == "" {
		config.IPFile = defaults.IPFile
	}
	config.HeadRoutines = NormalizeTraceRoutines(config.HeadRoutines)
	if config.HeadTestCount <= 0 {
		config.HeadTestCount = defaults.HeadTestCount
	}
	if config.HeadTimeout <= 0 {
		config.HeadTimeout = defaults.HeadTimeout
	}
	if config.TraceURL == "" {
		config.TraceURL = defaults.TraceURL
	}
	if config.TraceColoMode == "" {
		config.TraceColoMode = defaults.TraceColoMode
	}
	if config.URL == "" {
		config.URL = defaults.URL
	}
	if config.Timeout <= 0 {
		config.Timeout = defaults.Timeout
	}
	if config.TestCount <= 0 {
		config.TestCount = defaults.TestCount
	}
	if config.MinSpeedMetric == "" {
		config.MinSpeedMetric = defaults.MinSpeedMetric
	}
	if config.DownloadRoutines <= 0 || config.DownloadRoutines > MaxDownloadRoutines {
		config.DownloadRoutines = defaults.DownloadRoutines
	}
	if config.DownloadGetConcurrency <= 0 {
		config.DownloadGetConcurrency = defaults.DownloadGetConcurrency
	} else if config.DownloadGetConcurrency > MaxDownloadGetConcurrency {
		config.DownloadGetConcurrency = MaxDownloadGetConcurrency
	}
	if config.DownloadBufferKB < MinDownloadBufferKB || config.DownloadBufferKB > MaxDownloadBufferKB {
		config.DownloadBufferKB = defaults.DownloadBufferKB
	}
	config.DownloadHTTPProtocol = string(httpclient.NormalizeProtocol(config.DownloadHTTPProtocol, httpclient.ProtocolAuto))
	if config.DownloadSampleInterval <= 0 {
		config.DownloadSampleInterval = defaults.DownloadSampleInterval
	}
	if config.DownloadWarmupDuration < 0 {
		config.DownloadWarmupDuration = 0
	}
	if config.UserAgent == "" {
		config.UserAgent = defaults.UserAgent
	}
	return config
}
