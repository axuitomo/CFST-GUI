package probecore

import (
	"fmt"
	"net/url"
	"runtime"
	"strings"

	"github.com/axuitomo/CFST-GUI/internal/httpcfg"
	"github.com/axuitomo/CFST-GUI/internal/httpclient"
	"github.com/axuitomo/CFST-GUI/internal/task"
	"github.com/axuitomo/CFST-GUI/internal/utils"
)

const (
	DefaultFileTestURL              = "https://speedtest.xyz9923.dpdns.org/500m"
	SourceColoFilterPhasePrecheck   = "precheck"
	DefaultMaxProbeTCPRoutines      = 1000
	DefaultMaxProbeStage3Routines   = task.MaxDownloadRoutines
	defaultProbeStage3Concurrency   = 1
	defaultProbeEventThrottleMS     = 100
	defaultProbeDownloadIntervalMS  = 500
	defaultProbeDownloadTimeSeconds = 4
	defaultProbeDownloadWarmupSec   = 1
)

type ProbeConfig struct {
	Strategy                           string  `json:"strategy"`
	Routines                           int     `json:"routines"`
	HeadRoutines                       int     `json:"headRoutines"`
	PingTimes                          int     `json:"pingTimes"`
	SkipFirstLatency                   bool    `json:"skipFirstLatencySample"`
	EventThrottleMS                    int     `json:"eventThrottleMs"`
	DownloadSpeedSampleIntervalMS      int     `json:"downloadSpeedSampleIntervalMs"`
	DownloadSpeedSampleIntervalSeconds int     `json:"downloadSpeedSampleIntervalSeconds"`
	DownloadGetConcurrency             int     `json:"downloadGetConcurrency"`
	DownloadBufferKB                   int     `json:"downloadBufferKB"`
	DownloadHTTPProtocol               string  `json:"downloadHTTPProtocol"`
	DownloadSpeedMetric                string  `json:"downloadSpeedMetric"`
	HeadTestCount                      int     `json:"headTestCount"`
	TestCount                          int     `json:"testCount"`
	Stage1Limit                        int     `json:"stage1Limit"`
	Stage3Limit                        int     `json:"stage3Limit"`
	Stage1TimeoutMS                    int     `json:"stage1TimeoutMs"`
	Stage2TimeoutMS                    int     `json:"stage2TimeoutMs"`
	Stage3Concurrency                  int     `json:"stage3Concurrency"`
	DownloadTimeSeconds                int     `json:"downloadTimeSeconds"`
	DownloadWarmupSeconds              int     `json:"downloadWarmupSeconds"`
	PortPolicy                         string  `json:"portPolicy"`
	TCPPort                            int     `json:"tcpPort"`
	URL                                string  `json:"url"`
	TraceURL                           string  `json:"traceUrl"`
	TraceColoMode                      string  `json:"traceColoMode"`
	SourceColoFilterPhase              string  `json:"sourceColoFilterPhase"`
	UserAgent                          string  `json:"userAgent"`
	HostHeader                         string  `json:"hostHeader"`
	SNI                                string  `json:"sni"`
	RequestHeaders                     string  `json:"requestHeaders"`
	Httping                            bool    `json:"httping"`
	HttpingStatusCode                  int     `json:"httpingStatusCode"`
	HttpingCFColo                      string  `json:"httpingCFColo"`
	HttpingCFColoMode                  string  `json:"httpingCFColoMode"`
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
	CSVEncoding                        string  `json:"csvEncoding"`
	DisableDownload                    bool    `json:"disableDownload"`
	TestAll                            bool    `json:"testAll"`
	RetryMaxAttempts                   int     `json:"retryMaxAttempts"`
	RetryBackoffMS                     int     `json:"retryBackoffMs"`
	CooldownFailures                   int     `json:"cooldownFailures"`
	CooldownMS                         int     `json:"cooldownMs"`
	Debug                              bool    `json:"debug"`
	DebugCaptureEnabled                bool    `json:"debugCaptureEnabled"`
	DebugCaptureAddress                string  `json:"debugCaptureAddress"`
	DebugLogMode                       string  `json:"debugLogMode"`
	DebugLogFormat                     string  `json:"debugLogFormat"`
	DebugLogVerbosity                  string  `json:"debugLogVerbosity"`
}

type ProbeConfigNormalizeOptions struct {
	MaxTCPRoutines    int
	MaxStage3Routines int
}

func DefaultProbeConfig() ProbeConfig {
	return ProbeConfig{
		Strategy:                           "fast",
		Routines:                           200,
		HeadRoutines:                       task.MaxTraceRoutines,
		PingTimes:                          4,
		SkipFirstLatency:                   true,
		EventThrottleMS:                    defaultProbeEventThrottleMS,
		DownloadSpeedSampleIntervalMS:      defaultProbeDownloadIntervalMS,
		DownloadSpeedSampleIntervalSeconds: 0,
		DownloadGetConcurrency:             4,
		DownloadBufferKB:                   256,
		DownloadHTTPProtocol:               string(httpclient.ProtocolAuto),
		DownloadSpeedMetric:                utils.DownloadSpeedMetricAverage,
		HeadTestCount:                      0,
		TestCount:                          10,
		Stage1Limit:                        0,
		Stage3Limit:                        10,
		Stage1TimeoutMS:                    1000,
		Stage2TimeoutMS:                    1000,
		Stage3Concurrency:                  defaultProbeStage3Concurrency,
		DownloadTimeSeconds:                defaultProbeDownloadTimeSeconds,
		DownloadWarmupSeconds:              defaultProbeDownloadWarmupSec,
		PortPolicy:                         PortPolicySourceOverrideGlobal,
		TCPPort:                            443,
		URL:                                DefaultFileTestURL,
		TraceURL:                           "",
		TraceColoMode:                      task.TraceColoModeStandard,
		SourceColoFilterPhase:              SourceColoFilterPhasePrecheck,
		UserAgent:                          httpcfg.DefaultUserAgent,
		HostHeader:                         "",
		SNI:                                "",
		RequestHeaders:                     "",
		Httping:                            false,
		HttpingStatusCode:                  0,
		HttpingCFColo:                      "",
		HttpingCFColoMode:                  task.ColoFilterModeAllow,
		MaxDelayMS:                         9999,
		HeadMaxDelayMS:                     0,
		MinDelayMS:                         0,
		MaxLossRate:                        float64(utils.DefaultMaxLossRate),
		MinSpeedMB:                         0,
		PrintNum:                           0,
		IPFile:                             "ip.txt",
		OutputFile:                         "result.csv",
		WriteOutput:                        true,
		ExportAppend:                       false,
		CSVEncoding:                        utils.CSVEncodingUTF8,
		DisableDownload:                    true,
		TestAll:                            false,
		RetryMaxAttempts:                   0,
		RetryBackoffMS:                     0,
		CooldownFailures:                   3,
		CooldownMS:                         250,
		Debug:                              false,
		DebugCaptureEnabled:                false,
		DebugCaptureAddress:                "",
		DebugLogMode:                       utils.DebugLogModeStructured,
		DebugLogFormat:                     "",
		DebugLogVerbosity:                  utils.DebugLogVerbosityDetailed,
	}
}

func NormalizeProbeConfig(cfg ProbeConfig, options ProbeConfigNormalizeOptions) (ProbeConfig, []string) {
	options = normalizeProbeConfigOptions(options)
	def := DefaultProbeConfig()
	warnings := make([]string, 0)
	warn := func(format string, args ...any) {
		warnings = append(warnings, fmt.Sprintf(format, args...))
	}
	if cfg.Strategy == "" && cfg.Routines == 0 && cfg.PingTimes == 0 && cfg.URL == "" {
		def.TraceURL, _ = DeriveTraceURL(def.URL)
		return def, nil
	}
	switch strategy := strings.ToLower(strings.TrimSpace(cfg.Strategy)); strategy {
	case "":
		cfg.Strategy = def.Strategy
	case "fast", "latency", "http-colo":
		cfg.Strategy = "fast"
	case "full", "speed", "exhaustive":
		cfg.Strategy = "full"
	default:
		warn("未知探测策略 %q，已改为 %s。", cfg.Strategy, def.Strategy)
		cfg.Strategy = def.Strategy
	}
	if cfg.Routines <= 0 {
		warn("TCP并发线程必须大于 0，已改为 %d。", def.Routines)
		cfg.Routines = def.Routines
	} else if cfg.Routines > options.MaxTCPRoutines {
		warn("TCP并发线程最大支持 %d，已改为 %d。", options.MaxTCPRoutines, options.MaxTCPRoutines)
		cfg.Routines = options.MaxTCPRoutines
	}
	normalizedHeadRoutines := task.NormalizeTraceRoutines(cfg.HeadRoutines)
	if normalizedHeadRoutines != cfg.HeadRoutines {
		if cfg.HeadRoutines > task.MaxTraceRoutines {
			warn("追踪并发线程最大支持 %d，已改为 %d。", task.MaxTraceRoutines, normalizedHeadRoutines)
		} else {
			warn("追踪并发线程必须大于 0，已改为 %d。", normalizedHeadRoutines)
		}
	}
	cfg.HeadRoutines = normalizedHeadRoutines
	if cfg.PingTimes <= 0 {
		warn("TCP 发包次数必须大于 0，已改为 %d。", def.PingTimes)
		cfg.PingTimes = def.PingTimes
	} else if cfg.PingTimes < task.MinPingTimes {
		warn("TCP 发包次数必须至少为 %d，已改为 %d。", task.MinPingTimes, task.MinPingTimes)
		cfg.PingTimes = task.MinPingTimes
	}
	if !cfg.SkipFirstLatency {
		cfg.SkipFirstLatency = def.SkipFirstLatency
	}
	if cfg.TestCount <= 0 {
		cfg.TestCount = def.TestCount
	}
	if cfg.Stage3Limit <= 0 {
		cfg.Stage3Limit = cfg.TestCount
	}
	if cfg.Stage3Limit <= 0 {
		warn("阶段三候选上限必须大于 0，已改为 %d。", def.Stage3Limit)
		cfg.Stage3Limit = def.Stage3Limit
	}
	if cfg.Stage1TimeoutMS <= 0 {
		warn("阶段1 TCP 超时必须大于 0，已改为 %dms。", def.Stage1TimeoutMS)
		cfg.Stage1TimeoutMS = def.Stage1TimeoutMS
	}
	if cfg.Stage2TimeoutMS <= 0 {
		warn("追踪超时必须大于 0，已改为 %dms。", def.Stage2TimeoutMS)
		cfg.Stage2TimeoutMS = def.Stage2TimeoutMS
	}
	if cfg.Stage3Concurrency != def.Stage3Concurrency {
		warn("测速并发线程固定为 %d，已忽略配置值 %d。", options.MaxStage3Routines, cfg.Stage3Concurrency)
		cfg.Stage3Concurrency = def.Stage3Concurrency
	}
	if cfg.EventThrottleMS <= 0 {
		warn("事件节流必须大于 0，已改为 %dms。", def.EventThrottleMS)
		cfg.EventThrottleMS = def.EventThrottleMS
	}
	if cfg.DownloadSpeedSampleIntervalMS <= 0 {
		warn("下载速度采样间隔必须大于 0，已改为 %dms。", def.DownloadSpeedSampleIntervalMS)
		cfg.DownloadSpeedSampleIntervalMS = def.DownloadSpeedSampleIntervalMS
	}
	if cfg.DownloadGetConcurrency <= 0 {
		warn("单 IP GET 分片并发必须大于 0，已改为 %d。", def.DownloadGetConcurrency)
		cfg.DownloadGetConcurrency = def.DownloadGetConcurrency
	} else if cfg.DownloadGetConcurrency > task.MaxDownloadGetConcurrency {
		warn("单 IP GET 分片并发最大支持 %d，已改为 %d。", task.MaxDownloadGetConcurrency, task.MaxDownloadGetConcurrency)
		cfg.DownloadGetConcurrency = task.MaxDownloadGetConcurrency
	}
	if cfg.DownloadBufferKB <= 0 {
		warn("下载缓冲必须大于 0，已改为 %d KiB。", def.DownloadBufferKB)
		cfg.DownloadBufferKB = def.DownloadBufferKB
	} else if cfg.DownloadBufferKB < task.MinDownloadBufferKB {
		warn("下载缓冲最小支持 %d KiB，已改为 %d KiB。", task.MinDownloadBufferKB, task.MinDownloadBufferKB)
		cfg.DownloadBufferKB = task.MinDownloadBufferKB
	} else if cfg.DownloadBufferKB > task.MaxDownloadBufferKB {
		warn("下载缓冲最大支持 %d KiB，已改为 %d KiB。", task.MaxDownloadBufferKB, task.MaxDownloadBufferKB)
		cfg.DownloadBufferKB = task.MaxDownloadBufferKB
	}
	rawDownloadProtocol := strings.TrimSpace(cfg.DownloadHTTPProtocol)
	normalizedDownloadProtocol := httpclient.NormalizeProtocol(rawDownloadProtocol, "")
	if rawDownloadProtocol == "" {
		normalizedDownloadProtocol = httpclient.ProtocolAuto
	} else if normalizedDownloadProtocol == "" {
		warn("未知下载 HTTP 协议 %q，已改为 auto。", cfg.DownloadHTTPProtocol)
		normalizedDownloadProtocol = httpclient.ProtocolAuto
	}
	if normalizedDownloadProtocol == httpclient.ProtocolAuto {
		if fallbackProtocol := platformDownloadAutoFallback(runtime.GOOS, runtime.GOARCH); fallbackProtocol != "" {
			warn("当前平台 %s/%s 默认将下载 HTTP 协议 auto 调整为 %s，以避免 H3/UDP 异常。", runtime.GOOS, runtime.GOARCH, fallbackProtocol)
			normalizedDownloadProtocol = fallbackProtocol
		}
	}
	cfg.DownloadHTTPProtocol = string(normalizedDownloadProtocol)
	cfg.DownloadSpeedMetric = utils.NormalizeDownloadSpeedMetric(cfg.DownloadSpeedMetric)
	if cfg.DownloadTimeSeconds <= 0 {
		warn("单 IP 下载测速时间必须大于 0，已改为 %d 秒。", def.DownloadTimeSeconds)
		cfg.DownloadTimeSeconds = def.DownloadTimeSeconds
	}
	if cfg.DownloadWarmupSeconds < 0 {
		warn("下载预热时间不能为负数，已改为 %d 秒。", def.DownloadWarmupSeconds)
		cfg.DownloadWarmupSeconds = def.DownloadWarmupSeconds
	}
	if cfg.TCPPort <= 0 || cfg.TCPPort > 65535 {
		warn("测速端口必须在 1-65535 之间，已改为 %d。", def.TCPPort)
		cfg.TCPPort = def.TCPPort
	}
	rawPortPolicy := strings.TrimSpace(cfg.PortPolicy)
	switch strings.ToLower(rawPortPolicy) {
	case "", PortPolicySourceOverrideGlobal:
		cfg.PortPolicy = def.PortPolicy
	case PortPolicyFixedGlobal:
		cfg.PortPolicy = PortPolicyFixedGlobal
	default:
		normalizedPortPolicy := NormalizePortPolicy(rawPortPolicy)
		if normalizedPortPolicy == PortPolicyFixedGlobal || normalizedPortPolicy == PortPolicySourceOverrideGlobal {
			cfg.PortPolicy = normalizedPortPolicy
			if normalizedPortPolicy != rawPortPolicy {
				warn("端口策略 %q 已规范化为 %s。", rawPortPolicy, normalizedPortPolicy)
			}
		} else {
			warn("未知端口策略 %q，已改为 %s。", cfg.PortPolicy, def.PortPolicy)
			cfg.PortPolicy = def.PortPolicy
		}
	}
	if strings.TrimSpace(cfg.URL) == "" {
		warn("文件测速URL不能为空，已改为 %s。", def.URL)
		cfg.URL = def.URL
	}
	cfg.URL = NormalizeProbeURLInput(cfg.URL)
	cfg.TraceURL = NormalizeProbeURLInput(cfg.TraceURL)
	switch phase := strings.ToLower(strings.TrimSpace(cfg.SourceColoFilterPhase)); phase {
	case "", SourceColoFilterPhasePrecheck, "cloudflare-colos", "cloudflare_colos", "colo", "dictionary":
		cfg.SourceColoFilterPhase = SourceColoFilterPhasePrecheck
	case SourceColoFilterPhaseStage2, "stage-2", "second_stage", "second-stage":
		cfg.SourceColoFilterPhase = SourceColoFilterPhaseStage2
	default:
		warn("未知输入源 COLO 筛选阶段 %q，已改为 %s。", cfg.SourceColoFilterPhase, SourceColoFilterPhasePrecheck)
		cfg.SourceColoFilterPhase = SourceColoFilterPhasePrecheck
	}
	switch mode := strings.ToLower(strings.TrimSpace(cfg.TraceColoMode)); mode {
	case "", task.TraceColoModeStandard:
		cfg.TraceColoMode = task.TraceColoModeStandard
	case task.TraceColoModeTraceURL, "trace-url", "traceurl":
		cfg.TraceColoMode = task.TraceColoModeTraceURL
	default:
		warn("未知第二阶段 COLO 获取模式 %q，已改为 %s。", cfg.TraceColoMode, task.TraceColoModeStandard)
		cfg.TraceColoMode = task.TraceColoModeStandard
	}
	if cfg.TraceURL == "" {
		if derived, ok := DeriveTraceURL(cfg.URL); ok {
			cfg.TraceURL = derived
		} else if derived, ok := DeriveTraceURL(def.URL); ok {
			warn("追踪 URL 无法从文件测速URL派生，已改为 %s。", derived)
			cfg.TraceURL = derived
		}
	} else if !IsValidProbeURL(cfg.TraceURL) {
		if derived, ok := DeriveTraceURL(cfg.URL); ok {
			warn("追踪 URL 无效，已改为 %s。", derived)
			cfg.TraceURL = derived
		}
	}
	if (!cfg.DisableDownload || cfg.Strategy == "full") && IsTraceProbeURL(cfg.URL) {
		warn("文件测速URL当前指向 /cdn-cgi/trace；完整模式建议填写真实文件 URL，追踪 URL 会单独用于追踪阶段。")
	}
	if strings.TrimSpace(cfg.UserAgent) == "" {
		warn("User-Agent 不能为空，已改为默认值。")
		cfg.UserAgent = def.UserAgent
	}
	if normalizedHeaders, headerWarnings := httpcfg.NormalizeRequestHeaders(cfg.RequestHeaders); len(headerWarnings) > 0 || normalizedHeaders != cfg.RequestHeaders {
		warnings = append(warnings, headerWarnings...)
		cfg.RequestHeaders = normalizedHeaders
	}
	if cfg.HttpingStatusCode != 0 && (cfg.HttpingStatusCode < 100 || cfg.HttpingStatusCode > 599) {
		warn("追踪有效状态码必须为 0 或 100-599，已改为 %d。", def.HttpingStatusCode)
		cfg.HttpingStatusCode = def.HttpingStatusCode
	}
	cfg.HttpingCFColoMode = task.NormalizeColoFilterMode(cfg.HttpingCFColoMode)
	if cfg.MaxDelayMS <= 0 {
		warn("TCP 延迟上限必须大于 0，已改为 %dms。", def.MaxDelayMS)
		cfg.MaxDelayMS = def.MaxDelayMS
	}
	if cfg.HeadMaxDelayMS != 0 {
		warn("追踪延迟上限设置已停用，运行时固定不限制。")
		cfg.HeadMaxDelayMS = def.HeadMaxDelayMS
	}
	if cfg.MinDelayMS < 0 {
		warn("TCP 延迟下限不能为负数，已改为 %d。", def.MinDelayMS)
		cfg.MinDelayMS = def.MinDelayMS
	}
	if cfg.MaxLossRate < 0 {
		warn("TCP 丢包率上限不能为负数，已改为 %.2f。", def.MaxLossRate)
		cfg.MaxLossRate = def.MaxLossRate
	} else if cfg.MaxLossRate > float64(utils.MaxAllowedLossRate) {
		warn("TCP 丢包率上限最大支持 %.0f%%，已改为 %.2f。", float64(utils.MaxAllowedLossRate)*100, float64(utils.MaxAllowedLossRate))
		cfg.MaxLossRate = float64(utils.MaxAllowedLossRate)
	}
	if cfg.MinSpeedMB < 0 {
		warn("最低下载速度不能为负数，已改为 %.2f MB/s。", def.MinSpeedMB)
		cfg.MinSpeedMB = def.MinSpeedMB
	}
	if cfg.PrintNum < 0 {
		cfg.PrintNum = 0
	}
	if cfg.RetryMaxAttempts < 0 {
		warn("重试最大次数不能为负数，已改为 %d。", def.RetryMaxAttempts)
		cfg.RetryMaxAttempts = def.RetryMaxAttempts
	}
	if cfg.RetryBackoffMS < 0 {
		warn("重试退避不能为负数，已改为 %dms。", def.RetryBackoffMS)
		cfg.RetryBackoffMS = def.RetryBackoffMS
	}
	if cfg.CooldownFailures < 0 {
		warn("连续失败冷却阈值不能为负数，已改为 %d。", def.CooldownFailures)
		cfg.CooldownFailures = def.CooldownFailures
	}
	if cfg.CooldownMS < 0 {
		warn("冷却时长不能为负数，已改为 %dms。", def.CooldownMS)
		cfg.CooldownMS = def.CooldownMS
	}
	if strings.TrimSpace(cfg.IPFile) == "" {
		warn("IP 文件路径不能为空，已改为 %s。", def.IPFile)
		cfg.IPFile = def.IPFile
	}
	if cfg.WriteOutput && strings.TrimSpace(cfg.OutputFile) == "" {
		warn("导出文件路径不能为空，已改为 %s。", def.OutputFile)
		cfg.OutputFile = def.OutputFile
	}
	cfg.UserAgent = strings.TrimSpace(cfg.UserAgent)
	cfg.HostHeader = strings.TrimSpace(cfg.HostHeader)
	cfg.SNI = strings.TrimSpace(cfg.SNI)
	cfg.RequestHeaders = strings.TrimSpace(cfg.RequestHeaders)
	cfg.HttpingCFColo = strings.TrimSpace(cfg.HttpingCFColo)
	cfg.IPFile = strings.TrimSpace(cfg.IPFile)
	cfg.OutputFile = strings.TrimSpace(cfg.OutputFile)
	rawCSVEncoding := strings.TrimSpace(cfg.CSVEncoding)
	cfg.CSVEncoding = utils.NormalizeCSVEncoding(rawCSVEncoding)
	if rawCSVEncoding != "" && !utils.IsKnownCSVEncoding(rawCSVEncoding) {
		warn("未知 CSV 编码 %q，已改为 %s。", rawCSVEncoding, utils.CSVEncodingUTF8)
	}
	cfg.DebugCaptureAddress = strings.TrimSpace(cfg.DebugCaptureAddress)
	if cfg.DebugCaptureAddress == "" {
		cfg.DebugCaptureEnabled = false
	}
	cfg.DebugLogMode = strings.ToLower(strings.TrimSpace(cfg.DebugLogMode))
	switch cfg.DebugLogMode {
	case "", utils.DebugLogModeStructured:
		cfg.DebugLogMode = utils.DebugLogModeStructured
	case utils.DebugLogModeFreeform:
		warn("调试日志模式 %q 已停用，已改为 %s。", cfg.DebugLogMode, utils.DebugLogModeStructured)
		cfg.DebugLogMode = utils.DebugLogModeStructured
	default:
		warn("未知调试日志模式 %q，已改为 %s。", cfg.DebugLogMode, utils.DebugLogModeStructured)
		cfg.DebugLogMode = utils.DebugLogModeStructured
	}
	cfg.DebugLogFormat = strings.TrimSpace(cfg.DebugLogFormat)
	if cfg.DebugLogMode == utils.DebugLogModeStructured {
		cfg.DebugLogFormat = ""
	}
	cfg.DebugLogVerbosity = strings.ToLower(strings.TrimSpace(cfg.DebugLogVerbosity))
	switch cfg.DebugLogVerbosity {
	case "", utils.DebugLogVerbosityDetailed:
		cfg.DebugLogVerbosity = utils.DebugLogVerbosityDetailed
	case utils.DebugLogVerbositySimple:
		cfg.DebugLogVerbosity = utils.DebugLogVerbositySimple
	default:
		warn("未知调试日志粒度 %q，已改为 %s。", cfg.DebugLogVerbosity, utils.DebugLogVerbosityDetailed)
		cfg.DebugLogVerbosity = utils.DebugLogVerbosityDetailed
	}
	return cfg, DedupeStrings(warnings)
}

func platformDownloadAutoFallback(goos, goarch string) httpclient.Protocol {
	if strings.EqualFold(strings.TrimSpace(goos), "linux") {
		switch strings.ToLower(strings.TrimSpace(goarch)) {
		case "arm", "arm64":
			return httpclient.ProtocolTCP
		}
	}
	return ""
}

func DeriveTraceURL(rawURL string) (string, bool) {
	parsed, err := url.Parse(NormalizeProbeURLInput(rawURL))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", false
	}
	parsed.Path = "/cdn-cgi/trace"
	parsed.RawPath = ""
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), true
}

func IsValidProbeURL(rawURL string) bool {
	parsed, err := url.Parse(NormalizeProbeURLInput(rawURL))
	return err == nil && parsed.Scheme != "" && parsed.Host != ""
}

func IsTraceProbeURL(rawURL string) bool {
	parsed, err := url.Parse(NormalizeProbeURLInput(rawURL))
	if err != nil {
		return false
	}
	return strings.EqualFold(strings.TrimRight(parsed.EscapedPath(), "/"), "/cdn-cgi/trace")
}

func NormalizeProbeURLInput(rawURL string) string {
	value := strings.TrimSpace(rawURL)
	for strings.Contains(value, `\/`) {
		value = strings.ReplaceAll(value, `\/`, `/`)
	}
	return value
}

func normalizeProbeConfigOptions(options ProbeConfigNormalizeOptions) ProbeConfigNormalizeOptions {
	if options.MaxTCPRoutines <= 0 {
		options.MaxTCPRoutines = DefaultMaxProbeTCPRoutines
	}
	if options.MaxStage3Routines <= 0 {
		options.MaxStage3Routines = DefaultMaxProbeStage3Routines
	}
	return options
}
