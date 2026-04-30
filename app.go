package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/XIU2/CloudflareSpeedTest/internal/httpcfg"
	"github.com/XIU2/CloudflareSpeedTest/task"
	"github.com/XIU2/CloudflareSpeedTest/utils"
)

const guiSchemaVersion = "cfst-gui-wails-v1"

type App struct {
	ctx context.Context
	mu  sync.Mutex
}

type ProbeConfig struct {
	Strategy            string  `json:"strategy"`
	Routines            int     `json:"routines"`
	PingTimes           int     `json:"pingTimes"`
	SkipFirstLatency    bool    `json:"skipFirstLatencySample"`
	EventThrottleMS     int     `json:"eventThrottleMs"`
	TestCount           int     `json:"testCount"`
	DownloadTimeSeconds int     `json:"downloadTimeSeconds"`
	TCPPort             int     `json:"tcpPort"`
	URL                 string  `json:"url"`
	UserAgent           string  `json:"userAgent"`
	HostHeader          string  `json:"hostHeader"`
	SNI                 string  `json:"sni"`
	Httping             bool    `json:"httping"`
	HttpingStatusCode   int     `json:"httpingStatusCode"`
	HttpingCFColo       string  `json:"httpingCFColo"`
	MaxDelayMS          int     `json:"maxDelayMS"`
	MinDelayMS          int     `json:"minDelayMS"`
	MaxLossRate         float64 `json:"maxLossRate"`
	MinSpeedMB          float64 `json:"minSpeedMB"`
	PrintNum            int     `json:"printNum"`
	IPFile              string  `json:"ipFile"`
	IPText              string  `json:"ipText"`
	OutputFile          string  `json:"outputFile"`
	WriteOutput         bool    `json:"writeOutput"`
	DisableDownload     bool    `json:"disableDownload"`
	TestAll             bool    `json:"testAll"`
	Debug               bool    `json:"debug"`
	DebugCaptureAddress string  `json:"debugCaptureAddress"`
}

type ConfigSnapshot struct {
	Probe         ProbeConfig `json:"probe"`
	SourceText    string      `json:"sourceText"`
	SavedAt       string      `json:"savedAt"`
	SchemaVersion string      `json:"schemaVersion"`
}

type ConfigCommandResult struct {
	ConfigPath     string         `json:"configPath"`
	ConfigSnapshot ConfigSnapshot `json:"configSnapshot"`
	Message        string         `json:"message"`
	Ready          bool           `json:"ready"`
	Warnings       []string       `json:"warnings"`
}

type DesktopCommandResult struct {
	Code          string      `json:"code"`
	Data          interface{} `json:"data"`
	Message       string      `json:"message"`
	OK            bool        `json:"ok"`
	SchemaVersion string      `json:"schema_version"`
	TaskID        *string     `json:"task_id"`
	Warnings      []string    `json:"warnings"`
}

type HealthResult struct {
	ConfigPath     string `json:"configPath"`
	Online         bool   `json:"online"`
	SchemaVersion  string `json:"schemaVersion"`
	Service        string `json:"service"`
	Version        string `json:"version"`
	WailsTransport string `json:"wailsTransport"`
}

type SourceSummary struct {
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

type ProbeRequest struct {
	Config     ProbeConfig `json:"config"`
	SourceText string      `json:"sourceText"`
}

type DesktopProbePayload struct {
	Config  map[string]interface{} `json:"config"`
	Sources []DesktopSource        `json:"sources"`
	TaskID  string                 `json:"task_id"`
}

type DesktopSource struct {
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

type DesktopSourceStatus struct {
	ID               string `json:"id"`
	LastFetchedAt    string `json:"last_fetched_at"`
	LastFetchedCount int    `json:"last_fetched_count"`
	StatusText       string `json:"status_text"`
}

type preparedDesktopSources struct {
	Text           string
	InvalidCount   int
	SourceStatuses []DesktopSourceStatus
	Warnings       []string
}

type ProbeRunResult struct {
	Config         ProbeConfig           `json:"config"`
	DurationMS     int64                 `json:"durationMs"`
	OutputFile     string                `json:"outputFile"`
	Results        []ProbeRow            `json:"results"`
	Source         SourceSummary         `json:"source"`
	SourceStatuses []DesktopSourceStatus `json:"sourceStatuses"`
	StartedAt      string                `json:"startedAt"`
	Summary        ProbeSummary          `json:"summary"`
	Warnings       []string              `json:"warnings"`
	SchemaVersion  string                `json:"schemaVersion"`
}

type ProbeSummary struct {
	AverageDelayMS float64 `json:"averageDelayMs"`
	BestIP         string  `json:"bestIp"`
	BestSpeedMB    float64 `json:"bestSpeedMb"`
	Failed         int     `json:"failed"`
	Passed         int     `json:"passed"`
	Total          int     `json:"total"`
}

type ProbeRow struct {
	Colo            string  `json:"colo"`
	DelayMS         float64 `json:"delayMs"`
	DownloadSpeedMB float64 `json:"downloadSpeedMb"`
	IP              string  `json:"ip"`
	LossRate        float64 `json:"lossRate"`
	Received        int     `json:"received"`
	Sended          int     `json:"sended"`
}

type StrategyPreset struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Config      ProbeConfig `json:"config"`
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) GetHealth() HealthResult {
	return HealthResult{
		ConfigPath:     configFilePath(),
		Online:         true,
		SchemaVersion:  guiSchemaVersion,
		Service:        "CFST Wails Bridge",
		Version:        version,
		WailsTransport: "window.go.main.App",
	}
}

func (a *App) GetDefaultConfig() ProbeConfig {
	return defaultProbeConfig()
}

func (a *App) GetStrategyPresets() []StrategyPreset {
	base := defaultProbeConfig()
	full := base
	full.Strategy = "full"
	full.DisableDownload = false
	full.TestCount = 10
	full.MinSpeedMB = 0

	return []StrategyPreset{
		{
			ID:          base.Strategy,
			Name:        "极速模式",
			Description: "仅执行 TCP/HTTP 响应测速，跳过下载环节，适合快速更新日常节点。",
			Config:      base,
		},
		{
			ID:          full.Strategy,
			Name:        "完整模式",
			Description: "在低延迟筛选基础上追加真实下载测速，更适合大流量业务和流媒体代理。",
			Config:      full,
		},
	}
}

func (a *App) LoadDesktopConfig() DesktopCommandResult {
	path := desktopConfigFilePath()
	snapshot := defaultDesktopConfigSnapshot()

	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return desktopCommandResult("CONFIG_READY", map[string]interface{}{
				"configPath":      path,
				"config_snapshot": snapshot,
			}, "配置文件尚未创建，已加载默认桌面配置。", true, nil, nil)
		}
		return desktopCommandResult("CONFIG_READ_FAILED", nil, err.Error(), false, nil, nil)
	}

	var saved map[string]interface{}
	if err := json.Unmarshal(raw, &saved); err != nil {
		return desktopCommandResult("CONFIG_PARSE_FAILED", nil, err.Error(), false, nil, nil)
	}
	if value, ok := saved["config_snapshot"].(map[string]interface{}); ok {
		snapshot = value
	} else {
		snapshot = saved
	}

	return desktopCommandResult("CONFIG_READ_OK", map[string]interface{}{
		"configPath":      path,
		"config_snapshot": snapshot,
	}, "配置已加载。", true, nil, nil)
}

func (a *App) SaveDesktopConfig(payload map[string]interface{}) DesktopCommandResult {
	path := desktopConfigFilePath()
	snapshot, ok := payload["config_snapshot"].(map[string]interface{})
	if !ok {
		return desktopCommandResult("CONFIG_INVALID", nil, "缺少 config_snapshot。", false, nil, nil)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return desktopCommandResult("CONFIG_WRITE_FAILED", nil, err.Error(), false, nil, nil)
	}

	body := map[string]interface{}{
		"config_snapshot": snapshot,
		"saved_at":        time.Now().Format(time.RFC3339),
		"schema_version":  guiSchemaVersion,
	}
	raw, err := json.MarshalIndent(body, "", "  ")
	if err != nil {
		return desktopCommandResult("CONFIG_SERIALIZE_FAILED", nil, err.Error(), false, nil, nil)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		return desktopCommandResult("CONFIG_WRITE_FAILED", nil, err.Error(), false, nil, nil)
	}

	return desktopCommandResult("CONFIG_SAVE_OK", map[string]interface{}{
		"configPath":      path,
		"config_snapshot": snapshot,
	}, "配置已保存到本机。", true, nil, nil)
}

func (a *App) RunDesktopProbe(payload DesktopProbePayload) (ProbeRunResult, error) {
	cfg := desktopConfigToProbeConfig(payload.Config)
	taskID := strings.TrimSpace(payload.TaskID)
	if taskID == "" {
		taskID = fmt.Sprintf("cfst-%d", time.Now().UnixNano())
	}
	emitter := newDesktopProbeEmitter(a, taskID, time.Duration(cfg.EventThrottleMS)*time.Millisecond)
	prepared := prepareDesktopSources(cfg, payload.Sources)
	if err := persistDesktopSourceStatuses(prepared.SourceStatuses); err != nil {
		prepared.Warnings = append(prepared.Warnings, fmt.Sprintf("更新输入源状态失败：%v", err))
	}
	preparedSummary := summarizeSource(prepared.Text)
	preparedInvalidCount := preparedSummary.InvalidCount + prepared.InvalidCount
	emitter.emit("probe.preprocessed", map[string]interface{}{
		"accepted":        preparedSummary.ValidCount,
		"filtered":        preparedSummary.DuplicateCount,
		"invalid":         preparedInvalidCount,
		"source_statuses": prepared.SourceStatuses,
		"total":           preparedSummary.ValidCount,
	})
	if strings.TrimSpace(prepared.Text) == "" && len(prepared.Warnings) > 0 {
		err := errors.New(strings.Join(prepared.Warnings, "；"))
		emitter.emit("probe.failed", map[string]interface{}{
			"message":     err.Error(),
			"recoverable": false,
		})
		return ProbeRunResult{}, err
	}
	result, err := a.runProbe(ProbeRequest{
		Config:     cfg,
		SourceText: prepared.Text,
	}, emitter)
	if err != nil {
		emitter.emit("probe.failed", map[string]interface{}{
			"message":     err.Error(),
			"recoverable": false,
		})
		return ProbeRunResult{}, err
	}
	result.SourceStatuses = prepared.SourceStatuses
	result.Warnings = append(result.Warnings, prepared.Warnings...)
	exportedCount := 0
	if strings.TrimSpace(result.OutputFile) != "" && len(result.Results) > 0 {
		exportedCount = len(result.Results)
	}
	emitter.emit("probe.completed", map[string]interface{}{
		"exported": exportedCount,
		"failed":   result.Summary.Failed,
		"failure_summary": map[string]interface{}{
			"duplicate_count": preparedSummary.DuplicateCount,
			"invalid_count":   preparedInvalidCount,
		},
		"passed":       result.Summary.Passed,
		"result_count": len(result.Results),
		"target_path":  result.OutputFile,
	})
	return result, nil
}

func (a *App) OpenPath(targetPath string) error {
	targetPath = strings.TrimSpace(targetPath)
	if targetPath == "" {
		return nil
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", targetPath)
	case "darwin":
		cmd = exec.Command("open", targetPath)
	default:
		cmd = exec.Command("xdg-open", targetPath)
	}
	return cmd.Start()
}

func (a *App) LoadConfig() (ConfigCommandResult, error) {
	path := configFilePath()
	snapshot := ConfigSnapshot{
		Probe:         defaultProbeConfig(),
		SchemaVersion: guiSchemaVersion,
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ConfigCommandResult{
				ConfigPath:     path,
				ConfigSnapshot: snapshot,
				Message:        "配置文件尚未创建，已加载默认测速策略。",
				Ready:          true,
			}, nil
		}
		return ConfigCommandResult{}, err
	}

	if err := json.Unmarshal(raw, &snapshot); err != nil {
		return ConfigCommandResult{}, err
	}

	snapshot.Probe = normalizeProbeConfig(snapshot.Probe)
	if snapshot.SchemaVersion == "" {
		snapshot.SchemaVersion = guiSchemaVersion
	}

	return ConfigCommandResult{
		ConfigPath:     path,
		ConfigSnapshot: snapshot,
		Message:        "配置已加载。",
		Ready:          true,
	}, nil
}

func (a *App) SaveConfig(snapshot ConfigSnapshot) (ConfigCommandResult, error) {
	snapshot.Probe = normalizeProbeConfig(snapshot.Probe)
	snapshot.SavedAt = time.Now().Format(time.RFC3339)
	snapshot.SchemaVersion = guiSchemaVersion

	path := configFilePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return ConfigCommandResult{}, err
	}

	raw, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return ConfigCommandResult{}, err
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		return ConfigCommandResult{}, err
	}

	return ConfigCommandResult{
		ConfigPath:     path,
		ConfigSnapshot: snapshot,
		Message:        "配置已保存到本机。",
		Ready:          true,
	}, nil
}

func (a *App) ValidateSources(raw string) SourceSummary {
	return summarizeSource(raw)
}

func (a *App) RunProbe(req ProbeRequest) (ProbeRunResult, error) {
	return a.runProbe(req, nil)
}

func (a *App) runProbe(req ProbeRequest, emitter *desktopProbeEmitter) (ProbeRunResult, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	start := time.Now()
	cfg := normalizeProbeConfig(req.Config)
	_, source, err := resolveProbeSource(cfg, req.SourceText)
	if err != nil {
		return ProbeRunResult{}, err
	}
	if source.ValidCount == 0 {
		return ProbeRunResult{}, errors.New("没有可用的 IP/CIDR 输入")
	}

	cfg.IPText = strings.Join(source.Valid, ",")
	applyProbeConfig(cfg)
	closeDebugLog, debugWarnings := configureProbeDebugRuntime(cfg)
	defer closeDebugLog()
	task.InitRandSeed()

	totalWork := source.ValidCount
	task.LatencyProgressHook = func(processed, passed, failed, _ int) {
		if emitter == nil {
			return
		}
		emitter.emitProgress("latency", processed, passed, failed, totalWork)
	}
	task.DownloadProgressHook = nil
	defer func() {
		task.LatencyProgressHook = nil
		task.DownloadProgressHook = nil
	}()

	if emitter != nil {
		emitter.emitProgress("latency", 0, 0, 0, totalWork)
	}

	pingData := task.NewPing().Run().FilterDelay().FilterLossRate()
	downloadTotal := 0
	if !cfg.DisableDownload {
		downloadTotal = estimateDownloadProbeCount(len(pingData))
		if downloadTotal > 0 {
			totalWork += downloadTotal
			task.DownloadProgressHook = func(processed, qualified, _ int) {
				if emitter == nil {
					return
				}
				emitter.emitProgress("download", source.ValidCount+processed, qualified, processed-qualified, totalWork)
			}
			if emitter != nil {
				emitter.emitProgress("download", source.ValidCount, 0, 0, totalWork)
			}
		}
	}
	speedData := task.TestDownloadSpeed(pingData)
	warnings := append(buildProbeWarnings(source), debugWarnings...)
	resultData := []utils.CloudflareIPData(speedData)
	if len(resultData) == 0 && len(pingData) > 0 && !cfg.DisableDownload {
		// 下载阈值过严时，回退展示延迟通过的候选，避免整轮测速后结果面板为空。
		resultData = []utils.CloudflareIPData(pingData)
		warnings = append(warnings, fmt.Sprintf("下载测速未命中最低下载速度阈值 %.2f MB/s，已回退展示延迟通过的候选节点。", cfg.MinSpeedMB))
	}

	outputFile := ""
	if len(resultData) > 0 {
		outputFile = currentOutputFile(cfg)
		if outputFile != "" {
			if err := utils.ExportCsv(resultData); err != nil {
				warnings = append(warnings, fmt.Sprintf("结果导出失败：%v", err))
				outputFile = ""
			} else if emitter != nil {
				emitter.emit("probe.partial_export", map[string]interface{}{
					"target_path": outputFile,
					"written":     len(resultData),
				})
			}
		}
	}

	rows := make([]ProbeRow, 0, len(resultData))
	for _, item := range resultData {
		rows = append(rows, convertProbeRow(item))
	}

	return ProbeRunResult{
		Config:        cfg,
		DurationMS:    time.Since(start).Milliseconds(),
		OutputFile:    outputFile,
		Results:       rows,
		Source:        source,
		StartedAt:     start.Format(time.RFC3339),
		Summary:       summarizeProbeRows(rows, source.CandidateCount),
		Warnings:      dedupeStrings(warnings),
		SchemaVersion: guiSchemaVersion,
	}, nil
}

func defaultProbeConfig() ProbeConfig {
	return ProbeConfig{
		Strategy:            "fast",
		Routines:            200,
		PingTimes:           4,
		SkipFirstLatency:    true,
		EventThrottleMS:     100,
		TestCount:           10,
		DownloadTimeSeconds: 10,
		TCPPort:             443,
		URL:                 "https://cf.xiu2.xyz/url",
		UserAgent:           httpcfg.DefaultUserAgent,
		HostHeader:          "",
		SNI:                 "",
		Httping:             false,
		HttpingStatusCode:   0,
		HttpingCFColo:       "",
		MaxDelayMS:          9999,
		MinDelayMS:          0,
		MaxLossRate:         1,
		MinSpeedMB:          0,
		PrintNum:            10,
		IPFile:              "ip.txt",
		OutputFile:          "result.csv",
		WriteOutput:         true,
		DisableDownload:     true,
		TestAll:             false,
		Debug:               false,
		DebugCaptureAddress: "",
	}
}

func normalizeProbeConfig(cfg ProbeConfig) ProbeConfig {
	def := defaultProbeConfig()
	if cfg.Strategy == "" && cfg.Routines == 0 && cfg.PingTimes == 0 && cfg.URL == "" {
		return def
	}
	if cfg.Strategy == "" {
		cfg.Strategy = def.Strategy
	}
	if cfg.Routines <= 0 {
		cfg.Routines = def.Routines
	}
	if cfg.PingTimes <= 0 {
		cfg.PingTimes = def.PingTimes
	}
	if !cfg.SkipFirstLatency {
		cfg.SkipFirstLatency = def.SkipFirstLatency
	}
	if cfg.TestCount <= 0 {
		cfg.TestCount = def.TestCount
	}
	if cfg.EventThrottleMS <= 0 {
		cfg.EventThrottleMS = def.EventThrottleMS
	}
	if cfg.DownloadTimeSeconds <= 0 {
		cfg.DownloadTimeSeconds = def.DownloadTimeSeconds
	}
	if cfg.TCPPort <= 0 || cfg.TCPPort >= 65535 {
		cfg.TCPPort = def.TCPPort
	}
	if strings.TrimSpace(cfg.URL) == "" {
		cfg.URL = def.URL
	}
	if strings.TrimSpace(cfg.UserAgent) == "" {
		cfg.UserAgent = def.UserAgent
	}
	if cfg.MaxDelayMS <= 0 {
		cfg.MaxDelayMS = def.MaxDelayMS
	}
	if cfg.MinDelayMS < 0 {
		cfg.MinDelayMS = def.MinDelayMS
	}
	if cfg.MaxLossRate < 0 || cfg.MaxLossRate > 1 {
		cfg.MaxLossRate = def.MaxLossRate
	}
	if cfg.MinSpeedMB < 0 {
		cfg.MinSpeedMB = def.MinSpeedMB
	}
	if cfg.PrintNum < 0 {
		cfg.PrintNum = def.PrintNum
	}
	if strings.TrimSpace(cfg.IPFile) == "" {
		cfg.IPFile = def.IPFile
	}
	if cfg.WriteOutput && strings.TrimSpace(cfg.OutputFile) == "" {
		cfg.OutputFile = def.OutputFile
	}
	cfg.URL = strings.TrimSpace(cfg.URL)
	cfg.UserAgent = strings.TrimSpace(cfg.UserAgent)
	cfg.HostHeader = strings.TrimSpace(cfg.HostHeader)
	cfg.SNI = strings.TrimSpace(cfg.SNI)
	cfg.HttpingCFColo = strings.TrimSpace(cfg.HttpingCFColo)
	cfg.IPFile = strings.TrimSpace(cfg.IPFile)
	cfg.OutputFile = strings.TrimSpace(cfg.OutputFile)
	cfg.DebugCaptureAddress = strings.TrimSpace(cfg.DebugCaptureAddress)
	return cfg
}

func applyProbeConfig(cfg ProbeConfig) {
	task.Routines = cfg.Routines
	task.PingTimes = cfg.PingTimes
	task.SkipFirstLatencySample = cfg.SkipFirstLatency
	task.TestCount = cfg.TestCount
	task.Timeout = time.Duration(cfg.DownloadTimeSeconds) * time.Second
	task.TCPPort = cfg.TCPPort
	task.URL = cfg.URL
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
	task.IPFile = cfg.IPFile
	task.IPText = cfg.IPText

	utils.InputMaxDelay = time.Duration(cfg.MaxDelayMS) * time.Millisecond
	utils.InputMinDelay = time.Duration(cfg.MinDelayMS) * time.Millisecond
	utils.InputMaxLossRate = float32(cfg.MaxLossRate)
	utils.PrintNum = cfg.PrintNum
	utils.Output = currentOutputFile(cfg)
	utils.Debug = cfg.Debug
}

func configureProbeDebugRuntime(cfg ProbeConfig) (func(), []string) {
	path, err := utils.ConfigureDebugLog(cfg.Debug, debugLogFilePath())
	if err != nil {
		return func() {}, []string{fmt.Sprintf("初始化调试日志失败：%v", err)}
	}

	warnings := make([]string, 0, 2)
	if cfg.Debug && path != "" {
		warnings = append(warnings, fmt.Sprintf("调试日志已写入 %s", path))
	}
	if cfg.Debug && strings.TrimSpace(cfg.DebugCaptureAddress) != "" {
		captureAddress := httpcfg.Resolve("", "", "", cfg.DebugCaptureAddress, true).CaptureAddress
		warnings = append(warnings, fmt.Sprintf("调试模式已将请求拨号目标覆盖为 %s", captureAddress))
	}

	return func() {
		_ = utils.CloseDebugLog()
	}, warnings
}

func currentOutputFile(cfg ProbeConfig) string {
	if !cfg.WriteOutput {
		return ""
	}
	return cfg.OutputFile
}

func resolveProbeSource(cfg ProbeConfig, raw string) (string, SourceSummary, error) {
	sourceText := strings.TrimSpace(raw)
	if sourceText == "" && strings.TrimSpace(cfg.IPText) != "" {
		sourceText = cfg.IPText
	}
	if sourceText == "" {
		path := cfg.IPFile
		fileRaw, err := os.ReadFile(path)
		if err != nil {
			return "", SourceSummary{}, fmt.Errorf("读取 IP 数据文件失败：%w", err)
		}
		sourceText = string(fileRaw)
	}

	summary := summarizeSource(sourceText)
	return sourceText, summary, nil
}

func summarizeSource(raw string) SourceSummary {
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	summary := SourceSummary{RawLineCount: len(lines)}
	seen := map[string]struct{}{}

	for _, token := range sourceTokens(raw) {
		summary.CandidateCount++
		normalized, ok := normalizeIPToken(token)
		if !ok {
			summary.Invalid = append(summary.Invalid, token)
			continue
		}
		if _, exists := seen[normalized]; exists {
			summary.Duplicates = append(summary.Duplicates, normalized)
			continue
		}
		seen[normalized] = struct{}{}
		summary.Valid = append(summary.Valid, normalized)
	}

	summary.ValidCount = len(summary.Valid)
	summary.InvalidCount = len(summary.Invalid)
	summary.DuplicateCount = len(summary.Duplicates)
	summary.UniqueCount = summary.ValidCount
	return summary
}

func sourceTokens(raw string) []string {
	tokens := make([]string, 0)
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	for _, line := range lines {
		if idx := strings.IndexByte(line, '#'); idx >= 0 {
			line = line[:idx]
		}
		parts := strings.FieldsFunc(line, func(r rune) bool {
			return r == ',' || r == ';' || r == '\t' || r == ' ' || r == '\n'
		})
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part != "" {
				tokens = append(tokens, part)
			}
		}
	}
	return tokens
}

func normalizeIPToken(token string) (string, bool) {
	token = strings.TrimSpace(token)
	if token == "" {
		return "", false
	}
	if strings.Contains(token, "/") {
		ip, ipNet, err := net.ParseCIDR(token)
		if err != nil {
			return "", false
		}
		return ip.String() + "/" + maskSize(ipNet), true
	}
	ip := net.ParseIP(token)
	if ip == nil {
		return "", false
	}
	return ip.String(), true
}

func maskSize(ipNet *net.IPNet) string {
	ones, _ := ipNet.Mask.Size()
	return fmt.Sprintf("%d", ones)
}

func convertProbeRow(item utils.CloudflareIPData) ProbeRow {
	lossRate := 0.0
	if item.Sended > 0 {
		lossRate = float64(item.Sended-item.Received) / float64(item.Sended)
	}
	colo := item.Colo
	if colo == "" {
		colo = "N/A"
	}
	return ProbeRow{
		Colo:            colo,
		DelayMS:         item.Delay.Seconds() * 1000,
		DownloadSpeedMB: item.DownloadSpeed / 1024 / 1024,
		IP:              item.IP.String(),
		LossRate:        lossRate,
		Received:        item.Received,
		Sended:          item.Sended,
	}
}

func summarizeProbeRows(rows []ProbeRow, total int) ProbeSummary {
	summary := ProbeSummary{
		Failed: total - len(rows),
		Passed: len(rows),
		Total:  total,
	}
	if summary.Failed < 0 {
		summary.Failed = 0
	}
	if len(rows) == 0 {
		return summary
	}
	var delay float64
	for _, row := range rows {
		delay += row.DelayMS
	}
	summary.AverageDelayMS = delay / float64(len(rows))
	summary.BestIP = rows[0].IP
	summary.BestSpeedMB = rows[0].DownloadSpeedMB
	return summary
}

func estimateDownloadProbeCount(candidateCount int) int {
	if task.Disable || candidateCount <= 0 {
		return 0
	}
	if candidateCount < task.TestCount || task.MinSpeed > 0 {
		return candidateCount
	}
	return task.TestCount
}

func buildProbeWarnings(source SourceSummary) []string {
	warnings := make([]string, 0)
	if source.InvalidCount > 0 {
		warnings = append(warnings, fmt.Sprintf("已忽略 %d 条非法 IP/CIDR。", source.InvalidCount))
	}
	if source.DuplicateCount > 0 {
		warnings = append(warnings, fmt.Sprintf("已忽略 %d 条重复候选。", source.DuplicateCount))
	}
	return warnings
}

func desktopCommandResult(code string, data interface{}, message string, ok bool, taskID *string, warnings []string) DesktopCommandResult {
	if warnings == nil {
		warnings = []string{}
	}
	return DesktopCommandResult{
		Code:          code,
		Data:          data,
		Message:       message,
		OK:            ok,
		SchemaVersion: guiSchemaVersion,
		TaskID:        taskID,
		Warnings:      warnings,
	}
}

func defaultDesktopConfigSnapshot() map[string]interface{} {
	return map[string]interface{}{
		"cloudflare": map[string]interface{}{
			"api_token":   "",
			"comment":     "",
			"proxied":     false,
			"record_name": "",
			"record_type": "A",
			"ttl":         1,
			"zone_id":     "",
		},
		"export": map[string]interface{}{
			"file_name":  "result.csv",
			"format":     "csv",
			"overwrite":  "replace_on_start",
			"target_dir": "",
		},
		"probe": map[string]interface{}{
			"concurrency": map[string]interface{}{
				"stage1": 200,
				"stage2": 10,
				"stage3": 1,
			},
			"cooldown_policy": map[string]interface{}{
				"consecutive_failures": 3,
				"cooldown_ms":          250,
			},
			"debug":                 false,
			"debug_capture_address": "",
			"disable_download":      true,
			"download_count":        10,
			"download_time_seconds": 10,
			"event_throttle_ms":     100,
			"httping":               false,
			"httping_cf_colo":       "",
			"httping_status_code":   0,
			"max_loss_rate":         1,
			"min_delay_ms":          0,
			"ping_times":            4,
			"print_num":             10,
			"retry_policy": map[string]interface{}{
				"backoff_ms":   0,
				"max_attempts": 0,
			},
			"skip_first_latency_sample": true,
			"stage_limits": map[string]interface{}{
				"stage1": 512,
				"stage2": 64,
				"stage3": 10,
			},
			"strategy":    "fast",
			"host_header": "",
			"sni":         "",
			"tcp_port":    443,
			"test_all":    false,
			"thresholds": map[string]interface{}{
				"max_http_latency_ms": nil,
				"max_tcp_latency_ms":  nil,
				"min_download_mbps":   0,
			},
			"timeouts": map[string]interface{}{
				"stage1_ms": 1000,
				"stage2_ms": 1000,
				"stage3_ms": 10000,
			},
			"url":        "https://cf.xiu2.xyz/url",
			"user_agent": httpcfg.DefaultUserAgent,
		},
		"sources": []map[string]interface{}{
			{
				"content":            "",
				"enabled":            true,
				"id":                 "source-1",
				"ip_limit":           3000,
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

func desktopConfigToProbeConfig(config map[string]interface{}) ProbeConfig {
	cfg := defaultProbeConfig()
	probe := mapValue(config["probe"])
	exportCfg := mapValue(config["export"])
	concurrency := mapValue(probe["concurrency"])
	stageLimits := mapValue(probe["stage_limits"])
	thresholds := mapValue(probe["thresholds"])
	timeouts := mapValue(probe["timeouts"])
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
	cfg.PingTimes = intValue(firstNonNil(probe["ping_times"], probe["pingTimes"]), cfg.PingTimes)
	cfg.SkipFirstLatency = boolValue(firstNonNil(probe["skip_first_latency_sample"], probe["skipFirstLatencySample"]), true)
	cfg.EventThrottleMS = intValue(firstNonNil(probe["event_throttle_ms"], probe["eventThrottleMs"]), cfg.EventThrottleMS)
	cfg.TestCount = intValue(firstNonNil(probe["download_count"], probe["downloadCount"], stageLimits["stage3"]), cfg.TestCount)
	downloadTimeSeconds := intValue(firstNonNil(probe["download_time_seconds"], probe["downloadTimeSeconds"]), cfg.DownloadTimeSeconds)
	if downloadTimeSeconds <= 0 {
		cfg.DownloadTimeSeconds = intValue(timeouts["stage3_ms"], cfg.DownloadTimeSeconds*1000) / 1000
	} else {
		cfg.DownloadTimeSeconds = downloadTimeSeconds
	}
	if cfg.DownloadTimeSeconds <= 0 {
		cfg.DownloadTimeSeconds = 1
	}
	cfg.TCPPort = intValue(firstNonNil(probe["tcp_port"], probe["tcpPort"]), cfg.TCPPort)
	cfg.URL = stringValue(probe["url"], cfg.URL)
	cfg.UserAgent = stringValue(firstNonNil(probe["user_agent"], probe["userAgent"]), cfg.UserAgent)
	cfg.HostHeader = stringValue(firstNonNil(probe["host_header"], probe["hostHeader"]), cfg.HostHeader)
	cfg.SNI = stringValue(probe["sni"], cfg.SNI)
	cfg.Httping = boolValue(probe["httping"], rawStrategy == "http-colo")
	cfg.HttpingStatusCode = intValue(firstNonNil(probe["httping_status_code"], probe["httpingStatusCode"]), cfg.HttpingStatusCode)
	cfg.HttpingCFColo = stringValue(firstNonNil(probe["httping_cf_colo"], probe["httpingCfColo"]), cfg.HttpingCFColo)
	cfg.MaxDelayMS = intValue(firstNonNil(thresholds["max_tcp_latency_ms"], thresholds["max_http_latency_ms"]), cfg.MaxDelayMS)
	cfg.MinDelayMS = intValue(firstNonNil(probe["min_delay_ms"], probe["minDelayMs"]), cfg.MinDelayMS)
	cfg.MaxLossRate = floatValue(firstNonNil(probe["max_loss_rate"], probe["maxLossRate"]), cfg.MaxLossRate)
	cfg.MinSpeedMB = floatValue(thresholds["min_download_mbps"], cfg.MinSpeedMB)
	cfg.PrintNum = intValue(firstNonNil(probe["print_num"], probe["printNum"]), cfg.PrintNum)
	cfg.DisableDownload = strategy == "fast"
	cfg.TestAll = false
	cfg.Debug = boolValue(probe["debug"], cfg.Debug)
	cfg.DebugCaptureAddress = stringValue(firstNonNil(probe["debug_capture_address"], probe["debugCaptureAddress"]), cfg.DebugCaptureAddress)

	switch strategy {
	case "fast":
		cfg.MinSpeedMB = 0
	case "full":
		cfg.DisableDownload = false
	}

	if fileName := strings.TrimSpace(stringValue(exportCfg["file_name"], "")); fileName != "" {
		targetDir := strings.TrimSpace(stringValue(exportCfg["target_dir"], ""))
		if targetDir != "" {
			cfg.OutputFile = filepath.Join(targetDir, fileName)
		} else {
			cfg.OutputFile = fileName
		}
		cfg.WriteOutput = true
	}

	return normalizeProbeConfig(cfg)
}

func desktopSourceName(source DesktopSource) string {
	if name := strings.TrimSpace(source.Name); name != "" {
		return name
	}
	if label := strings.TrimSpace(source.Label); label != "" {
		return label
	}
	switch desktopSourceKind(source) {
	case "file":
		return "本地文件来源"
	case "inline":
		return "手动输入来源"
	default:
		return "远程来源"
	}
}

func desktopSourceKind(source DesktopSource) string {
	switch strings.ToLower(strings.TrimSpace(source.Kind)) {
	case "inline", "file":
		return strings.ToLower(strings.TrimSpace(source.Kind))
	default:
		return "url"
	}
}

func desktopSourceEnabled(source DesktopSource) bool {
	if source.Enabled {
		return true
	}
	return source.ID == "" && source.Name == "" && source.IPLimit == 0 && source.IPMode == ""
}

func desktopSourceIPLimit(source DesktopSource) int {
	if source.IPLimit <= 0 {
		return 3000
	}
	return source.IPLimit
}

func desktopSourceIPMode(source DesktopSource) string {
	if strings.EqualFold(strings.TrimSpace(source.IPMode), "mcis") {
		return "mcis"
	}
	return "traverse"
}

func prepareDesktopSources(cfg ProbeConfig, sources []DesktopSource) preparedDesktopSources {
	client := newDesktopSourceHTTPClient(cfg)
	now := time.Now()
	parts := make([]string, 0)
	statuses := make([]DesktopSourceStatus, 0, len(sources))
	warnings := make([]string, 0)
	invalidCount := 0

	for index, source := range sources {
		name := desktopSourceName(source)
		if name == "" {
			name = fmt.Sprintf("输入源 %d", index+1)
		}

		status := DesktopSourceStatus{
			ID:               strings.TrimSpace(source.ID),
			LastFetchedAt:    strings.TrimSpace(source.LastFetchedAt),
			LastFetchedCount: source.LastFetchedCount,
			StatusText:       strings.TrimSpace(source.StatusText),
		}

		if !desktopSourceEnabled(source) {
			if status.StatusText == "" {
				status.StatusText = "已停用，启动任务时不会读取该输入源。"
			}
			statuses = append(statuses, status)
			continue
		}

		result, err := processDesktopSource(cfg, source, client, now)
		if err != nil {
			statuses = append(statuses, result.Status)
			invalidCount += result.InvalidCount
			warnings = append(warnings, fmt.Sprintf("输入源 %s 读取失败：%v", name, err))
			warnings = append(warnings, result.Warnings...)
			continue
		}

		warnings = append(warnings, result.Warnings...)
		invalidCount += result.InvalidCount
		if len(result.Entries) > 0 {
			parts = append(parts, strings.Join(result.Entries, "\n"))
		}
		statuses = append(statuses, result.Status)
	}

	return preparedDesktopSources{
		Text:           strings.Join(parts, "\n"),
		InvalidCount:   invalidCount,
		SourceStatuses: statuses,
		Warnings:       dedupeStrings(warnings),
	}
}

func loadDesktopSourceContent(source DesktopSource, cfg ProbeConfig, client *http.Client) (string, error) {
	switch desktopSourceKind(source) {
	case "inline":
		return strings.TrimSpace(source.Content), nil
	case "file":
		path := strings.TrimSpace(source.Path)
		if path == "" {
			return "", errors.New("缺少文件路径")
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		return string(raw), nil
	default:
		url := strings.TrimSpace(source.URL)
		if url == "" {
			return "", errors.New("缺少远程 URL")
		}
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return "", err
		}
		httpcfg.Resolve(cfg.UserAgent, "", "", "", true).Apply(req)
		res, err := client.Do(req)
		if err != nil {
			return "", err
		}
		raw, readErr := io.ReadAll(res.Body)
		_ = res.Body.Close()
		if readErr != nil {
			return "", readErr
		}
		if res.StatusCode < 200 || res.StatusCode >= 300 {
			return "", fmt.Errorf("远程来源返回状态 %s", res.Status)
		}
		return string(raw), nil
	}
}

func enumerateCIDRIPs(ipNet *net.IPNet, limit int) ([]string, bool) {
	if limit <= 0 {
		return nil, true
	}
	_, bits := ipNet.Mask.Size()
	current := cloneIPForBits(ipNet.IP, bits)
	entries := make([]string, 0, limit)

	for len(entries) < limit && ipNet.Contains(current) {
		entries = append(entries, current.String())
		incrementIP(current)
	}

	return entries, ipNet.Contains(current)
}

func cloneIPForBits(ip net.IP, bits int) net.IP {
	if bits == 32 {
		cloned := append(net.IP(nil), ip.To4()...)
		return cloned
	}
	cloned := append(net.IP(nil), ip.To16()...)
	return cloned
}

func incrementIP(ip net.IP) {
	for i := len(ip) - 1; i >= 0; i-- {
		ip[i]++
		if ip[i] != 0 {
			return
		}
	}
}

func dedupeStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func persistDesktopSourceStatuses(statuses []DesktopSourceStatus) error {
	if len(statuses) == 0 {
		return nil
	}

	path := desktopConfigFilePath()
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}

	var saved map[string]interface{}
	if err := json.Unmarshal(raw, &saved); err != nil {
		return err
	}

	snapshot := mapValue(saved["config_snapshot"])
	if len(snapshot) == 0 {
		snapshot = saved
	}
	sourceItems, ok := snapshot["sources"].([]interface{})
	if !ok {
		return nil
	}

	statusMap := make(map[string]DesktopSourceStatus, len(statuses))
	for _, status := range statuses {
		if id := strings.TrimSpace(status.ID); id != "" {
			statusMap[id] = status
		}
	}
	if len(statusMap) == 0 {
		return nil
	}

	for index, item := range sourceItems {
		sourceMap := mapValue(item)
		id := strings.TrimSpace(stringValue(sourceMap["id"], ""))
		status, exists := statusMap[id]
		if !exists {
			continue
		}
		sourceMap["last_fetched_at"] = status.LastFetchedAt
		sourceMap["last_fetched_count"] = status.LastFetchedCount
		sourceMap["status_text"] = status.StatusText
		sourceItems[index] = sourceMap
	}

	snapshot["sources"] = sourceItems
	body := map[string]interface{}{
		"config_snapshot": snapshot,
		"saved_at":        time.Now().Format(time.RFC3339),
		"schema_version":  guiSchemaVersion,
	}
	encoded, err := json.MarshalIndent(body, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, encoded, 0o600)
}

func mapValue(value interface{}) map[string]interface{} {
	if typed, ok := value.(map[string]interface{}); ok {
		return typed
	}
	return map[string]interface{}{}
}

func firstNonNil(values ...interface{}) interface{} {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func intValue(value interface{}, fallback int) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case json.Number:
		parsed, err := typed.Int64()
		if err == nil {
			return int(parsed)
		}
	case string:
		var parsed int
		if _, err := fmt.Sscanf(typed, "%d", &parsed); err == nil {
			return parsed
		}
	}
	return fallback
}

func floatValue(value interface{}, fallback float64) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	case json.Number:
		parsed, err := typed.Float64()
		if err == nil {
			return parsed
		}
	case string:
		var parsed float64
		if _, err := fmt.Sscanf(typed, "%f", &parsed); err == nil {
			return parsed
		}
	}
	return fallback
}

func boolValue(value interface{}, fallback bool) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case int:
		return typed != 0
	case int64:
		return typed != 0
	case float64:
		return typed != 0
	case json.Number:
		parsed, err := typed.Int64()
		if err == nil {
			return parsed != 0
		}
	case string:
		normalized := strings.ToLower(strings.TrimSpace(typed))
		switch normalized {
		case "1", "true", "yes", "on":
			return true
		case "0", "false", "no", "off":
			return false
		}
	}
	return fallback
}

func stringValue(value interface{}, fallback string) string {
	if value == nil {
		return fallback
	}
	if typed, ok := value.(string); ok {
		return typed
	}
	return fmt.Sprint(value)
}

func configFilePath() string {
	dir, err := os.UserConfigDir()
	if err != nil || dir == "" {
		dir = "."
	}
	return filepath.Join(dir, "CFST-GUI", "config.json")
}

func desktopConfigFilePath() string {
	dir, err := os.UserConfigDir()
	if err != nil || dir == "" {
		dir = "."
	}
	return filepath.Join(dir, "CFST-GUI", "desktop-config.json")
}

func debugLogFilePath() string {
	return filepath.Join(filepath.Dir(desktopConfigFilePath()), "cfip-log.txt")
}
