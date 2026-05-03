package mobileapi

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/XIU2/CloudflareSpeedTest/internal/httpcfg"
	"github.com/XIU2/CloudflareSpeedTest/task"
	"github.com/XIU2/CloudflareSpeedTest/utils"
)

const (
	maxMobileTCPRoutines       = 1000
	maxMobileStage3Routines    = task.MaxDownloadRoutines
	defaultFileTestURL         = "https://speed.cloudflare.com/__down?bytes=10000000"
	defaultMobileSourceIPLimit = 500
)

func (s *Service) LoadConfig() string {
	path := s.configPath()
	snapshot := defaultConfigSnapshot()
	profiles, profileErr := s.loadProfileStore()
	warnings := make([]string, 0)
	if profileErr != nil {
		warnings = append(warnings, fmt.Sprintf("读取配置档案失败：%v", profileErr))
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return encodeCommand(commandResultFor("CONFIG_READY", map[string]any{
				"configPath":      path,
				"config_snapshot": snapshot,
				"profiles":        profiles,
				"storage":         s.storageStatus(),
			}, "移动端配置文件尚未创建，已加载默认配置。", true, nil, warnings))
		}
		return encodeCommand(commandResultFor("CONFIG_READ_FAILED", nil, err.Error(), false, nil, nil))
	}

	var saved map[string]any
	if err := json.Unmarshal(raw, &saved); err != nil {
		return encodeCommand(commandResultFor("CONFIG_PARSE_FAILED", nil, err.Error(), false, nil, nil))
	}
	if value, ok := saved["config_snapshot"].(map[string]any); ok {
		snapshot = value
	} else {
		snapshot = saved
	}
	_, configWarnings := configToProbeConfig(snapshot)
	warnings = append(warnings, configWarnings...)
	return encodeCommand(commandResultFor("CONFIG_READ_OK", map[string]any{
		"configPath":      path,
		"config_snapshot": snapshot,
		"profiles":        profiles,
		"storage":         s.storageStatus(),
	}, "移动端配置已加载。", true, nil, warnings))
}

func (s *Service) SaveConfig(payloadJSON string) string {
	payload, err := decodeObject(payloadJSON)
	if err != nil {
		return encodeCommand(commandResultFor("CONFIG_INVALID", nil, err.Error(), false, nil, nil))
	}
	snapshot := mapValue(payload["config_snapshot"])
	if len(snapshot) == 0 {
		return encodeCommand(commandResultFor("CONFIG_INVALID", nil, "缺少 config_snapshot。", false, nil, nil))
	}
	if err := s.writeConfigSnapshot(snapshot); err != nil {
		return encodeCommand(commandResultFor("CONFIG_WRITE_FAILED", nil, err.Error(), false, nil, nil))
	}
	_, warnings := configToProbeConfig(snapshot)
	profiles, profileErr := s.loadProfileStore()
	if profileErr != nil {
		warnings = append(warnings, fmt.Sprintf("读取配置档案失败：%v", profileErr))
	}
	return encodeCommand(commandResultFor("CONFIG_SAVE_OK", map[string]any{
		"configPath":      s.configPath(),
		"config_snapshot": snapshot,
		"profiles":        profiles,
		"storage":         s.storageStatus(),
	}, "移动端配置已保存。", true, nil, warnings))
}

func defaultProbeConfig() probeConfig {
	return probeConfig{
		Strategy:                           "fast",
		Routines:                           200,
		HeadRoutines:                       task.MaxTraceRoutines,
		PingTimes:                          4,
		SkipFirstLatency:                   true,
		EventThrottleMS:                    100,
		DownloadSpeedSampleIntervalSeconds: 2,
		HeadTestCount:                      512,
		TestCount:                          10,
		Stage1Limit:                        512,
		Stage3Limit:                        10,
		Stage1TimeoutMS:                    1000,
		Stage2TimeoutMS:                    1000,
		Stage3Concurrency:                  1,
		DownloadTimeSeconds:                10,
		TCPPort:                            443,
		URL:                                defaultFileTestURL,
		TraceURL:                           "",
		UserAgent:                          httpcfg.DefaultUserAgent,
		HostHeader:                         "",
		SNI:                                "",
		Httping:                            false,
		HttpingStatusCode:                  0,
		HttpingCFColo:                      "",
		MaxDelayMS:                         9999,
		HeadMaxDelayMS:                     0,
		MinDelayMS:                         0,
		MaxLossRate:                        float64(utils.MaxAllowedLossRate),
		MinSpeedMB:                         0,
		PrintNum:                           10,
		IPFile:                             "ip.txt",
		OutputFile:                         "result.csv",
		WriteOutput:                        true,
		ExportAppend:                       false,
		DisableDownload:                    true,
		TestAll:                            false,
		RetryMaxAttempts:                   0,
		RetryBackoffMS:                     0,
		CooldownFailures:                   3,
		CooldownMS:                         250,
		Debug:                              false,
		DebugCaptureAddress:                "",
	}
}

func defaultConfigSnapshot() map[string]any {
	return map[string]any{
		"cloudflare": map[string]any{
			"api_token":   "",
			"comment":     "",
			"proxied":     false,
			"record_name": "",
			"record_type": "A",
			"ttl":         defaultCloudflareTTL,
			"zone_id":     "",
		},
		"export": map[string]any{
			"file_name":          "result.csv",
			"file_name_template": "",
			"format":             "csv",
			"overwrite":          "replace_on_start",
			"target_dir":         "",
			"target_uri":         "",
		},
		"probe": map[string]any{
			"concurrency": map[string]any{
				"stage1": 200,
				"stage2": task.MaxTraceRoutines,
				"stage3": 1,
			},
			"cooldown_policy": map[string]any{
				"consecutive_failures": 3,
				"cooldown_ms":          250,
			},
			"debug":                                  false,
			"debug_capture_address":                  "",
			"disable_download":                       true,
			"download_count":                         10,
			"download_speed_sample_interval_seconds": 2,
			"download_time_seconds":                  10,
			"event_throttle_ms":                      100,
			"host_header":                            "",
			"httping":                                false,
			"httping_cf_colo":                        "",
			"httping_status_code":                    0,
			"max_loss_rate":                          float64(utils.MaxAllowedLossRate),
			"min_delay_ms":                           0,
			"ping_times":                             4,
			"print_num":                              10,
			"skip_first_latency_sample":              true,
			"retry_policy": map[string]any{
				"backoff_ms":   0,
				"max_attempts": 0,
			},
			"stage_limits": map[string]any{
				"stage1": 512,
				"stage2": 512,
				"stage3": 10,
			},
			"strategy": "fast",
			"sni":      "",
			"tcp_port": 443,
			"test_all": false,
			"thresholds": map[string]any{
				"max_http_latency_ms": nil,
				"max_tcp_latency_ms":  nil,
				"min_download_mbps":   0,
			},
			"timeouts": map[string]any{
				"stage1_ms": 1000,
				"stage2_ms": 1000,
				"stage3_ms": 10000,
			},
			"trace_url":  "",
			"url":        defaultFileTestURL,
			"user_agent": httpcfg.DefaultUserAgent,
		},
		"sources": []map[string]any{
			{
				"content":            "",
				"colo_filter":        "",
				"enabled":            true,
				"id":                 "source-1",
				"ip_limit":           defaultMobileSourceIPLimit,
				"ip_mode":            "traverse",
				"kind":               "url",
				"last_fetched_at":    "",
				"last_fetched_count": 0,
				"name":               "输入源 1",
				"path":               "",
				"status_text":        "",
				"url":                "",
			},
		},
	}
}

func configToProbeConfig(config map[string]any) (probeConfig, []string) {
	cfg := defaultProbeConfig()
	probe := mapValue(config["probe"])
	exportCfg := mapValue(config["export"])
	concurrency := mapValue(probe["concurrency"])
	stageLimits := mapValue(firstNonNil(probe["stage_limits"], probe["stageLimits"]))
	thresholds := mapValue(probe["thresholds"])
	timeouts := mapValue(probe["timeouts"])
	cooldownPolicy := mapValue(firstNonNil(probe["cooldown_policy"], probe["cooldownPolicy"]))
	retryPolicy := mapValue(firstNonNil(probe["retry_policy"], probe["retryPolicy"]))

	rawStrategy := strings.ToLower(strings.TrimSpace(stringValue(probe["strategy"], cfg.Strategy)))
	strategy := rawStrategy
	switch strategy {
	case "speed", "exhaustive", "full":
		strategy = "full"
	case "latency", "http-colo", "fast":
		strategy = "fast"
	default:
		strategy = "fast"
	}

	cfg.Strategy = strategy
	cfg.Routines = intValue(concurrency["stage1"], cfg.Routines)
	cfg.HeadRoutines = intValue(concurrency["stage2"], cfg.HeadRoutines)
	cfg.PingTimes = intValue(firstNonNil(probe["ping_times"], probe["pingTimes"]), cfg.PingTimes)
	cfg.SkipFirstLatency = boolValue(firstNonNil(probe["skip_first_latency_sample"], probe["skipFirstLatencySample"]), true)
	cfg.EventThrottleMS = intValue(firstNonNil(probe["event_throttle_ms"], probe["eventThrottleMs"]), cfg.EventThrottleMS)
	cfg.DownloadSpeedSampleIntervalSeconds = intValue(firstNonNil(probe["download_speed_sample_interval_seconds"], probe["downloadSpeedSampleIntervalSeconds"]), cfg.DownloadSpeedSampleIntervalSeconds)
	cfg.Stage1Limit = intValue(stageLimits["stage1"], cfg.Stage1Limit)
	cfg.HeadTestCount = intValue(stageLimits["stage2"], cfg.HeadTestCount)
	cfg.Stage3Limit = intValue(firstNonNil(stageLimits["stage3"], probe["stage3_limit"], probe["stage3Limit"], probe["download_count"], probe["downloadCount"]), cfg.Stage3Limit)
	cfg.TestCount = intValue(firstNonNil(probe["download_count"], probe["downloadCount"], cfg.Stage3Limit), cfg.TestCount)
	cfg.Stage3Concurrency = intValue(concurrency["stage3"], cfg.Stage3Concurrency)
	cfg.Stage1TimeoutMS = intValue(firstNonNil(timeouts["stage1_ms"], timeouts["stage1Ms"]), cfg.Stage1TimeoutMS)
	cfg.Stage2TimeoutMS = intValue(firstNonNil(timeouts["stage2_ms"], timeouts["stage2Ms"]), cfg.Stage2TimeoutMS)
	downloadTimeSeconds := intValue(firstNonNil(probe["download_time_seconds"], probe["downloadTimeSeconds"]), cfg.DownloadTimeSeconds)
	if downloadTimeSeconds <= 0 {
		cfg.DownloadTimeSeconds = intValue(timeouts["stage3_ms"], cfg.DownloadTimeSeconds*1000) / 1000
	} else {
		cfg.DownloadTimeSeconds = downloadTimeSeconds
	}
	cfg.TCPPort = intValue(firstNonNil(probe["tcp_port"], probe["tcpPort"]), cfg.TCPPort)
	cfg.URL = stringValue(probe["url"], cfg.URL)
	cfg.TraceURL = stringValue(firstNonNil(probe["trace_url"], probe["traceUrl"]), cfg.TraceURL)
	cfg.UserAgent = stringValue(firstNonNil(probe["user_agent"], probe["userAgent"]), cfg.UserAgent)
	cfg.HostHeader = stringValue(firstNonNil(probe["host_header"], probe["hostHeader"]), cfg.HostHeader)
	cfg.SNI = stringValue(probe["sni"], cfg.SNI)
	cfg.Httping = boolValue(probe["httping"], rawStrategy == "http-colo")
	cfg.HttpingStatusCode = intValue(firstNonNil(probe["httping_status_code"], probe["httpingStatusCode"]), cfg.HttpingStatusCode)
	cfg.HttpingCFColo = stringValue(firstNonNil(probe["httping_cf_colo"], probe["httpingCfColo"]), cfg.HttpingCFColo)
	cfg.MaxDelayMS = intValue(thresholds["max_tcp_latency_ms"], cfg.MaxDelayMS)
	cfg.HeadMaxDelayMS = intValue(thresholds["max_http_latency_ms"], cfg.HeadMaxDelayMS)
	cfg.MinDelayMS = intValue(firstNonNil(probe["min_delay_ms"], probe["minDelayMs"]), cfg.MinDelayMS)
	cfg.MaxLossRate = floatValue(firstNonNil(probe["max_loss_rate"], probe["maxLossRate"]), cfg.MaxLossRate)
	cfg.MinSpeedMB = floatValue(thresholds["min_download_mbps"], cfg.MinSpeedMB)
	cfg.PrintNum = intValue(firstNonNil(probe["print_num"], probe["printNum"]), cfg.PrintNum)
	cfg.DisableDownload = strategy == "fast"
	cfg.TestAll = false
	cfg.RetryMaxAttempts = intValue(firstNonNil(retryPolicy["max_attempts"], retryPolicy["maxAttempts"]), cfg.RetryMaxAttempts)
	cfg.RetryBackoffMS = intValue(firstNonNil(retryPolicy["backoff_ms"], retryPolicy["backoffMs"]), cfg.RetryBackoffMS)
	cfg.CooldownFailures = intValue(firstNonNil(cooldownPolicy["consecutive_failures"], cooldownPolicy["consecutiveFailures"]), cfg.CooldownFailures)
	cfg.CooldownMS = intValue(firstNonNil(cooldownPolicy["cooldown_ms"], cooldownPolicy["cooldownMs"]), cfg.CooldownMS)
	cfg.Debug = boolValue(probe["debug"], cfg.Debug)
	cfg.DebugCaptureAddress = stringValue(firstNonNil(probe["debug_capture_address"], probe["debugCaptureAddress"]), cfg.DebugCaptureAddress)

	if strategy == "fast" {
		cfg.MinSpeedMB = 0
	} else {
		cfg.DisableDownload = false
	}
	if fileName := mobileExportFileName(exportCfg, "", "", time.Now()); fileName != "" {
		cfg.OutputFile = mobileExportPath(exportCfg, fileName)
		cfg.WriteOutput = true
	}
	cfg.ExportAppend = strings.EqualFold(strings.TrimSpace(stringValue(exportCfg["overwrite"], "")), "append")
	return normalizeProbeConfig(cfg)
}

func (s *Service) applyExportConfig(cfg probeConfig, config map[string]any, taskID string) probeConfig {
	exportCfg := mapValue(config["export"])
	if len(exportCfg) == 0 {
		return cfg
	}
	if fileName := mobileExportFileName(exportCfg, taskID, s.activeProfileName(), time.Now()); fileName != "" {
		cfg.OutputFile = mobileExportPath(exportCfg, fileName)
		cfg.WriteOutput = true
	}
	return cfg
}

func mobileExportFileName(exportCfg map[string]any, taskID, profileName string, now time.Time) string {
	if template := strings.TrimSpace(stringValue(firstNonNil(exportCfg["file_name_template"], exportCfg["fileNameTemplate"]), "")); template != "" {
		if fileName := renderExportFileTemplate(template, taskID, profileName, now); fileName != "" {
			return fileName
		}
	}
	return sanitizeTemplateFileName(stringValue(firstNonNil(exportCfg["file_name"], exportCfg["fileName"]), ""))
}

func mobileExportPath(exportCfg map[string]any, fileName string) string {
	targetDir := strings.TrimSpace(stringValue(firstNonNil(exportCfg["target_dir"], exportCfg["targetDir"]), ""))
	if targetDir == "" {
		return fileName
	}
	return filepath.Join(targetDir, fileName)
}

func normalizeProbeConfig(cfg probeConfig) (probeConfig, []string) {
	def := defaultProbeConfig()
	warnings := make([]string, 0)
	warn := func(format string, args ...any) {
		warnings = append(warnings, fmt.Sprintf(format, args...))
	}
	if cfg.Strategy == "" && cfg.Routines == 0 && cfg.PingTimes == 0 && cfg.URL == "" {
		def.TraceURL, _ = deriveTraceURL(def.URL)
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
	} else if cfg.Routines > maxMobileTCPRoutines {
		warn("TCP并发线程最大支持 %d，已改为 %d。", maxMobileTCPRoutines, maxMobileTCPRoutines)
		cfg.Routines = maxMobileTCPRoutines
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
	if cfg.HeadTestCount <= 0 {
		warn("追踪候选上限必须大于 0，已改为 %d。", def.HeadTestCount)
		cfg.HeadTestCount = def.HeadTestCount
	}
	if cfg.Stage3Limit <= 0 {
		warn("阶段三候选上限必须大于 0，已改为 %d。", def.Stage3Limit)
		cfg.Stage3Limit = def.Stage3Limit
	}
	if cfg.Stage1Limit <= 0 {
		warn("阶段1候选上限必须大于 0，已改为 %d。", def.Stage1Limit)
		cfg.Stage1Limit = def.Stage1Limit
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
		warn("测速并发线程固定为 %d，已忽略配置值 %d。", maxMobileStage3Routines, cfg.Stage3Concurrency)
		cfg.Stage3Concurrency = def.Stage3Concurrency
	}
	if cfg.EventThrottleMS <= 0 {
		warn("事件节流必须大于 0，已改为 %dms。", def.EventThrottleMS)
		cfg.EventThrottleMS = def.EventThrottleMS
	}
	if cfg.DownloadSpeedSampleIntervalSeconds <= 0 {
		warn("下载速度采样间隔必须大于 0，已改为 %d 秒。", def.DownloadSpeedSampleIntervalSeconds)
		cfg.DownloadSpeedSampleIntervalSeconds = def.DownloadSpeedSampleIntervalSeconds
	}
	if cfg.DownloadTimeSeconds < 8 {
		warn("单 IP 下载测速时间必须至少为 8 秒，已改为 8 秒。")
		cfg.DownloadTimeSeconds = 8
	}
	if cfg.TCPPort <= 0 || cfg.TCPPort > 65535 {
		warn("测速端口必须在 1-65535 之间，已改为 %d。", def.TCPPort)
		cfg.TCPPort = def.TCPPort
	}
	if strings.TrimSpace(cfg.URL) == "" {
		warn("文件测速URL不能为空，已改为 %s。", def.URL)
		cfg.URL = def.URL
	}
	cfg.URL = normalizeProbeURLInput(cfg.URL)
	cfg.TraceURL = normalizeProbeURLInput(cfg.TraceURL)
	if cfg.TraceURL == "" {
		if derived, ok := deriveTraceURL(cfg.URL); ok {
			cfg.TraceURL = derived
		} else if derived, ok := deriveTraceURL(def.URL); ok {
			warn("追踪 URL 无法从文件测速URL派生，已改为 %s。", derived)
			cfg.TraceURL = derived
		}
	} else if !isValidProbeURL(cfg.TraceURL) {
		if derived, ok := deriveTraceURL(cfg.URL); ok {
			warn("追踪 URL 无效，已改为 %s。", derived)
			cfg.TraceURL = derived
		}
	}
	if (!cfg.DisableDownload || cfg.Strategy == "full") && isTraceProbeURL(cfg.URL) {
		warn("文件测速URL当前指向 /cdn-cgi/trace；完整模式建议填写真实文件 URL，追踪 URL 会单独用于追踪阶段。")
	}
	if strings.TrimSpace(cfg.UserAgent) == "" {
		warn("User-Agent 不能为空，已改为默认值。")
		cfg.UserAgent = def.UserAgent
	}
	if cfg.HttpingStatusCode > 0 && (cfg.HttpingStatusCode < 100 || cfg.HttpingStatusCode > 599) {
		warn("追踪有效状态码必须为 0 或 100-599，已改为 0。")
		cfg.HttpingStatusCode = def.HttpingStatusCode
	}
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
		warn("结果显示数量不能为负数，已改为 %d。", def.PrintNum)
		cfg.PrintNum = def.PrintNum
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
	cfg.HttpingCFColo = strings.TrimSpace(cfg.HttpingCFColo)
	cfg.IPFile = strings.TrimSpace(cfg.IPFile)
	cfg.OutputFile = strings.TrimSpace(cfg.OutputFile)
	cfg.DebugCaptureAddress = strings.TrimSpace(cfg.DebugCaptureAddress)
	return cfg, dedupeStrings(warnings)
}

func deriveTraceURL(rawURL string) (string, bool) {
	parsed, err := url.Parse(normalizeProbeURLInput(rawURL))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", false
	}
	parsed.Path = "/cdn-cgi/trace"
	parsed.RawPath = ""
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), true
}

func isValidProbeURL(rawURL string) bool {
	parsed, err := url.Parse(normalizeProbeURLInput(rawURL))
	return err == nil && parsed.Scheme != "" && parsed.Host != ""
}

func isTraceProbeURL(rawURL string) bool {
	parsed, err := url.Parse(normalizeProbeURLInput(rawURL))
	if err != nil {
		return false
	}
	return strings.EqualFold(strings.TrimRight(parsed.EscapedPath(), "/"), "/cdn-cgi/trace")
}

func normalizeProbeURLInput(rawURL string) string {
	value := strings.TrimSpace(rawURL)
	for strings.Contains(value, `\/`) {
		value = strings.ReplaceAll(value, `\/`, `/`)
	}
	return value
}

func (s *Service) applyProbeConfig(cfg probeConfig) {
	cfg.OutputFile = s.exportPath(cfg.OutputFile)
	task.Routines = cfg.Routines
	task.HeadRoutines = cfg.HeadRoutines
	task.HeadTestCount = cfg.HeadTestCount
	task.HeadMaxDelay = time.Duration(cfg.HeadMaxDelayMS) * time.Millisecond
	task.HeadTimeout = time.Duration(cfg.Stage2TimeoutMS) * time.Millisecond
	task.PingTimes = cfg.PingTimes
	task.SkipFirstLatencySample = cfg.SkipFirstLatency
	task.TCPConnectTimeout = time.Duration(cfg.Stage1TimeoutMS) * time.Millisecond
	task.TestCount = cfg.TestCount
	task.DownloadRoutines = cfg.Stage3Concurrency
	task.DownloadSpeedSampleInterval = time.Duration(cfg.DownloadSpeedSampleIntervalSeconds) * time.Second
	task.Timeout = time.Duration(cfg.DownloadTimeSeconds) * time.Second
	task.TCPPort = cfg.TCPPort
	task.URL = cfg.URL
	task.TraceURL = cfg.TraceURL
	task.UserAgent = cfg.UserAgent
	task.HostHeader = cfg.HostHeader
	task.SNI = cfg.SNI
	task.CaptureAddress = cfg.DebugCaptureAddress
	task.InsecureSkipVerify = true
	task.Httping = cfg.Httping
	task.HttpingStatusCode = cfg.HttpingStatusCode
	task.HttpingCFColo = cfg.HttpingCFColo
	task.HttpingCFColomap = task.MapColoMap()
	task.MinSpeed = cfg.MinSpeedMB
	task.Disable = cfg.DisableDownload
	task.TestAll = cfg.TestAll
	task.RetryMaxAttempts = cfg.RetryMaxAttempts
	task.RetryBackoff = time.Duration(cfg.RetryBackoffMS) * time.Millisecond
	task.CooldownConsecutiveFails = cfg.CooldownFailures
	task.CooldownDuration = time.Duration(cfg.CooldownMS) * time.Millisecond
	task.IPFile = cfg.IPFile
	task.IPText = cfg.IPText

	utils.InputMaxDelay = time.Duration(cfg.MaxDelayMS) * time.Millisecond
	utils.InputMinDelay = time.Duration(cfg.MinDelayMS) * time.Millisecond
	utils.InputMaxLossRate = float32(cfg.MaxLossRate)
	utils.PrintNum = cfg.PrintNum
	utils.Output = cfg.OutputFile
	utils.OutputAppend = cfg.ExportAppend
	utils.Debug = cfg.Debug
}
