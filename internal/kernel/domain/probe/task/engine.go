package task

import (
	"context"
	"net"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/kernel/domain/probe/colodict"
	"github.com/axuitomo/CFST-GUI/internal/kernel/infra/httpcfg"
	"github.com/axuitomo/CFST-GUI/internal/kernel/infra/httpclient"
	"github.com/axuitomo/CFST-GUI/internal/kernel/infra/utils"
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
	TraceStatusCode        int
	TraceCFColo            string
	TraceCFColoMode        string
	TraceCFColos           []string
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
	MinSpeed               float64
	DownloadSuccessLimit   int
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
	CustomHostSNI          bool
	DownloadHostHeader     string
	DownloadSNI            string
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
		Routines:               defaultRoutinesForPlatform(runtime.GOOS),
		TCPPort:                defaultPort,
		PingTimes:              defaultPingTimes,
		SkipFirstLatencySample: true,
		TCPConnectTimeout:      defaultTCPConnectTimeout,
		IPFile:                 defaultInputFile,
		TraceStatusCode:        DefaultTraceStatusCode,
		HeadRoutines:           defaultHeadRoutines,
		HeadTestCount:          defaultHeadTestCount,
		HeadTimeout:            defaultHeadTimeout,
		TraceURL:               defaultTraceURL,
		TraceColoMode:          TraceColoModeStandard,
		URL:                    defaultURL,
		Timeout:                defaultTimeout,
		MinSpeed:               defaultMinSpeed,
		MinSpeedMetric:         utils.DownloadSpeedMetricAverage,
		DownloadRoutines:       defaultDownloadRoutines,
		DownloadGetConcurrency: defaultDownloadGetConcurrency,
		DownloadBufferKB:       defaultDownloadBufferKB,
		DownloadHTTPProtocol:   string(httpclient.ProtocolAuto),
		DownloadSampleInterval: defaultDownloadSpeedSampleInterval,
		DownloadWarmupDuration: defaultDownloadWarmupDuration,
		InsecureSkipVerify:     true,
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
	DebugEnabled        func() bool
	EdgeRejected        func(rejection EdgeRejection)
}

// EdgeRejection 描述一次 403 / 1034 边缘拒绝，界面事件流与调试日志共用同一份事实。
type EdgeRejection struct {
	Stage   string
	IP      string
	Note    string // 403 或 1034
	Reason  string // forbidden 或 edge_ip_restricted
	Ignored bool   // true 表示按配置屏蔽：跳过该 IP，不重试也不计入连续失败冷却
}

type Engine struct {
	config Config
	hooks  Hooks

	cooldownMu    sync.Mutex
	cooldownFails map[string]int
	coloCache     engineColoDictionaryCache

	// workerPanics 记录异常退出的 worker 数。panic 只写调试日志，关掉 debug 时界面上
	// 完全看不到，所以另留一个不受 debug 开关约束的计数供上层拼 warning。
	workerPanics atomic.Int32

	// rejectMu 保护 rejectCounts：每个阶段按 reason 统计淘汰条数，阶段结束时
	// 由上层取走拼成一条「阶段汇总」，这样关掉调试日志也能看到漏斗为什么变窄。
	rejectMu     sync.Mutex
	rejectCounts map[string]map[string]int

	traceProbe            func(*Engine, *netIPAddr) traceProbeResult
	downloadHandler       func(*Engine, *netIPAddr) (float64, string)
	downloadHandlerResult func(*Engine, *netIPAddr) downloadResult
}

type engineColoDictionaryCache struct {
	sync.Mutex
	path    string
	lookup  *colodict.ColoLookup
	modTime time.Time
	size    int64
	// checkedAt is when the file was last stat'ed, which throttles the recheck.
	checkedAt time.Time
}

// netIPAddr is an alias used to keep test injection fields private without
// leaking implementation-only function types into the public API.
type netIPAddr = net.IPAddr

func NewEngine(config Config, hooks Hooks) *Engine {
	config.TraceCFColos = append([]string(nil), config.TraceCFColos...)
	config.SourceColoFilters = CloneSourceColoFilterMap(config.SourceColoFilters)
	return &Engine{
		config:        normalizeConfig(config),
		hooks:         hooks,
		cooldownFails: make(map[string]int),
	}
}

func (e *Engine) Config() Config {
	config := e.config
	config.TraceCFColos = append([]string(nil), e.config.TraceCFColos...)
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

// debugEnabled lets hot paths skip building a payload nobody will read.
func (e *Engine) debugEnabled() bool {
	if e == nil || e.hooks.DebugEvent == nil {
		return false
	}
	return e.hooks.DebugEnabled == nil || e.hooks.DebugEnabled()
}

func (e *Engine) debugEvent(event string, payload map[string]any) {
	if e == nil {
		return
	}
	// 淘汰事件同时进阶段计数：调试日志可以关，但阶段汇总必须一直有数据。
	if event == "stage.reject" {
		e.noteStageReject(stringField(payload, "stage"), stringField(payload, "reason"))
	}
	if e.hooks.DebugEvent != nil {
		e.hooks.DebugEvent(event, payload)
	}
}

func stringField(payload map[string]any, key string) string {
	value, _ := payload[key].(string)
	return value
}

// noteStageReject 按阶段累计淘汰原因。stage 为空时不计数，避免脏数据混进汇总。
func (e *Engine) noteStageReject(stage, reason string) {
	if e == nil || stage == "" || reason == "" {
		return
	}
	e.rejectMu.Lock()
	defer e.rejectMu.Unlock()
	if e.rejectCounts == nil {
		e.rejectCounts = make(map[string]map[string]int)
	}
	counts := e.rejectCounts[stage]
	if counts == nil {
		counts = make(map[string]int)
		e.rejectCounts[stage] = counts
	}
	counts[reason]++
}

// TakeStageRejectCounts 取走某阶段的淘汰原因计数（取走后清零），阶段汇总只读一次。
func (e *Engine) TakeStageRejectCounts(stage string) map[string]int {
	if e == nil || stage == "" {
		return nil
	}
	e.rejectMu.Lock()
	defer e.rejectMu.Unlock()
	counts := e.rejectCounts[stage]
	if counts == nil {
		return nil
	}
	delete(e.rejectCounts, stage)
	return counts
}

// noteEdgeRejected 把 403 / 1034 拒绝交给上层。和 debugEvent 不同，这条不受 debug 开关约束，
// 界面日志流靠它才能直接看到被边缘拒绝的 IP。
func (e *Engine) noteEdgeRejected(rejection EdgeRejection) {
	if e == nil {
		return
	}
	// 403 / 1034 不走 debugEvent，阶段汇总的淘汰原因要在这里单独计数。
	e.noteStageReject(rejection.Stage, rejection.Reason)
	if e.hooks.EdgeRejected != nil {
		e.hooks.EdgeRejected(rejection)
	}
}

func normalizeConfig(config Config) Config {
	defaults := DefaultConfig()
	if config.Routines <= 0 {
		config.Routines = defaults.Routines
	} else if config.Routines > MaxRoutines {
		config.Routines = MaxRoutines
	}
	if config.TCPPort <= 0 || config.TCPPort >= 65535 {
		config.TCPPort = defaults.TCPPort
	}
	if config.PingTimes <= 0 {
		config.PingTimes = defaults.PingTimes
	} else if config.PingTimes < MinPingTimes {
		config.PingTimes = MinPingTimes
	} else if config.PingTimes > MaxPingTimes {
		config.PingTimes = MaxPingTimes
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
	if config.MinSpeedMetric == "" {
		config.MinSpeedMetric = defaults.MinSpeedMetric
	}
	if config.DownloadRoutines <= 0 {
		config.DownloadRoutines = defaults.DownloadRoutines
	} else if config.DownloadRoutines > MaxDownloadRoutines {
		// Clamp to the ceiling instead of the default so a too-large value keeps
		// its intent (run faster) rather than silently going serial.
		config.DownloadRoutines = MaxDownloadRoutines
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

func makeBoundedChan[T any](size, fallback, ceiling int) chan T {
	if size <= 0 {
		return make(chan T, fallback)
	}
	if size > ceiling {
		return make(chan T, ceiling)
	}
	return make(chan T, size)
}
