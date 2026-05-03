package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
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
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const guiSchemaVersion = "cfst-gui-wails-v1"
const defaultFileTestURL = "https://speed.cloudflare.com/__down?bytes=10000000"

type App struct {
	ctx context.Context

	runMu sync.Mutex

	trayMu        sync.Mutex
	trayAvailable bool
	quitting      bool

	probeControlMu sync.Mutex
	currentTaskID  string
	pausedTaskID   string
	pauseRequested bool
	pauseCond      *sync.Cond
	pauseEmitter   *desktopProbeEmitter
}

type ProbeConfig struct {
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
	Code          string   `json:"code"`
	Data          any      `json:"data"`
	Message       string   `json:"message"`
	OK            bool     `json:"ok"`
	SchemaVersion string   `json:"schema_version"`
	TaskID        *string  `json:"task_id"`
	Warnings      []string `json:"warnings"`
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
	Config         ProbeConfig           `json:"config"`
	ConfigWarnings []string              `json:"configWarnings,omitempty"`
	SourceStatuses []DesktopSourceStatus `json:"sourceStatuses,omitempty"`
	SourceText     string                `json:"sourceText"`
	TaskID         string                `json:"taskId,omitempty"`
}

type DesktopProbePayload struct {
	Config  map[string]any  `json:"config"`
	Sources []DesktopSource `json:"sources"`
	TaskID  string          `json:"task_id"`
}

type DesktopSource struct {
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
	TraceDelayMS    float64 `json:"traceDelayMs"`
}

type StrategyPreset struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Config      ProbeConfig `json:"config"`
}

var (
	desktopTCPProbeRunner = func() utils.PingDelaySet {
		return task.NewPing().Run().FilterDelay().FilterLossRate()
	}
	desktopTraceProbeRunner    = task.TestTraceAvailability
	desktopDownloadProbeRunner = task.TestDownloadSpeed
)

func NewApp() *App {
	app := &App{}
	app.ensureProbeControl()
	return app
}

func (a *App) ensureProbeControl() {
	a.probeControlMu.Lock()
	defer a.probeControlMu.Unlock()
	if a.pauseCond == nil {
		a.pauseCond = sync.NewCond(&a.probeControlMu)
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.startTray()
}

func (a *App) GetHealth() HealthResult {
	return HealthResult{
		ConfigPath:     configFilePath(),
		Online:         true,
		SchemaVersion:  guiSchemaVersion,
		Service:        "CFST Wails Bridge",
		Version:        appVersion(),
		WailsTransport: "window.go.main.App",
	}
}

func (a *App) GetAppInfo() DesktopCommandResult {
	return desktopCommandResult("APP_INFO_READY", appInfoPayload(), "应用信息已读取。", true, nil, nil)
}

func (a *App) CheckForUpdates(payload map[string]any) DesktopCommandResult {
	_ = payload
	info, err := checkGitHubReleaseForUpdate(context.Background())
	if err != nil {
		return desktopCommandResult("UPDATE_CHECK_FAILED", map[string]any{
			"current_version": appVersion(),
			"release_url":     releasePageURL,
		}, err.Error(), false, nil, nil)
	}
	message := "当前已是最新版本。"
	if info.UpdateAvailable {
		message = fmt.Sprintf("发现新版本 %s。", info.LatestVersion)
	}
	return desktopCommandResult("UPDATE_CHECK_OK", info, message, true, nil, nil)
}

func (a *App) DownloadAndInstallUpdate(payload map[string]any) DesktopCommandResult {
	info, err := checkGitHubReleaseForUpdate(context.Background())
	if err != nil {
		return desktopCommandResult("UPDATE_CHECK_FAILED", nil, err.Error(), false, nil, nil)
	}
	if !info.UpdateAvailable {
		return desktopCommandResult("UPDATE_NOT_AVAILABLE", info, "当前已是最新版本。", true, nil, nil)
	}
	result, err := downloadAndInstallUpdate(context.Background(), info, stringValue(firstNonNil(payload["download_dir"], payload["downloadDir"]), ""))
	if err != nil {
		return desktopCommandResult("UPDATE_INSTALL_FAILED", result, err.Error(), false, nil, nil)
	}
	if result.InstallStarted {
		go func() {
			time.Sleep(200 * time.Millisecond)
			a.markQuitting()
			if a.ctx != nil {
				wailsruntime.Quit(a.ctx)
			}
		}()
	}
	return desktopCommandResult("UPDATE_INSTALL_READY", result, "更新包已下载并触发安装流程。", true, nil, nil)
}

func (a *App) OpenReleasePage() DesktopCommandResult {
	if err := openExternalURL(releasePageURL); err != nil {
		return desktopCommandResult("RELEASE_OPEN_FAILED", map[string]any{
			"release_url": releasePageURL,
		}, err.Error(), false, nil, nil)
	}
	return desktopCommandResult("RELEASE_OPENED", map[string]any{
		"release_url": releasePageURL,
	}, "已打开发行页。", true, nil, nil)
}

func (a *App) ShowMainWindow() DesktopCommandResult {
	if a.ctx == nil {
		return desktopCommandResult("WINDOW_UNAVAILABLE", nil, "主窗口尚未初始化。", false, nil, nil)
	}
	wailsruntime.WindowShow(a.ctx)
	return desktopCommandResult("WINDOW_SHOWN", nil, "主界面已打开。", true, nil, nil)
}

func (a *App) HideMainWindow() DesktopCommandResult {
	if a.ctx == nil {
		return desktopCommandResult("WINDOW_UNAVAILABLE", nil, "主窗口尚未初始化。", false, nil, nil)
	}
	wailsruntime.WindowHide(a.ctx)
	return desktopCommandResult("WINDOW_HIDDEN", nil, "主界面已隐藏。", true, nil, nil)
}

func (a *App) QuitApplication() DesktopCommandResult {
	a.markQuitting()
	if a.ctx != nil {
		wailsruntime.Quit(a.ctx)
	}
	return desktopCommandResult("APP_QUIT_REQUESTED", nil, "已请求关闭软件。", true, nil, nil)
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
			Description: "执行 TCP 和追踪筛选，跳过文件测速环节，适合快速更新日常节点。",
			Config:      base,
		},
		{
			ID:          full.Strategy,
			Name:        "完整模式",
			Description: "在 TCP 和追踪筛选后追加真实文件下载测速，更适合大流量业务和流媒体代理。",
			Config:      full,
		},
	}
}

func (a *App) LoadDesktopConfig() DesktopCommandResult {
	path := desktopConfigFilePath()
	snapshot := defaultDesktopConfigSnapshot()
	storage := resolveStorageState()
	profiles, profileErr := loadProfileStore()
	warnings := make([]string, 0)
	if profileErr != nil {
		warnings = append(warnings, fmt.Sprintf("读取配置档案失败：%v", profileErr))
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return desktopCommandResult("CONFIG_READY", map[string]any{
				"configPath":      path,
				"config_snapshot": snapshot,
				"profiles":        profiles,
				"storage":         storage,
			}, "配置文件尚未创建，已加载默认桌面配置。", true, nil, warnings)
		}
		return desktopCommandResult("CONFIG_READ_FAILED", nil, err.Error(), false, nil, nil)
	}

	var saved map[string]any
	if err := json.Unmarshal(raw, &saved); err != nil {
		return desktopCommandResult("CONFIG_PARSE_FAILED", nil, err.Error(), false, nil, nil)
	}
	if value, ok := saved["config_snapshot"].(map[string]any); ok {
		snapshot = value
	} else {
		snapshot = saved
	}
	_, configWarnings := desktopConfigToProbeConfig(snapshot)
	warnings = append(warnings, configWarnings...)

	return desktopCommandResult("CONFIG_READ_OK", map[string]any{
		"configPath":      path,
		"config_snapshot": snapshot,
		"profiles":        profiles,
		"storage":         storage,
	}, "配置已加载。", true, nil, warnings)
}

func (a *App) SaveDesktopConfig(payload map[string]any) DesktopCommandResult {
	path := desktopConfigFilePath()
	snapshot, ok := payload["config_snapshot"].(map[string]any)
	if !ok {
		return desktopCommandResult("CONFIG_INVALID", nil, "缺少 config_snapshot。", false, nil, nil)
	}

	if err := writeDesktopConfigSnapshot(path, snapshot); err != nil {
		return desktopCommandResult("CONFIG_WRITE_FAILED", nil, err.Error(), false, nil, nil)
	}
	_, warnings := desktopConfigToProbeConfig(snapshot)
	profiles, profileErr := loadProfileStore()
	if profileErr != nil {
		warnings = append(warnings, fmt.Sprintf("读取配置档案失败：%v", profileErr))
	}

	return desktopCommandResult("CONFIG_SAVE_OK", map[string]any{
		"configPath":      path,
		"config_snapshot": snapshot,
		"profiles":        profiles,
		"storage":         resolveStorageState(),
	}, "配置已保存到本机。", true, nil, warnings)
}

func (a *App) RunDesktopProbe(payload DesktopProbePayload) (ProbeRunResult, error) {
	cfg, configWarnings := desktopConfigToProbeConfig(payload.Config)
	taskID := strings.TrimSpace(payload.TaskID)
	if taskID == "" {
		taskID = fmt.Sprintf("cfst-%d", time.Now().UnixNano())
	}
	cfg = applyDesktopExportConfig(cfg, payload.Config, taskID)
	emitter := newDesktopProbeEmitter(a, taskID, time.Duration(cfg.EventThrottleMS)*time.Millisecond)
	prepareStart := time.Now()
	prepared := prepareDesktopSources(cfg, payload.Sources)
	if err := persistDesktopSourceStatuses(prepared.SourceStatuses); err != nil {
		prepared.Warnings = append(prepared.Warnings, fmt.Sprintf("更新输入源状态失败：%v", err))
	}
	preparedSummary := summarizeSource(prepared.Text)
	preparedSummary, stage1LimitWarnings := applyStage1CandidateLimit(cfg, preparedSummary)
	prepared.Warnings = append(prepared.Warnings, stage1LimitWarnings...)
	prepared.Text = strings.Join(preparedSummary.Valid, "\n")
	preparedInvalidCount := preparedSummary.InvalidCount + prepared.InvalidCount
	emitter.emit("probe.preprocessed", map[string]any{
		"accepted":        preparedSummary.ValidCount,
		"filtered":        preparedSummary.DuplicateCount,
		"invalid":         preparedInvalidCount,
		"source_statuses": prepared.SourceStatuses,
		"stage":           "stage0_pool",
		"total":           preparedSummary.ValidCount,
	})
	if strings.TrimSpace(prepared.Text) == "" && len(prepared.Warnings) > 0 {
		err := errors.New(strings.Join(prepared.Warnings, "；"))
		logDesktopProbePreparationFailure(cfg, taskID, preparedSummary, preparedInvalidCount, prepared.SourceStatuses, time.Since(prepareStart), err)
		emitter.emit("probe.failed", map[string]any{
			"message":     err.Error(),
			"recoverable": false,
		})
		return ProbeRunResult{}, err
	}
	a.setCurrentProbeTask(taskID, emitter)
	defer a.clearCurrentProbeTask(taskID)
	result, err := a.runProbe(ProbeRequest{
		ConfigWarnings: configWarnings,
		Config:         cfg,
		SourceStatuses: prepared.SourceStatuses,
		SourceText:     prepared.Text,
		TaskID:         taskID,
	}, emitter)
	if err != nil {
		emitter.emit("probe.failed", map[string]any{
			"message":     err.Error(),
			"recoverable": false,
		})
		return ProbeRunResult{}, err
	}
	result.SourceStatuses = prepared.SourceStatuses
	result.Warnings = dedupeStrings(append(result.Warnings, prepared.Warnings...))
	exportedCount := 0
	if strings.TrimSpace(result.OutputFile) != "" && len(result.Results) > 0 {
		exportedCount = len(result.Results)
	}
	emitter.emit("probe.completed", map[string]any{
		"exported": exportedCount,
		"failed":   result.Summary.Failed,
		"failure_summary": map[string]any{
			"duplicate_count": preparedSummary.DuplicateCount,
			"invalid_count":   preparedInvalidCount,
		},
		"passed":       result.Summary.Passed,
		"result_count": len(result.Results),
		"target_path":  result.OutputFile,
	})
	return result, nil
}

func (a *App) CancelProbe(payload map[string]any) DesktopCommandResult {
	a.ensureProbeControl()
	taskID := strings.TrimSpace(stringValue(firstNonNil(payload["task_id"], payload["taskId"]), ""))

	a.probeControlMu.Lock()
	if taskID == "" {
		taskID = a.currentTaskID
	}
	if taskID == "" || taskID != a.currentTaskID {
		a.probeControlMu.Unlock()
		return desktopCommandResult("PROBE_STOP_UNAVAILABLE", nil, "当前没有可暂停的探测任务。", false, &taskID, nil)
	}
	a.pauseRequested = true
	a.pausedTaskID = taskID
	emitter := a.pauseEmitter
	if a.pauseCond != nil {
		a.pauseCond.Broadcast()
	}
	a.probeControlMu.Unlock()

	if emitter != nil {
		emitter.emit("probe.cooling", map[string]any{
			"reason":      "已收到暂停请求，任务将在当前安全点暂停。",
			"recoverable": true,
		})
	}
	return desktopCommandResult("PROBE_STOP_REQUESTED", nil, "已请求暂停探测任务。", true, &taskID, nil)
}

func (a *App) ResumeProbe(payload map[string]any) DesktopCommandResult {
	a.ensureProbeControl()
	taskID := strings.TrimSpace(stringValue(firstNonNil(payload["task_id"], payload["taskId"]), ""))

	a.probeControlMu.Lock()
	if taskID == "" {
		taskID = a.pausedTaskID
	}
	if taskID == "" || taskID != a.pausedTaskID || !a.pauseRequested {
		a.probeControlMu.Unlock()
		return desktopCommandResult("PROBE_RESUME_UNAVAILABLE", nil, "当前没有可继续的探测任务。", false, &taskID, nil)
	}
	a.pauseRequested = false
	a.pausedTaskID = ""
	if a.pauseCond != nil {
		a.pauseCond.Broadcast()
	}
	a.probeControlMu.Unlock()

	return desktopCommandResult("PROBE_RESUME_REQUESTED", nil, "已请求继续探测任务。", true, &taskID, nil)
}

func (a *App) setCurrentProbeTask(taskID string, emitter *desktopProbeEmitter) {
	a.ensureProbeControl()
	a.probeControlMu.Lock()
	defer a.probeControlMu.Unlock()
	a.currentTaskID = taskID
	a.pausedTaskID = ""
	a.pauseRequested = false
	a.pauseEmitter = emitter
	if a.pauseCond != nil {
		a.pauseCond.Broadcast()
	}
}

func (a *App) clearCurrentProbeTask(taskID string) {
	a.ensureProbeControl()
	a.probeControlMu.Lock()
	defer a.probeControlMu.Unlock()
	if a.currentTaskID == taskID {
		a.currentTaskID = ""
		a.pausedTaskID = ""
		a.pauseRequested = false
		a.pauseEmitter = nil
		if a.pauseCond != nil {
			a.pauseCond.Broadcast()
		}
	}
}

func (a *App) waitIfProbePaused(taskID, stage, ip string, emitter *desktopProbeEmitter) {
	a.ensureProbeControl()
	a.probeControlMu.Lock()
	announced := false
	for a.currentTaskID == taskID && a.pauseRequested && a.pausedTaskID == taskID {
		if !announced {
			a.probeControlMu.Unlock()
			if emitter != nil {
				emitter.emit("probe.cooling", map[string]any{
					"ip":          ip,
					"reason":      fmt.Sprintf("%s 已暂停，点击继续任务后从当前进度继续。", stage),
					"recoverable": true,
					"stage":       stage,
				})
			}
			a.probeControlMu.Lock()
			announced = true
			continue
		}
		a.pauseCond.Wait()
	}
	a.probeControlMu.Unlock()
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

func (a *App) SelectPath(payload map[string]any) DesktopCommandResult {
	if a.ctx == nil {
		return desktopCommandResult("PATH_DIALOG_UNAVAILABLE", nil, "系统文件选择器尚未初始化。", false, nil, nil)
	}

	mode := normalizePathSelectionMode(stringValue(firstNonNil(payload["mode"], payload["kind"]), ""))
	currentPath := strings.TrimSpace(stringValue(firstNonNil(payload["current_path"], payload["currentPath"]), ""))
	defaultFileName := strings.TrimSpace(stringValue(firstNonNil(payload["default_file_name"], payload["defaultFileName"]), ""))
	title := strings.TrimSpace(stringValue(payload["title"], ""))
	defaultDir := selectPathDefaultDirectory(currentPath)

	data := map[string]any{
		"canceled": false,
		"mode":     mode,
	}
	cancel := func(message string) DesktopCommandResult {
		data["canceled"] = true
		return desktopCommandResult("PATH_SELECTION_CANCELED", data, message, true, nil, nil)
	}

	switch mode {
	case "export_target", "export_dir", "directory", "storage_dir":
		if title == "" {
			if mode == "storage_dir" {
				title = "选择储存目录"
			} else {
				title = "选择导出目录"
			}
		}
		if defaultDir == "" && mode == "storage_dir" {
			defaultDir = storageRoot()
		}
		selected, err := wailsruntime.OpenDirectoryDialog(a.ctx, wailsruntime.OpenDialogOptions{
			Title:            title,
			DefaultDirectory: defaultDir,
		})
		if err != nil {
			return desktopCommandResult("PATH_SELECTION_FAILED", nil, err.Error(), false, nil, nil)
		}
		if strings.TrimSpace(selected) == "" {
			return cancel("已取消选择导出目录。")
		}
		data["path"] = selected
		data["directory"] = selected
		message := "已选择导出目录。"
		if mode == "storage_dir" {
			message = "已选择储存目录。"
		}
		return desktopCommandResult("PATH_SELECTED", data, message, true, nil, nil)

	case "config_import", "import_config":
		if title == "" {
			title = "导入配置文件"
		}
		selected, err := wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
			Title:            title,
			DefaultDirectory: defaultDir,
			Filters: []wailsruntime.FileFilter{
				{DisplayName: "JSON 配置文件 (*.json)", Pattern: "*.json"},
				{DisplayName: "所有文件 (*.*)", Pattern: "*.*"},
			},
		})
		if err != nil {
			return desktopCommandResult("PATH_SELECTION_FAILED", nil, err.Error(), false, nil, nil)
		}
		if strings.TrimSpace(selected) == "" {
			return cancel("已取消导入配置。")
		}
		raw, err := os.ReadFile(selected)
		if err != nil {
			return desktopCommandResult("CONFIG_IMPORT_READ_FAILED", nil, err.Error(), false, nil, nil)
		}
		data["path"] = selected
		data["content"] = string(raw)
		return desktopCommandResult("PATH_SELECTED", data, "已读取配置文件。", true, nil, nil)

	case "export_file", "save_file", "config_export":
		if title == "" {
			if mode == "config_export" {
				title = "导出配置文件"
			} else {
				title = "选择导出文件"
			}
		}
		if defaultFileName == "" {
			if mode == "config_export" {
				defaultFileName = fmt.Sprintf("cfst-gui-config-%s.json", time.Now().Format("20060102-150405"))
			} else {
				defaultFileName = "result.csv"
			}
		}
		filters := []wailsruntime.FileFilter{
			{DisplayName: "CSV 文件 (*.csv)", Pattern: "*.csv"},
			{DisplayName: "所有文件 (*.*)", Pattern: "*.*"},
		}
		if mode == "config_export" {
			filters = []wailsruntime.FileFilter{
				{DisplayName: "JSON 配置文件 (*.json)", Pattern: "*.json"},
				{DisplayName: "所有文件 (*.*)", Pattern: "*.*"},
			}
		}
		selected, err := wailsruntime.SaveFileDialog(a.ctx, wailsruntime.SaveDialogOptions{
			Title:            title,
			DefaultDirectory: defaultDir,
			DefaultFilename:  defaultFileName,
			Filters:          filters,
		})
		if err != nil {
			return desktopCommandResult("PATH_SELECTION_FAILED", nil, err.Error(), false, nil, nil)
		}
		if strings.TrimSpace(selected) == "" {
			return cancel("已取消选择导出文件。")
		}
		data["path"] = selected
		data["directory"] = filepath.Dir(selected)
		data["file_name"] = filepath.Base(selected)
		return desktopCommandResult("PATH_SELECTED", data, "已选择导出文件。", true, nil, nil)

	default:
		if title == "" {
			title = "选择输入源文件"
		}
		selected, err := wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
			Title:            title,
			DefaultDirectory: defaultDir,
			Filters: []wailsruntime.FileFilter{
				{DisplayName: "文本/CSV 文件 (*.txt, *.csv)", Pattern: "*.txt;*.csv"},
				{DisplayName: "所有文件 (*.*)", Pattern: "*.*"},
			},
		})
		if err != nil {
			return desktopCommandResult("PATH_SELECTION_FAILED", nil, err.Error(), false, nil, nil)
		}
		if strings.TrimSpace(selected) == "" {
			return cancel("已取消选择输入源文件。")
		}
		data["path"] = selected
		return desktopCommandResult("PATH_SELECTED", data, "已选择输入源文件。", true, nil, nil)
	}
}

func (a *App) SetStorageDirectory(payload map[string]any) DesktopCommandResult {
	status, migration, err := setStorageDirectory(payload)
	data := map[string]any{
		"migration": migration,
		"storage":   status,
	}
	if err != nil {
		return desktopCommandResult("STORAGE_SET_FAILED", data, err.Error(), false, nil, nil)
	}
	return desktopCommandResult("STORAGE_SET_OK", data, "储存目录已更新。", true, nil, nil)
}

func (a *App) CheckStorageHealth(payload map[string]any) DesktopCommandResult {
	path := strings.TrimSpace(stringValue(firstNonNil(payload["storage_dir"], payload["storageDir"], payload["path"], payload["directory"]), ""))
	if path == "" {
		path = storageRoot()
	}
	health := checkStorageHealthForPath(path, false)
	return desktopCommandResult("STORAGE_HEALTH_READY", map[string]any{
		"health":  health,
		"storage": resolveStorageState(),
	}, "储存目录健康检查已完成。", true, nil, nil)
}

func (a *App) ExportConfig(payload map[string]any) DesktopCommandResult {
	targetPath := strings.TrimSpace(stringValue(firstNonNil(payload["target_path"], payload["targetPath"], payload["path"]), ""))
	if targetPath == "" {
		return desktopCommandResult("CONFIG_EXPORT_INVALID", nil, "缺少导出目标路径。", false, nil, nil)
	}
	snapshot := mapValue(firstNonNil(payload["config_snapshot"], payload["configSnapshot"]))
	if len(snapshot) == 0 {
		loaded, err := loadDesktopConfigSnapshotFromDisk()
		if err != nil {
			return desktopCommandResult("CONFIG_EXPORT_READ_FAILED", nil, err.Error(), false, nil, nil)
		}
		snapshot = loaded
	}
	profiles, err := loadProfileStore()
	if err != nil {
		return desktopCommandResult("CONFIG_EXPORT_PROFILE_FAILED", nil, err.Error(), false, nil, nil)
	}
	body := map[string]any{
		"app_version":     version,
		"config_snapshot": snapshot,
		"exported_at":     time.Now().Format(time.RFC3339),
		"profiles":        profiles,
		"schema_version":  guiSchemaVersion,
		"storage":         resolveStorageState(),
	}
	raw, err := json.MarshalIndent(body, "", "  ")
	if err != nil {
		return desktopCommandResult("CONFIG_EXPORT_SERIALIZE_FAILED", nil, err.Error(), false, nil, nil)
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return desktopCommandResult("CONFIG_EXPORT_WRITE_FAILED", nil, err.Error(), false, nil, nil)
	}
	if err := os.WriteFile(targetPath, raw, 0o600); err != nil {
		return desktopCommandResult("CONFIG_EXPORT_WRITE_FAILED", nil, err.Error(), false, nil, nil)
	}
	return desktopCommandResult("CONFIG_EXPORT_OK", map[string]any{
		"path": targetPath,
	}, "完整配置已导出。", true, nil, []string{"导出的配置包含完整 Cloudflare API Token，请仅保存到可信位置。"})
}

func (a *App) BackupCurrentConfig(payload map[string]any) DesktopCommandResult {
	snapshot := mapValue(firstNonNil(payload["config_snapshot"], payload["configSnapshot"]))
	if len(snapshot) == 0 {
		loaded, err := loadDesktopConfigSnapshotFromDisk()
		if err != nil {
			return desktopCommandResult("CONFIG_BACKUP_READ_FAILED", nil, err.Error(), false, nil, nil)
		}
		snapshot = loaded
	}
	targetDir := filepath.Join(storageRoot(), "backups")
	targetPath := filepath.Join(targetDir, fmt.Sprintf("config-%s.json", time.Now().Format("20060102-150405")))
	body := map[string]any{
		"backed_up_at":    time.Now().Format(time.RFC3339),
		"config_snapshot": snapshot,
		"schema_version":  guiSchemaVersion,
	}
	raw, err := json.MarshalIndent(body, "", "  ")
	if err != nil {
		return desktopCommandResult("CONFIG_BACKUP_SERIALIZE_FAILED", nil, err.Error(), false, nil, nil)
	}
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return desktopCommandResult("CONFIG_BACKUP_WRITE_FAILED", nil, err.Error(), false, nil, nil)
	}
	if err := os.WriteFile(targetPath, raw, 0o600); err != nil {
		return desktopCommandResult("CONFIG_BACKUP_WRITE_FAILED", nil, err.Error(), false, nil, nil)
	}
	return desktopCommandResult("CONFIG_BACKUP_OK", map[string]any{
		"path": targetPath,
	}, "当前配置已备份。", true, nil, nil)
}

func (a *App) LoadProfiles() DesktopCommandResult {
	store, err := loadProfileStore()
	if err != nil {
		return desktopCommandResult("PROFILE_LOAD_FAILED", nil, err.Error(), false, nil, nil)
	}
	return desktopCommandResult("PROFILE_LOAD_OK", store, "配置档案已加载。", true, nil, nil)
}

func (a *App) SaveCurrentProfile(payload map[string]any) DesktopCommandResult {
	snapshot := mapValue(firstNonNil(payload["config_snapshot"], payload["configSnapshot"]))
	if len(snapshot) == 0 {
		return desktopCommandResult("PROFILE_INVALID", nil, "缺少 config_snapshot。", false, nil, nil)
	}
	name := strings.TrimSpace(stringValue(payload["name"], ""))
	profileID := strings.TrimSpace(stringValue(firstNonNil(payload["profile_id"], payload["profileId"], payload["id"]), ""))
	if name == "" {
		name = "默认档案"
	}
	now := time.Now().Format(time.RFC3339)
	store, err := loadProfileStore()
	if err != nil {
		return desktopCommandResult("PROFILE_LOAD_FAILED", nil, err.Error(), false, nil, nil)
	}
	if profileID == "" {
		profileID = fmt.Sprintf("profile-%d", time.Now().UnixNano())
	}
	updated := false
	for index := range store.Items {
		if store.Items[index].ID != profileID {
			continue
		}
		store.Items[index].ConfigSnapshot = snapshot
		store.Items[index].Name = name
		store.Items[index].UpdatedAt = now
		updated = true
		break
	}
	if !updated {
		store.Items = append(store.Items, profileItem{
			ConfigSnapshot: snapshot,
			CreatedAt:      now,
			ID:             profileID,
			Name:           name,
			UpdatedAt:      now,
		})
	}
	if boolValue(firstNonNil(payload["set_active"], payload["setActive"]), true) {
		store.ActiveProfileID = profileID
	}
	if err := saveProfileStore(store); err != nil {
		return desktopCommandResult("PROFILE_SAVE_FAILED", nil, err.Error(), false, nil, nil)
	}
	return desktopCommandResult("PROFILE_SAVE_OK", store, "配置档案已保存。", true, nil, nil)
}

func (a *App) SwitchProfile(payload map[string]any) DesktopCommandResult {
	profileID := strings.TrimSpace(stringValue(firstNonNil(payload["profile_id"], payload["profileId"], payload["id"]), ""))
	if profileID == "" {
		return desktopCommandResult("PROFILE_INVALID", nil, "缺少 profile_id。", false, nil, nil)
	}
	store, err := loadProfileStore()
	if err != nil {
		return desktopCommandResult("PROFILE_LOAD_FAILED", nil, err.Error(), false, nil, nil)
	}
	for _, item := range store.Items {
		if item.ID != profileID {
			continue
		}
		store.ActiveProfileID = profileID
		if err := saveProfileStore(store); err != nil {
			return desktopCommandResult("PROFILE_SAVE_FAILED", nil, err.Error(), false, nil, nil)
		}
		if err := writeDesktopConfigSnapshot(desktopConfigFilePath(), item.ConfigSnapshot); err != nil {
			return desktopCommandResult("PROFILE_SWITCH_FAILED", nil, err.Error(), false, nil, nil)
		}
		return desktopCommandResult("PROFILE_SWITCH_OK", map[string]any{
			"configPath":      desktopConfigFilePath(),
			"config_snapshot": item.ConfigSnapshot,
			"profiles":        store,
			"storage":         resolveStorageState(),
		}, "配置档案已切换。", true, nil, nil)
	}
	return desktopCommandResult("PROFILE_NOT_FOUND", nil, "未找到配置档案。", false, nil, nil)
}

func (a *App) DeleteProfile(payload map[string]any) DesktopCommandResult {
	profileID := strings.TrimSpace(stringValue(firstNonNil(payload["profile_id"], payload["profileId"], payload["id"]), ""))
	if profileID == "" {
		return desktopCommandResult("PROFILE_INVALID", nil, "缺少 profile_id。", false, nil, nil)
	}
	store, err := loadProfileStore()
	if err != nil {
		return desktopCommandResult("PROFILE_LOAD_FAILED", nil, err.Error(), false, nil, nil)
	}
	nextItems := make([]profileItem, 0, len(store.Items))
	deleted := false
	for _, item := range store.Items {
		if item.ID == profileID {
			deleted = true
			continue
		}
		nextItems = append(nextItems, item)
	}
	if !deleted {
		return desktopCommandResult("PROFILE_NOT_FOUND", nil, "未找到配置档案。", false, nil, nil)
	}
	store.Items = nextItems
	if store.ActiveProfileID == profileID {
		store.ActiveProfileID = ""
	}
	if err := saveProfileStore(store); err != nil {
		return desktopCommandResult("PROFILE_DELETE_FAILED", nil, err.Error(), false, nil, nil)
	}
	return desktopCommandResult("PROFILE_DELETE_OK", store, "配置档案已删除。", true, nil, nil)
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

	snapshot.Probe, _ = normalizeProbeConfig(snapshot.Probe)
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
	snapshot.Probe, _ = normalizeProbeConfig(snapshot.Probe)
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
	a.runMu.Lock()
	defer a.runMu.Unlock()

	start := time.Now()
	cfg, normalizeWarnings := normalizeProbeConfig(req.Config)
	configWarnings := append([]string{}, req.ConfigWarnings...)
	configWarnings = append(configWarnings, normalizeWarnings...)
	taskID := strings.TrimSpace(req.TaskID)
	if taskID == "" && emitter != nil {
		taskID = emitter.taskID
	}
	if taskID == "" {
		taskID = fmt.Sprintf("cfst-%d", start.UnixNano())
	}
	utils.Debug = cfg.Debug
	closeDebugLog, debugWarnings := configureProbeDebugRuntime(cfg)
	utils.SetDebugLogContext(taskID)
	defer closeDebugLog()

	utils.DebugEvent("probe.start", map[string]any{
		"config":  debugProbeConfigSummary(cfg),
		"message": "探测任务启动。",
		"source": map[string]any{
			"status":          "pending",
			"source_statuses": req.SourceStatuses,
		},
		"task_id": taskID,
	})

	completedStages := make([]string, 0, 4)
	currentStage := "stage0_pool"
	stageStart := time.Now()
	utils.DebugEvent("stage.start", map[string]any{
		"message": "开始生成 IP 池。",
		"stage":   currentStage,
		"task_id": taskID,
	})
	_, source, err := resolveProbeSource(cfg, req.SourceText)
	if err != nil {
		logProbeFailed(taskID, currentStage, start, completedStages, err, false)
		return ProbeRunResult{}, err
	}
	var stage1LimitWarnings []string
	source, stage1LimitWarnings = applyStage1CandidateLimit(cfg, source)
	if source.ValidCount == 0 {
		err := errors.New("没有可用的 IP/CIDR 输入")
		logProbeFailed(taskID, currentStage, start, completedStages, err, false)
		return ProbeRunResult{}, err
	}

	cfg.IPText = strings.Join(source.Valid, ",")
	applyProbeConfig(cfg)
	task.InitRandSeed()
	utils.DebugEvent("stage.complete", map[string]any{
		"counts":      debugStage0Counts(source, source.InvalidCount),
		"duration_ms": time.Since(stageStart).Milliseconds(),
		"message":     "IP 池生成完成。",
		"source":      debugSourceSummary(source, req.SourceStatuses),
		"stage":       currentStage,
		"task_id":     taskID,
	})
	completedStages = append(completedStages, currentStage)

	totalWork := source.ValidCount
	task.LatencyProgressHook = func(processed, passed, failed, _ int) {
		if emitter == nil {
			return
		}
		emitter.emitProgress("stage1_tcp", processed, passed, failed, totalWork)
	}
	task.HeadProgressHook = nil
	task.TraceProgressHook = nil
	task.DownloadProgressHook = nil
	task.DownloadSpeedSampleHook = nil
	task.ProbePauseHook = nil
	defer func() {
		task.LatencyProgressHook = nil
		task.HeadProgressHook = nil
		task.TraceProgressHook = nil
		task.DownloadProgressHook = nil
		task.DownloadSpeedSampleHook = nil
		task.ProbePauseHook = nil
	}()
	if taskID != "" {
		task.ProbePauseHook = func(stage, ip string) {
			a.waitIfProbePaused(taskID, stage, ip, emitter)
		}
	}

	if emitter != nil {
		emitter.emitProgress("stage1_tcp", 0, 0, 0, totalWork)
	}
	task.CheckProbePause("stage1_tcp", "")

	task.Httping = false
	currentStage = "stage1_tcp"
	stageStart = time.Now()
	utils.DebugEvent("stage.start", map[string]any{
		"config": map[string]any{
			"candidate_limit":           cfg.Stage1Limit,
			"concurrency":               cfg.Routines,
			"max_loss_rate":             cfg.MaxLossRate,
			"max_tcp_latency_ms":        cfg.MaxDelayMS,
			"min_delay_ms":              cfg.MinDelayMS,
			"ping_times":                cfg.PingTimes,
			"retry_backoff_ms":          cfg.RetryBackoffMS,
			"retry_max_attempts":        cfg.RetryMaxAttempts,
			"skip_first_latency_sample": cfg.SkipFirstLatency,
			"tcp_port":                  cfg.TCPPort,
			"timeout_ms":                cfg.Stage1TimeoutMS,
		},
		"counts":  map[string]any{"total": totalWork},
		"message": "开始 TCP 测延迟。",
		"stage":   currentStage,
		"task_id": taskID,
	})
	tcpData := desktopTCPProbeRunner()
	utils.DebugEvent("stage.complete", map[string]any{
		"counts":      debugStageCounts(totalWork, len(tcpData), totalWork-len(tcpData)),
		"duration_ms": time.Since(stageStart).Milliseconds(),
		"message":     "TCP 测延迟完成。",
		"stage":       currentStage,
		"task_id":     taskID,
		"tcp": map[string]any{
			"delay_column":              "TCP延迟(ms)",
			"max_latency_ms":            cfg.MaxDelayMS,
			"ping_times":                cfg.PingTimes,
			"skip_first_latency_sample": cfg.SkipFirstLatency,
		},
	})
	completedStages = append(completedStages, currentStage)

	traceTotal := task.EstimateTraceProbeCount(len(tcpData))
	task.TraceProgressHook = func(processed, passed, failed, total int) {
		if emitter == nil {
			return
		}
		emitter.emitProgress("stage2_trace", processed, passed, failed, total)
	}
	if emitter != nil {
		emitter.emitProgress("stage2_trace", 0, 0, 0, traceTotal)
	}
	task.CheckProbePause("stage2_trace", "")
	currentStage = "stage2_trace"
	stageStart = time.Now()
	utils.DebugEvent("stage.start", map[string]any{
		"config": map[string]any{
			"accepted_status_code": cfg.HttpingStatusCode,
			"cf_colo_filter":       cfg.HttpingCFColo,
			"trace_concurrency":    cfg.HeadRoutines,
			"trace_max_latency_ms": cfg.HeadMaxDelayMS,
			"trace_routines_limit": task.MaxTraceRoutines,
			"trace_test_count":     cfg.HeadTestCount,
			"trace_url":            cfg.TraceURL,
			"retry_backoff_ms":     cfg.RetryBackoffMS,
			"retry_max_attempts":   cfg.RetryMaxAttempts,
			"timeout_ms":           cfg.Stage2TimeoutMS,
		},
		"counts":  map[string]any{"input": len(tcpData), "total": traceTotal},
		"message": "开始追踪探测。",
		"stage":   currentStage,
		"task_id": taskID,
	})
	traceData := desktopTraceProbeRunner(tcpData)
	utils.DebugEvent("stage.complete", map[string]any{
		"counts":      debugStageCounts(traceTotal, len(traceData), traceTotal-len(traceData)),
		"duration_ms": time.Since(stageStart).Milliseconds(),
		"trace": map[string]any{
			"accepted_status_code": cfg.HttpingStatusCode,
			"cf_colo_filter":       cfg.HttpingCFColo,
			"concurrency":          cfg.HeadRoutines,
			"max_latency_ms":       cfg.HeadMaxDelayMS,
			"url":                  cfg.TraceURL,
		},
		"message": "追踪探测完成。",
		"stage":   currentStage,
		"task_id": taskID,
	})
	completedStages = append(completedStages, currentStage)

	warnings := append(buildProbeWarnings(source), stage1LimitWarnings...)
	warnings = append(warnings, configWarnings...)
	warnings = append(warnings, debugWarnings...)
	if len(traceData) == 0 && len(tcpData) > 0 {
		warnings = append(warnings, "追踪探测未命中可用候选，已无可导出的结果。")
	}

	resultData := []utils.CloudflareIPData(traceData)
	if !cfg.DisableDownload {
		downloadInput := traceData
		downloadTotal := estimateDownloadProbeCount(len(downloadInput))
		currentStage = "stage3_get"
		stageStart = time.Now()
		utils.DebugEvent("stage.start", map[string]any{
			"config": map[string]any{
				"concurrency":                  cfg.Stage3Concurrency,
				"download_time_seconds_per_ip": cfg.DownloadTimeSeconds,
				"legacy_download_count":        cfg.TestCount,
				"min_download_mbps":            cfg.MinSpeedMB,
				"retry_backoff_ms":             cfg.RetryBackoffMS,
				"retry_max_attempts":           cfg.RetryMaxAttempts,
			},
			"counts":  map[string]any{"input": len(downloadInput), "total": downloadTotal},
			"message": "开始文件测速。",
			"stage":   currentStage,
			"task_id": taskID,
		})
		if downloadTotal > 0 {
			task.DownloadProgressHook = func(processed, qualified, _ int) {
				if emitter == nil {
					return
				}
				emitter.emitProgress("stage3_get", processed, qualified, processed-qualified, downloadTotal)
			}
			task.DownloadSpeedSampleHook = func(sample task.DownloadSpeedSample) {
				if emitter == nil {
					return
				}
				emitter.emitSpeed(sample)
			}
			if emitter != nil {
				emitter.emitProgress("stage3_get", 0, 0, 0, downloadTotal)
			}
		}
		task.CheckProbePause("stage3_get", "")
		speedData := desktopDownloadProbeRunner(downloadInput)
		utils.DebugEvent("stage.complete", map[string]any{
			"counts":      debugStageCounts(downloadTotal, len(speedData), downloadTotal-len(speedData)),
			"duration_ms": time.Since(stageStart).Milliseconds(),
			"get": map[string]any{
				"concurrency":                  cfg.Stage3Concurrency,
				"download_time_seconds_per_ip": cfg.DownloadTimeSeconds,
				"min_download_mbps":            cfg.MinSpeedMB,
			},
			"message": "文件测速完成。",
			"stage":   currentStage,
			"task_id": taskID,
		})
		completedStages = append(completedStages, currentStage)
		resultData = []utils.CloudflareIPData(speedData)
	}
	resultData = limitCloudflareResultData(resultData, cfg.PrintNum)

	outputFile := ""
	if len(resultData) > 0 {
		outputFile = currentOutputFile(cfg)
		if outputFile != "" {
			if err := utils.ExportCsv(resultData); err != nil {
				warnings = append(warnings, fmt.Sprintf("结果导出失败：%v", err))
				utils.DebugEvent("probe.export", map[string]any{
					"error":       err.Error(),
					"level":       "warn",
					"message":     "CSV 导出失败。",
					"reason":      "csv_export_failed",
					"target_path": outputFile,
					"task_id":     taskID,
				})
				outputFile = ""
			} else {
				if emitter != nil {
					emitter.emit("probe.partial_export", map[string]any{
						"target_path": outputFile,
						"written":     len(resultData),
					})
				}
				utils.DebugEvent("probe.export", map[string]any{
					"counts": map[string]any{
						"written": len(resultData),
					},
					"message":     "CSV 导出完成。",
					"target_path": outputFile,
					"task_id":     taskID,
					"tcp":         map[string]any{"delay_column": "TCP延迟(ms)"},
				})
			}
		}
	}

	rows := make([]ProbeRow, 0, len(resultData))
	for _, item := range resultData {
		rows = append(rows, convertProbeRow(item))
	}

	result := ProbeRunResult{
		Config:        cfg,
		DurationMS:    time.Since(start).Milliseconds(),
		OutputFile:    outputFile,
		Results:       rows,
		Source:        source,
		StartedAt:     start.Format(time.RFC3339),
		Summary:       summarizeProbeRows(rows, source.CandidateCount),
		Warnings:      dedupeStrings(warnings),
		SchemaVersion: guiSchemaVersion,
	}
	utils.DebugEvent("probe.complete", map[string]any{
		"counts": map[string]any{
			"exported": len(result.Results),
			"failed":   result.Summary.Failed,
			"passed":   result.Summary.Passed,
			"total":    result.Summary.Total,
		},
		"duration_ms":      result.DurationMS,
		"message":          "探测任务完成。",
		"output_file":      result.OutputFile,
		"completed_stages": completedStages,
		"task_id":          taskID,
		"warnings":         result.Warnings,
	})
	return result, nil
}

func logDesktopProbePreparationFailure(cfg ProbeConfig, taskID string, source SourceSummary, invalidCount int, statuses []DesktopSourceStatus, duration time.Duration, err error) {
	utils.Debug = cfg.Debug
	closeDebugLog, _ := configureProbeDebugRuntime(cfg)
	utils.SetDebugLogContext(taskID)
	defer closeDebugLog()

	utils.DebugEvent("probe.start", map[string]any{
		"config":  debugProbeConfigSummary(cfg),
		"message": "探测任务启动。",
		"source":  debugSourceSummary(source, statuses),
		"task_id": taskID,
	})
	utils.DebugEvent("stage.start", map[string]any{
		"message": "开始生成 IP 池。",
		"stage":   "stage0_pool",
		"task_id": taskID,
	})
	utils.DebugEvent("stage.complete", map[string]any{
		"counts":      debugStage0Counts(source, invalidCount),
		"duration_ms": duration.Milliseconds(),
		"message":     "IP 池生成失败。",
		"source":      debugSourceSummary(source, statuses),
		"stage":       "stage0_pool",
		"task_id":     taskID,
	})
	logProbeFailed(taskID, "stage0_pool", time.Now().Add(-duration), nil, err, false)
}

func logProbeFailed(taskID, stage string, startedAt time.Time, completedStages []string, err error, recoverable bool) {
	message := "探测任务失败。"
	errText := ""
	if err != nil {
		message = err.Error()
		errText = err.Error()
	}
	utils.DebugEvent("probe.failed", map[string]any{
		"completed_stages": completedStages,
		"duration_ms":      time.Since(startedAt).Milliseconds(),
		"error":            errText,
		"message":          message,
		"recoverable":      recoverable,
		"stage":            stage,
		"task_id":          taskID,
	})
}

func debugStageCounts(total, passed, failed int) map[string]any {
	if failed < 0 {
		failed = 0
	}
	return map[string]any{
		"failed": failed,
		"passed": passed,
		"total":  total,
	}
}

func debugStage0Counts(source SourceSummary, invalidCount int) map[string]any {
	total := source.CandidateCount
	if total == 0 {
		total = source.ValidCount + source.DuplicateCount + invalidCount
	}
	return map[string]any{
		"accepted": source.ValidCount,
		"filtered": source.DuplicateCount,
		"invalid":  invalidCount,
		"total":    total,
	}
}

func debugSourceSummary(source SourceSummary, statuses []DesktopSourceStatus) map[string]any {
	return map[string]any{
		"candidate_count": source.CandidateCount,
		"duplicate_count": source.DuplicateCount,
		"invalid_count":   source.InvalidCount,
		"raw_line_count":  source.RawLineCount,
		"source_statuses": statuses,
		"unique_count":    source.UniqueCount,
		"valid_count":     source.ValidCount,
	}
}

func debugProbeConfigSummary(cfg ProbeConfig) map[string]any {
	return map[string]any{
		"debug_capture_address":                  cfg.DebugCaptureAddress,
		"disable_download":                       cfg.DisableDownload,
		"download_count":                         cfg.TestCount,
		"download_concurrency":                   cfg.Stage3Concurrency,
		"download_speed_sample_interval_seconds": cfg.DownloadSpeedSampleIntervalSeconds,
		"download_time_seconds_per_ip":           cfg.DownloadTimeSeconds,
		"event_throttle_ms":                      cfg.EventThrottleMS,
		"export_append":                          cfg.ExportAppend,
		"cooldown_failures":                      cfg.CooldownFailures,
		"cooldown_ms":                            cfg.CooldownMS,
		"trace_concurrency":                      cfg.HeadRoutines,
		"trace_max_latency_ms":                   cfg.HeadMaxDelayMS,
		"trace_test_count":                       cfg.HeadTestCount,
		"trace_timeout_ms":                       cfg.Stage2TimeoutMS,
		"trace_url":                              cfg.TraceURL,
		"host_header":                            cfg.HostHeader,
		"httping_cf_colo":                        cfg.HttpingCFColo,
		"httping_status_code":                    cfg.HttpingStatusCode,
		"max_loss_rate":                          cfg.MaxLossRate,
		"max_tcp_latency_ms":                     cfg.MaxDelayMS,
		"min_delay_ms":                           cfg.MinDelayMS,
		"min_download_mbps":                      cfg.MinSpeedMB,
		"ping_times":                             cfg.PingTimes,
		"retry_backoff_ms":                       cfg.RetryBackoffMS,
		"retry_max_attempts":                     cfg.RetryMaxAttempts,
		"routines":                               cfg.Routines,
		"skip_first_latency_sample":              cfg.SkipFirstLatency,
		"stage1_limit":                           cfg.Stage1Limit,
		"tcp_timeout_ms":                         cfg.Stage1TimeoutMS,
		"sni":                                    cfg.SNI,
		"strategy":                               cfg.Strategy,
		"tcp_port":                               cfg.TCPPort,
		"url":                                    cfg.URL,
		"user_agent":                             cfg.UserAgent,
		"write_output":                           cfg.WriteOutput,
	}
}

func defaultProbeConfig() ProbeConfig {
	return ProbeConfig{
		Strategy:                           "fast",
		Routines:                           200,
		HeadRoutines:                       task.MaxTraceRoutines,
		PingTimes:                          4,
		SkipFirstLatency:                   true,
		EventThrottleMS:                    100,
		DownloadSpeedSampleIntervalSeconds: 2,
		HeadTestCount:                      64,
		TestCount:                          10,
		Stage1Limit:                        512,
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

const (
	maxDesktopTCPRoutines       = 1000
	maxDesktopStage3Routines    = task.MaxDownloadRoutines
	defaultDesktopSourceIPLimit = 500
)

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

func normalizeProbeConfig(cfg ProbeConfig) (ProbeConfig, []string) {
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
	} else if cfg.Routines > maxDesktopTCPRoutines {
		warn("TCP并发线程最大支持 %d，已改为 %d。", maxDesktopTCPRoutines, maxDesktopTCPRoutines)
		cfg.Routines = maxDesktopTCPRoutines
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
	if cfg.HeadTestCount <= 0 {
		warn("追踪候选上限必须大于 0，已改为 %d。", def.HeadTestCount)
		cfg.HeadTestCount = def.HeadTestCount
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
		warn("测速并发线程固定为 %d，已忽略配置值 %d。", maxDesktopStage3Routines, cfg.Stage3Concurrency)
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
	if cfg.DownloadTimeSeconds < 10 {
		warn("单 IP 下载测速时间必须至少为 10 秒，已改为 %d 秒。", def.DownloadTimeSeconds)
		cfg.DownloadTimeSeconds = def.DownloadTimeSeconds
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

func applyProbeConfig(cfg ProbeConfig) {
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
	utils.Output = currentOutputFile(cfg)
	utils.OutputAppend = cfg.ExportAppend
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

func normalizePathSelectionMode(mode string) string {
	mode = strings.ToLower(strings.TrimSpace(mode))
	mode = strings.ReplaceAll(mode, "-", "_")
	if mode == "" {
		return "source_file"
	}
	return mode
}

func selectPathDefaultDirectory(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	info, err := os.Stat(path)
	if err == nil && info.IsDir() {
		return path
	}
	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		return ""
	}
	if info, err := os.Stat(dir); err == nil && info.IsDir() {
		return dir
	}
	return ""
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

func applyStage1CandidateLimit(cfg ProbeConfig, source SourceSummary) (SourceSummary, []string) {
	if cfg.Stage1Limit <= 0 || source.ValidCount <= cfg.Stage1Limit {
		return source, nil
	}

	originalCount := source.ValidCount
	source.Valid = append([]string(nil), source.Valid[:cfg.Stage1Limit]...)
	source.ValidCount = len(source.Valid)
	source.UniqueCount = source.ValidCount
	return source, []string{fmt.Sprintf("阶段1候选上限为 %d，已从 %d 条候选中截取前 %d 条进行 TCP 探测。", cfg.Stage1Limit, originalCount, source.ValidCount)}
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
		TraceDelayMS:    item.HeadDelay.Seconds() * 1000,
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
	return candidateCount
}

func limitPingDelaySet(ipSet utils.PingDelaySet, limit int) utils.PingDelaySet {
	if limit <= 0 || len(ipSet) <= limit {
		return ipSet
	}
	return ipSet[:limit]
}

func limitCloudflareResultData(data []utils.CloudflareIPData, limit int) []utils.CloudflareIPData {
	return utils.SelectTopWeightedResults(data, limit)
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

func desktopCommandResult(code string, data any, message string, ok bool, taskID *string, warnings []string) DesktopCommandResult {
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

func defaultDesktopConfigSnapshot() map[string]any {
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
			"httping":                                false,
			"httping_cf_colo":                        "",
			"httping_status_code":                    0,
			"max_loss_rate":                          float64(utils.MaxAllowedLossRate),
			"min_delay_ms":                           0,
			"ping_times":                             4,
			"print_num":                              10,
			"retry_policy": map[string]any{
				"backoff_ms":   0,
				"max_attempts": 0,
			},
			"skip_first_latency_sample": true,
			"stage_limits": map[string]any{
				"stage1": 512,
				"stage2": 64,
				"stage3": 10,
			},
			"strategy":    "fast",
			"host_header": "",
			"sni":         "",
			"tcp_port":    443,
			"test_all":    false,
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
				"ip_limit":           defaultDesktopSourceIPLimit,
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

func loadDesktopConfigSnapshotFromDisk() (map[string]any, error) {
	path := desktopConfigFilePath()
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return defaultDesktopConfigSnapshot(), nil
		}
		return nil, err
	}
	var saved map[string]any
	if err := json.Unmarshal(raw, &saved); err != nil {
		return nil, err
	}
	if snapshot := mapValue(saved["config_snapshot"]); len(snapshot) > 0 {
		return snapshot, nil
	}
	return saved, nil
}

func writeDesktopConfigSnapshot(path string, snapshot map[string]any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	body := map[string]any{
		"config_snapshot": snapshot,
		"saved_at":        time.Now().Format(time.RFC3339),
		"schema_version":  guiSchemaVersion,
	}
	raw, err := json.MarshalIndent(body, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o600)
}

func desktopConfigToProbeConfig(config map[string]any) (ProbeConfig, []string) {
	cfg := defaultProbeConfig()
	probe := mapValue(config["probe"])
	exportCfg := mapValue(config["export"])
	concurrency := mapValue(probe["concurrency"])
	stageLimits := mapValue(firstNonNil(probe["stage_limits"], probe["stageLimits"]))
	thresholds := mapValue(probe["thresholds"])
	timeouts := mapValue(probe["timeouts"])
	cooldownPolicy := mapValue(firstNonNil(probe["cooldown_policy"], probe["cooldownPolicy"]))
	retryPolicy := mapValue(firstNonNil(probe["retry_policy"], probe["retryPolicy"]))
	warnings := make([]string, 0)
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
	cfg.TestCount = intValue(firstNonNil(probe["download_count"], probe["downloadCount"], stageLimits["stage3"]), cfg.TestCount)
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

	switch strategy {
	case "fast":
		cfg.MinSpeedMB = 0
	case "full":
		cfg.DisableDownload = false
	}

	if fileName := desktopExportFileName(exportCfg, "", activeProfileName(), time.Now()); fileName != "" {
		cfg.OutputFile = desktopExportPath(exportCfg, fileName)
		cfg.WriteOutput = true
	}
	cfg.ExportAppend = strings.EqualFold(strings.TrimSpace(stringValue(exportCfg["overwrite"], "")), "append")

	normalized, normalizeWarnings := normalizeProbeConfig(cfg)
	warnings = append(warnings, normalizeWarnings...)
	return normalized, dedupeStrings(warnings)
}

func applyDesktopExportConfig(cfg ProbeConfig, config map[string]any, taskID string) ProbeConfig {
	exportCfg := mapValue(config["export"])
	if len(exportCfg) == 0 {
		return cfg
	}
	if fileName := desktopExportFileName(exportCfg, taskID, activeProfileName(), time.Now()); fileName != "" {
		cfg.OutputFile = desktopExportPath(exportCfg, fileName)
		cfg.WriteOutput = true
	}
	return cfg
}

func desktopExportFileName(exportCfg map[string]any, taskID, profileName string, now time.Time) string {
	if template := strings.TrimSpace(stringValue(firstNonNil(exportCfg["file_name_template"], exportCfg["fileNameTemplate"]), "")); template != "" {
		if fileName := renderExportFileTemplate(template, taskID, profileName, now); fileName != "" {
			return fileName
		}
	}
	return sanitizeTemplateFileName(stringValue(firstNonNil(exportCfg["file_name"], exportCfg["fileName"]), ""))
}

func desktopExportPath(exportCfg map[string]any, fileName string) string {
	targetDir := strings.TrimSpace(stringValue(firstNonNil(exportCfg["target_dir"], exportCfg["targetDir"]), ""))
	if targetDir == "" {
		targetDir = storageRoot()
	}
	return filepath.Join(targetDir, fileName)
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
		return defaultDesktopSourceIPLimit
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

	var saved map[string]any
	if err := json.Unmarshal(raw, &saved); err != nil {
		return err
	}

	snapshot := mapValue(saved["config_snapshot"])
	if len(snapshot) == 0 {
		snapshot = saved
	}
	sourceItems, ok := snapshot["sources"].([]any)
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
	body := map[string]any{
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

func mapValue(value any) map[string]any {
	if typed, ok := value.(map[string]any); ok {
		return typed
	}
	return map[string]any{}
}

func firstNonNil(values ...any) any {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func intValue(value any, fallback int) int {
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

func floatValue(value any, fallback float64) float64 {
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

func boolValue(value any, fallback bool) bool {
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

func stringValue(value any, fallback string) string {
	if value == nil {
		return fallback
	}
	if typed, ok := value.(string); ok {
		return typed
	}
	return fmt.Sprint(value)
}

func configFilePath() string {
	return filepath.Join(storageRoot(), "config.json")
}

func desktopConfigFilePath() string {
	return filepath.Join(storageRoot(), "desktop-config.json")
}

func debugLogFilePath() string {
	return filepath.Join(storageRoot(), "cfip-log.txt")
}
