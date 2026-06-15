package mobileapi

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/appcore"
	"github.com/axuitomo/CFST-GUI/internal/httpcfg"
	"github.com/axuitomo/CFST-GUI/internal/probecore"
	"github.com/axuitomo/CFST-GUI/internal/task"
	"github.com/axuitomo/CFST-GUI/internal/utils"
)

const (
	maxMobileTCPRoutines          = 1000
	maxMobileStage3Routines       = task.MaxDownloadRoutines
	defaultFileTestURL            = probecore.DefaultFileTestURL
	defaultMobileSourceIPLimit    = 500
	sourceColoFilterPhasePrecheck = probecore.SourceColoFilterPhasePrecheck
	sourceColoFilterPhaseStage2   = probecore.SourceColoFilterPhaseStage2
)

func (s *Service) LoadConfig() string {
	path := s.configPath()
	snapshot := defaultConfigSnapshot()
	warnings := make([]string, 0)
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			warnings = append(warnings, s.configureRuntimeLog(snapshot)...)
			sourceProfiles, sourceProfileErr := s.loadSourceProfileStoreForSnapshot(snapshot)
			if sourceProfileErr != nil {
				warnings = append(warnings, fmt.Sprintf("读取输入源配置档案失败：%v", sourceProfileErr))
			}
			return encodeCommand(commandResultFor("CONFIG_READY", map[string]any{
				"configPath":      path,
				"config_snapshot": snapshot,
				"source_profiles": sourceProfiles,
				"storage":         s.storageStatus(),
			}, "移动端配置文件尚未创建，已加载默认配置。", true, nil, warnings))
		}
		return encodeCommand(commandResultFor("CONFIG_READ_FAILED", nil, err.Error(), false, nil, nil))
	}

	var saved map[string]any
	compatInfo, err := appcore.UnmarshalJSONCompat(raw, &saved)
	if err != nil {
		return encodeCommand(commandResultFor("CONFIG_PARSE_FAILED", nil, err.Error(), false, nil, nil))
	}
	if compatInfo.IgnoredTrailingContent {
		warnings = append(warnings, "检测到移动端配置文件尾部存在残留内容，已自动忽略。建议重新保存配置。")
	}
	if value, ok := saved["config_snapshot"].(map[string]any); ok {
		snapshot = sanitizeMobileConfigSnapshot(value)
	} else {
		snapshot = sanitizeMobileConfigSnapshot(saved)
	}
	sourceProfiles, sourceProfileErr := s.loadSourceProfileStoreForSnapshot(snapshot)
	if sourceProfileErr != nil {
		warnings = append(warnings, fmt.Sprintf("读取输入源配置档案失败：%v", sourceProfileErr))
	}
	_, configWarnings := configToProbeConfig(snapshot)
	warnings = append(warnings, configWarnings...)
	warnings = append(warnings, s.configureRuntimeLog(snapshot)...)
	return encodeCommand(commandResultFor("CONFIG_READ_OK", map[string]any{
		"configPath":      path,
		"config_snapshot": snapshot,
		"source_profiles": sourceProfiles,
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
	snapshot = sanitizeMobileConfigSnapshot(snapshot)
	if err := s.writeConfigSnapshot(snapshot); err != nil {
		return encodeCommand(commandResultFor("CONFIG_WRITE_FAILED", nil, err.Error(), false, nil, nil))
	}
	_, warnings := configToProbeConfig(snapshot)
	warnings = append(warnings, s.configureRuntimeLog(snapshot)...)
	sourceProfiles, sourceProfileErr := s.loadSourceProfileStoreForSnapshot(snapshot)
	if sourceProfileErr != nil {
		warnings = append(warnings, fmt.Sprintf("读取输入源配置档案失败：%v", sourceProfileErr))
	}
	schedulerStatus := s.refreshSchedulerStatusForSnapshot(snapshot)
	return encodeCommand(commandResultFor("CONFIG_SAVE_OK", map[string]any{
		"configPath":       s.configPath(),
		"config_snapshot":  snapshot,
		"scheduler_status": schedulerStatus,
		"source_profiles":  sourceProfiles,
		"storage":          s.storageStatus(),
	}, "移动端配置已保存。", true, nil, warnings))
}

func defaultProbeConfig() probeConfig {
	return probecore.DefaultProbeConfig()
}

func defaultConfigSnapshot() map[string]any {
	return probecore.DefaultConfigSnapshot(mobileConfigSnapshotOptions())
}

func configToProbeConfig(config map[string]any) (probeConfig, []string) {
	return probecore.ConfigSnapshotToProbeConfig(config, mobileConfigSnapshotOptions())
}

func (s *Service) configureRuntimeLog(config map[string]any) []string {
	cfg, warnings := probecore.ConfigSnapshotToRuntimeLogConfig(config)
	if err := utils.ConfigureRuntimeLog(cfg.Enabled, s.logDirectoryPath(), cfg.Level, cfg.RetentionDays, cfg.Durability); err != nil {
		warnings = append(warnings, fmt.Sprintf("初始化运行日志失败：%v", err))
	}
	warnings = append(warnings, s.configureProcessMonitor(cfg.MonitorEnabled)...)
	return warnings
}

func probeDownloadSpeedSampleIntervalMS(probe map[string]any, fallback probeConfig) int {
	return probecore.ProbeDownloadSpeedSampleIntervalMS(probe, fallback)
}

func (s *Service) applyExportConfig(cfg probeConfig, config map[string]any, taskID string, profileName string) probeConfig {
	exportCfg := mapValue(config["export"])
	if len(exportCfg) == 0 {
		return cfg
	}
	if fileName := mobileExportFileName(exportCfg, taskID, profileName, time.Now()); fileName != "" {
		cfg.OutputFile = mobileExportPath(exportCfg, fileName)
		cfg.WriteOutput = true
	}
	return cfg
}

func mobileExportFileName(exportCfg map[string]any, taskID, profileName string, now time.Time) string {
	return probecore.ExportFileName(exportCfg, taskID, profileName, now)
}

func mobileExportPath(exportCfg map[string]any, fileName string) string {
	return probecore.ExportPath(exportCfg, fileName, "")
}

func normalizeProbeConfig(cfg probeConfig) (probeConfig, []string) {
	return probecore.NormalizeProbeConfig(cfg, probecore.ProbeConfigNormalizeOptions{
		MaxTCPRoutines:    maxMobileTCPRoutines,
		MaxStage3Routines: maxMobileStage3Routines,
	})
}

func deriveTraceURL(rawURL string) (string, bool) {
	return probecore.DeriveTraceURL(rawURL)
}

func isValidProbeURL(rawURL string) bool {
	return probecore.IsValidProbeURL(rawURL)
}

func isTraceProbeURL(rawURL string) bool {
	return probecore.IsTraceProbeURL(rawURL)
}

func normalizeProbeURLInput(rawURL string) string {
	return probecore.NormalizeProbeURLInput(rawURL)
}

func (s *Service) applyProbeConfig(cfg probeConfig) error {
	resolvedHttpingColos, err := appcore.ResolveConfiguredColos(s.coloDictionaryPaths(), cfg.HttpingCFColo, "第二阶段全局 COLO 筛选")
	if err != nil {
		return err
	}
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
	task.DownloadGetConcurrency = cfg.DownloadGetConcurrency
	task.DownloadBufferKB = cfg.DownloadBufferKB
	task.DownloadHTTPProtocol = cfg.DownloadHTTPProtocol
	task.DownloadSpeedSampleInterval = time.Duration(cfg.DownloadSpeedSampleIntervalMS) * time.Millisecond
	task.Timeout = time.Duration(cfg.DownloadTimeSeconds) * time.Second
	task.DownloadWarmupDuration = time.Duration(cfg.DownloadWarmupSeconds) * time.Second
	task.TCPPort = cfg.TCPPort
	task.URL = cfg.URL
	task.TraceURL = cfg.TraceURL
	task.TraceColoMode = cfg.TraceColoMode
	task.ColoDictionaryPath = s.coloDictionaryPaths().Colo
	task.UserAgent = cfg.UserAgent
	task.HostHeader = cfg.HostHeader
	task.SNI = cfg.SNI
	task.RequestHeaders = cfg.RequestHeaders
	task.CaptureAddress = effectiveDebugCaptureAddress(cfg)
	task.InsecureSkipVerify = true
	task.Httping = cfg.Httping
	task.HttpingStatusCode = cfg.HttpingStatusCode
	task.HttpingCFColo = cfg.HttpingCFColo
	task.HttpingCFColoMode = cfg.HttpingCFColoMode
	task.HttpingCFColomap = task.MapColoSet(resolvedHttpingColos)
	task.MinSpeed = cfg.MinSpeedMB
	task.MinSpeedMetric = cfg.DownloadSpeedMetric
	task.Disable = cfg.DisableDownload
	task.TestAll = cfg.TestAll
	task.RetryMaxAttempts = cfg.RetryMaxAttempts
	task.RetryBackoff = time.Duration(cfg.RetryBackoffMS) * time.Millisecond
	task.CooldownConsecutiveFails = cfg.CooldownFailures
	task.CooldownDuration = time.Duration(cfg.CooldownMS) * time.Millisecond
	task.ResetStageCooldownCounters()
	task.IPFile = cfg.IPFile
	task.IPText = cfg.IPText

	utils.InputMaxDelay = time.Duration(cfg.MaxDelayMS) * time.Millisecond
	utils.InputMinDelay = time.Duration(cfg.MinDelayMS) * time.Millisecond
	utils.InputMaxLossRate = float32(cfg.MaxLossRate)
	utils.PrintNum = cfg.PrintNum
	utils.Output = cfg.OutputFile
	utils.OutputAppend = cfg.ExportAppend
	utils.OutputCSVEncoding = cfg.CSVEncoding
	utils.Debug = cfg.Debug
	return nil
}

func effectiveDebugCaptureAddress(cfg probeConfig) string {
	if !cfg.Debug || !cfg.DebugCaptureEnabled || strings.TrimSpace(cfg.DebugCaptureAddress) == "" {
		return ""
	}
	return httpcfg.Resolve("", "", "", cfg.DebugCaptureAddress, true).CaptureAddress
}
