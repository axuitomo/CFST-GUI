package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/appcore"
	"github.com/axuitomo/CFST-GUI/internal/httpcfg"
	"github.com/axuitomo/CFST-GUI/internal/probecore"
	"github.com/axuitomo/CFST-GUI/task"
	"github.com/axuitomo/CFST-GUI/utils"
)

const guiSchemaVersion = "cfst-gui-wails-v1"
const probeAlreadyRunningMessage = "当前已有探测任务运行或暂停，请完成后再启动新任务。"
const defaultFileTestURL = probecore.DefaultFileTestURL
const (
	defaultPortPolicy                   = probecore.PortPolicySourceOverrideGlobal
	defaultThemeMode                    = "auto_system_time"
	defaultThemeLightStart              = "07:00"
	defaultThemeDarkStart               = "19:00"
	defaultSchedulerConfigSource        = "draft_preferred"
	defaultSchedulerProfileAction       = "update_recent_run_profile"
	defaultSchedulerSourceProfileAction = "update_recent_run_source_profile"
	recentRunProfileID                  = "profile-recent-run"
	recentRunSourceProfileID            = "source-profile-recent-run"
	recentRunProfileName                = "最近运行档案"
	recentRunSourceProfileName          = "最近运行输入源"
)

type App struct {
	ctx context.Context

	runMu    sync.Mutex
	eventHub *webUIEventHub

	schedulerMu     sync.Mutex
	schedulerCancel context.CancelFunc
	schedulerStatus SchedulerStatus

	trayStartOnce sync.Once
	trayStopOnce  sync.Once
	trayMu        sync.Mutex
	trayAvailable bool
	quitting      bool

	probeControlMu    sync.Mutex
	currentTaskID     string
	pausedTaskID      string
	pauseRequested    bool
	pauseCond         *sync.Cond
	pauseEmitter      *desktopProbeEmitter
	downloadCancel    func()
	downloadCancelSeq int64
}

type ProbeConfig = probecore.ProbeConfig

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

type DesktopCommandResult = appcore.CommandResult

type HealthResult struct {
	ConfigPath     string `json:"configPath"`
	Online         bool   `json:"online"`
	SchemaVersion  string `json:"schemaVersion"`
	Service        string `json:"service"`
	Version        string `json:"version"`
	WailsTransport string `json:"wailsTransport"`
}

type SourceSummary = probecore.SourceSummary

type ProbeRequest struct {
	Config            ProbeConfig              `json:"config"`
	ConfigWarnings    []string                 `json:"configWarnings,omitempty"`
	DisableExport     bool                     `json:"-"`
	DisableDebugLog   bool                     `json:"-"`
	SourcePorts       map[string]int           `json:"-"`
	TaskContext       ProbeTaskContext         `json:"taskContext,omitempty"`
	SourceStatuses    []DesktopSourceStatus    `json:"sourceStatuses,omitempty"`
	SourceColoFilters task.SourceColoFilterMap `json:"-"`
	SourceText        string                   `json:"sourceText"`
	TaskID            string                   `json:"taskId,omitempty"`
}

type DesktopProbePayload = appcore.ProbePayload
type DesktopSource = appcore.Source
type DesktopSourceStatus = appcore.SourceStatus

type UploadSelectionConfig struct {
	SharedFilter   UploadSharedFilterConfig `json:"shared_filter"`
	CloudflareTopN int                      `json:"cloudflare_top_n"`
	GitHubTopN     int                      `json:"github_top_n"`
}

type UploadSharedFilterConfig struct {
	Enabled           bool     `json:"enabled"`
	Status            string   `json:"status"`
	IPVersion         string   `json:"ip_version"`
	ColoAllow         []string `json:"colo_allow"`
	ColoDeny          []string `json:"colo_deny"`
	MaxTCPLatencyMS   *float64 `json:"max_tcp_latency_ms"`
	MaxTraceLatencyMS *float64 `json:"max_trace_latency_ms"`
	MinDownloadMBPS   float64  `json:"min_download_mbps"`
	MaxLossRate       *float64 `json:"max_loss_rate"`
}

type UploadSelectionResult struct {
	InputRows      []ProbeRow `json:"input_rows"`
	FilteredRows   []ProbeRow `json:"filtered_rows"`
	CloudflareRows []ProbeRow `json:"cloudflare_rows"`
	GitHubRows     []ProbeRow `json:"github_rows"`
	Warnings       []string   `json:"warnings"`
}

type desktopSourceContentResult = appcore.SourceContentResult

type preparedDesktopSources = appcore.PreparedSources

type ProbeTaskContext = probecore.TaskContext

type ProbeRunResult = appcore.ProbeRunResult

type ProbeSummary = probecore.ProbeSummary

type ProbeRow = probecore.ProbeRow

type ProbeResultRow = appcore.ProbeResultRow

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
	app := &App{
		eventHub: newWebUIEventHub(),
	}
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
	a.reloadSchedulerFromDisk()
}

func (a *App) GetHealth() HealthResult {
	return HealthResult{
		ConfigPath:     configFilePath(),
		Online:         true,
		SchemaVersion:  guiSchemaVersion,
		Service:        "CFST Wails Bridge",
		Version:        appVersion(),
		WailsTransport: "window.go.app.App",
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
		a.scheduleQuitAfterUpdate()
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
			sourceProfiles, sourceProfileErr := loadSourceProfileStoreForSnapshot(snapshot)
			if sourceProfileErr != nil {
				warnings = append(warnings, fmt.Sprintf("读取输入源配置档案失败：%v", sourceProfileErr))
			}
			return desktopCommandResult("CONFIG_READY", map[string]any{
				"configPath":      path,
				"config_snapshot": snapshot,
				"draft_status":    desktopDraftStatusPayload(),
				"profiles":        profiles,
				"source_profiles": sourceProfiles,
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
		snapshot = sanitizeDesktopConfigSnapshot(value)
	} else {
		snapshot = sanitizeDesktopConfigSnapshot(saved)
	}
	sourceProfiles, sourceProfileErr := loadSourceProfileStoreForSnapshot(snapshot)
	if sourceProfileErr != nil {
		warnings = append(warnings, fmt.Sprintf("读取输入源配置档案失败：%v", sourceProfileErr))
	}
	_, configWarnings := desktopConfigToProbeConfig(snapshot)
	warnings = append(warnings, configWarnings...)

	return desktopCommandResult("CONFIG_READ_OK", map[string]any{
		"configPath":      path,
		"config_snapshot": snapshot,
		"draft_status":    desktopDraftStatusPayload(),
		"profiles":        profiles,
		"source_profiles": sourceProfiles,
		"storage":         storage,
	}, "配置已加载。", true, nil, warnings)
}

func (a *App) LoadDesktopDraft() DesktopCommandResult {
	return desktopCommandResult("DESKTOP_DRAFT_READY", desktopDraftStatusPayload(), "桌面草稿状态已读取。", true, nil, nil)
}

func (a *App) SaveDesktopDraft(payload map[string]any) DesktopCommandResult {
	snapshot := mapValue(firstNonNil(payload["config_snapshot"], payload["configSnapshot"]))
	if len(snapshot) == 0 {
		return desktopCommandResult("DESKTOP_DRAFT_INVALID", nil, "缺少 config_snapshot。", false, nil, nil)
	}
	if err := writeDesktopConfigSnapshot(desktopDraftFilePath(), snapshot); err != nil {
		return desktopCommandResult("DESKTOP_DRAFT_WRITE_FAILED", nil, err.Error(), false, nil, nil)
	}
	return desktopCommandResult("DESKTOP_DRAFT_SAVE_OK", desktopDraftStatusPayload(), "桌面草稿已保存。", true, nil, nil)
}

func (a *App) DiscardDesktopDraft(payload map[string]any) DesktopCommandResult {
	if err := removeDesktopDraft(); err != nil {
		return desktopCommandResult("DESKTOP_DRAFT_DISCARD_FAILED", desktopDraftStatusPayload(), err.Error(), false, nil, nil)
	}
	return desktopCommandResult("DESKTOP_DRAFT_DISCARDED", desktopDraftStatusPayload(), "桌面草稿已丢弃。", true, nil, nil)
}

func (a *App) SaveDesktopConfig(payload map[string]any) DesktopCommandResult {
	path := desktopConfigFilePath()
	snapshot, ok := payload["config_snapshot"].(map[string]any)
	if !ok {
		return desktopCommandResult("CONFIG_INVALID", nil, "缺少 config_snapshot。", false, nil, nil)
	}
	snapshot = sanitizeDesktopConfigSnapshot(snapshot)

	if err := writeDesktopConfigSnapshot(path, snapshot); err != nil {
		return desktopCommandResult("CONFIG_WRITE_FAILED", nil, err.Error(), false, nil, nil)
	}
	if err := removeDesktopDraft(); err != nil {
		return desktopCommandResult("CONFIG_WRITE_FAILED", nil, fmt.Sprintf("配置已保存，但清理草稿失败：%v", err), false, nil, nil)
	}
	_, warnings := desktopConfigToProbeConfig(snapshot)
	profiles, profileErr := loadProfileStore()
	if profileErr != nil {
		warnings = append(warnings, fmt.Sprintf("读取配置档案失败：%v", profileErr))
	}
	sourceProfiles, sourceProfileErr := loadSourceProfileStoreForSnapshot(snapshot)
	if sourceProfileErr != nil {
		warnings = append(warnings, fmt.Sprintf("读取输入源配置档案失败：%v", sourceProfileErr))
	}
	a.reloadSchedulerFromSnapshot(snapshot)

	return desktopCommandResult("CONFIG_SAVE_OK", map[string]any{
		"configPath":      path,
		"config_snapshot": snapshot,
		"draft_status":    desktopDraftStatusPayload(),
		"profiles":        profiles,
		"source_profiles": sourceProfiles,
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
	if ok, _ := a.setCurrentProbeTask(taskID, emitter); !ok {
		return ProbeRunResult{}, errors.New(probeAlreadyRunningMessage)
	}
	defer a.clearCurrentProbeTask(taskID)

	prepareStart := time.Now()
	prepared := prepareDesktopSources(cfg, payload.Sources)
	if err := persistDesktopSourceStatuses(prepared.SourceStatuses); err != nil {
		prepared.Warnings = append(prepared.Warnings, fmt.Sprintf("更新输入源状态失败：%v", err))
	}
	preparedSummary := summarizeSource(prepared.Text)
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
	if len(prepared.FatalErrors) > 0 {
		err := errors.New(strings.Join(prepared.FatalErrors, "；"))
		logDesktopProbePreparationFailure(cfg, taskID, preparedSummary, preparedInvalidCount, prepared.SourceStatuses, time.Since(prepareStart), err)
		emitter.emit("probe.failed", withDebugLogPath(map[string]any{
			"message":     err.Error(),
			"recoverable": false,
		}, debugLogPathForProbeConfig(cfg)))
		return ProbeRunResult{}, err
	}
	if strings.TrimSpace(prepared.Text) == "" && len(prepared.Warnings) > 0 {
		err := errors.New(strings.Join(prepared.Warnings, "；"))
		logDesktopProbePreparationFailure(cfg, taskID, preparedSummary, preparedInvalidCount, prepared.SourceStatuses, time.Since(prepareStart), err)
		emitter.emit("probe.failed", withDebugLogPath(map[string]any{
			"message":     err.Error(),
			"recoverable": false,
		}, debugLogPathForProbeConfig(cfg)))
		return ProbeRunResult{}, err
	}
	configSource := strings.TrimSpace(payload.ConfigSource)
	taskContext, portWarnings := probeTaskContextForPorts(cfg, prepared.SourcePorts)
	taskContext.ConfigSource = configSource
	prepared.Warnings = append(prepared.Warnings, portWarnings...)
	portGroups := probecore.PortGroups(preparedSummary.Valid, prepared.SourcePorts, cfg.TCPPort, cfg.PortPolicy)
	if cfg.PortPolicy == probecore.PortPolicySourceOverrideGlobal && len(portGroups) > 1 {
		prepared.Warnings = append(prepared.Warnings, fmt.Sprintf("输入源端口已按 %d 个测试端口分组执行：%v。", len(portGroups), portGroupPorts(portGroups)))
	}
	result, err := a.runDesktopProbePortGroups(cfg, configWarnings, taskContext, prepared, preparedSummary, taskID, portGroups, emitter)
	if err != nil {
		debugLogPath := result.DebugLogPath
		if debugLogPath == "" {
			debugLogPath = debugLogPathForProbeConfig(cfg)
		}
		payload := map[string]any{
			"message":     err.Error(),
			"recoverable": false,
		}
		if strings.TrimSpace(result.FailureStage) != "" {
			payload["failure_stage"] = strings.TrimSpace(result.FailureStage)
		}
		if len(result.TraceDiagnostics) > 0 {
			payload["trace_diagnostics"] = result.TraceDiagnostics
		}
		emitter.emit("probe.failed", withDebugLogPath(payload, debugLogPath))
		return ProbeRunResult{}, err
	}
	result.SourceStatuses = prepared.SourceStatuses
	result.Warnings = dedupeStrings(append(result.Warnings, prepared.Warnings...))
	exportedCount := 0
	if strings.TrimSpace(result.OutputFile) != "" && len(result.Results) > 0 {
		exportedCount = len(result.Results)
	}
	emitter.emit("probe.completed", withDebugLogPath(map[string]any{
		"exported": exportedCount,
		"failed":   result.Summary.Failed,
		"failure_summary": map[string]any{
			"duplicate_count": preparedSummary.DuplicateCount,
			"invalid_count":   preparedInvalidCount,
		},
		"failure_stage":     result.FailureStage,
		"passed":            result.Summary.Passed,
		"result_count":      len(result.Results),
		"task_context":      result.TaskContext,
		"target_path":       result.OutputFile,
		"trace_diagnostics": result.TraceDiagnostics,
	}, result.DebugLogPath))
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
	downloadCancel := a.downloadCancel
	if a.pauseCond != nil {
		a.pauseCond.Broadcast()
	}
	a.probeControlMu.Unlock()

	if downloadCancel != nil {
		downloadCancel()
	}
	if emitter != nil {
		emitter.emit("probe.cooling", map[string]any{
			"reason":      "已收到暂停请求，正在暂停当前测速进程。",
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

func (a *App) ListResultFile(payload map[string]any) DesktopCommandResult {
	config := mapValue(firstNonNil(payload["config"], payload["config_snapshot"], payload["configSnapshot"]))
	cfg, _ := desktopConfigToProbeConfig(config)
	taskID := strings.TrimSpace(stringValue(firstNonNil(payload["task_id"], payload["taskId"]), ""))
	sourcePath := resolveDesktopResultFilePath(payload, cfg)
	rows, err := readProbeResultRowsFromCSV(sourcePath)
	if err != nil {
		return desktopCommandResult("RESULT_FILE_UNAVAILABLE", nil, err.Error(), false, &taskID, nil)
	}
	data := map[string]any{
		"count":       len(rows),
		"results":     rows,
		"source_path": sourcePath,
	}
	return desktopCommandResult("RESULT_FILE_LISTED", data, "已从结果文件读取当前结果。", true, &taskID, nil)
}

func (a *App) setCurrentProbeTask(taskID string, emitter *desktopProbeEmitter) (bool, string) {
	a.ensureProbeControl()
	a.probeControlMu.Lock()
	defer a.probeControlMu.Unlock()
	if a.currentTaskID != "" {
		return false, a.currentTaskID
	}
	a.currentTaskID = taskID
	a.pausedTaskID = ""
	a.pauseRequested = false
	a.pauseEmitter = emitter
	a.downloadCancel = nil
	if a.pauseCond != nil {
		a.pauseCond.Broadcast()
	}
	return true, taskID
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
		a.downloadCancel = nil
		if a.pauseCond != nil {
			a.pauseCond.Broadcast()
		}
	}
}

func probeTaskContextForPorts(cfg ProbeConfig, sourcePorts map[string]int) (ProbeTaskContext, []string) {
	return probecore.TaskContextForPorts(cfg.TCPPort, sourcePorts, cfg.PortPolicy)
}

func probePortGroups(ips []string, sourcePorts map[string]int, globalPort int, portPolicy string) []probecore.PortGroup {
	return probecore.PortGroups(ips, sourcePorts, globalPort, portPolicy)
}

func portGroupPorts(groups []probecore.PortGroup) []int {
	return probecore.PortGroupPorts(groups)
}

func (a *App) runDesktopProbePortGroups(cfg ProbeConfig, configWarnings []string, taskContext ProbeTaskContext, prepared preparedDesktopSources, preparedSummary SourceSummary, taskID string, groups []probecore.PortGroup, emitter *desktopProbeEmitter) (ProbeRunResult, error) {
	workflowResult, err := probecore.RunProbeWorkflow(probecore.WorkflowRunRequest{
		Config: probecore.WorkflowConfig{
			DownloadSpeedMetric: cfg.DownloadSpeedMetric,
			PrintNum:            cfg.PrintNum,
			TCPPort:             cfg.TCPPort,
		},
		Groups:      groups,
		SourcePorts: prepared.SourcePorts,
		Source: probecore.WorkflowSource{
			Summary:  preparedSummary,
			Text:     prepared.Text,
			Warnings: prepared.Warnings,
		},
		TaskContext: taskContext,
		TaskID:      taskID,
	}, probecore.WorkflowAdapter{
		BeginMultiGroup: func(probecore.WorkflowRunRequest) (probecore.WorkflowLifecycle, error) {
			start := time.Now()
			closeDebugLog, debugWarnings, debugLogPath := configureProbeDebugRuntime(cfg)
			utils.SetDebugLogContext(taskID)
			return probecore.WorkflowLifecycle{
				DebugLogPath: debugLogPath,
				StartedAt:    start,
				Warnings:     debugWarnings,
				Close:        closeDebugLog,
			}, nil
		},
		Export: func(req probecore.WorkflowExportRequest) (probecore.WorkflowExportResult, error) {
			outputFile := currentOutputFile(cfg)
			if outputFile == "" {
				return probecore.WorkflowExportResult{}, nil
			}
			applyProbeConfig(cfg)
			if exportErr := utils.ExportCsv(req.RawResults); exportErr != nil {
				return probecore.WorkflowExportResult{
					Warnings: []string{fmt.Sprintf("结果导出失败：%v", exportErr)},
				}, nil
			}
			if emitter != nil {
				emitter.emit("probe.partial_export", withDebugLogPath(map[string]any{
					"target_path": outputFile,
					"written":     len(req.RawResults),
				}, req.DebugLogPath))
			}
			return probecore.WorkflowExportResult{OutputFile: outputFile}, nil
		},
		RunGroup: func(req probecore.WorkflowGroupRequest) (probecore.WorkflowGroupResult, error) {
			groupCfg := cfg
			if req.Group.Port > 0 {
				groupCfg.TCPPort = req.Group.Port
			}
			groupResult, groupErr := a.runProbe(ProbeRequest{
				ConfigWarnings:    configWarnings,
				Config:            groupCfg,
				DisableDebugLog:   req.DisableDebugLog,
				DisableExport:     req.DisableExport,
				SourcePorts:       prepared.SourcePorts,
				TaskContext:       req.TaskContext,
				SourceColoFilters: prepared.SourceColoFilters,
				SourceStatuses:    prepared.SourceStatuses,
				SourceText:        req.SourceText,
				TaskID:            req.TaskID,
			}, emitter)
			return probecore.WorkflowGroupResult{
				DebugLogPath:     groupResult.DebugLogPath,
				DurationMS:       groupResult.DurationMS,
				FailureStage:     groupResult.FailureStage,
				OutputFile:       groupResult.OutputFile,
				RawResults:       groupResult.RawResults,
				Results:          groupResult.Results,
				Source:           groupResult.Source,
				StartedAt:        groupResult.StartedAt,
				Summary:          groupResult.Summary,
				TaskContext:      groupResult.TaskContext,
				TraceDiagnostics: groupResult.TraceDiagnostics,
				Warnings:         groupResult.Warnings,
			}, groupErr
		},
	})
	resultCfg := cfg
	if len(groups) == 1 && groups[0].Port > 0 {
		resultCfg.TCPPort = groups[0].Port
	}
	result := ProbeRunResult{
		Config:           resultCfg,
		DebugLogPath:     workflowResult.DebugLogPath,
		DurationMS:       workflowResult.DurationMS,
		FailureStage:     workflowResult.FailureStage,
		OutputFile:       workflowResult.OutputFile,
		Results:          workflowResult.Results,
		Source:           workflowResult.Source,
		SourceStatuses:   prepared.SourceStatuses,
		StartedAt:        workflowResult.StartedAt,
		Summary:          workflowResult.Summary,
		TaskContext:      workflowResult.TaskContext,
		TraceDiagnostics: workflowResult.TraceDiagnostics,
		Warnings:         dedupeStrings(workflowResult.Warnings),
		SchemaVersion:    guiSchemaVersion,
		RawResults:       workflowResult.RawResults,
	}
	return result, err
}

func setSnapshotTCPPort(snapshot map[string]any, port int) map[string]any {
	if len(snapshot) == 0 || port <= 0 {
		return snapshot
	}
	probe := mapValue(snapshot["probe"])
	if len(probe) == 0 {
		probe = map[string]any{}
	}
	probe["tcp_port"] = port
	snapshot["probe"] = probe
	return snapshot
}

func (a *App) registerDownloadInterrupt(taskID, stage, ip string, interrupt func()) func() {
	a.ensureProbeControl()
	a.probeControlMu.Lock()
	if a.currentTaskID == taskID && stage == task.DownloadSpeedSampleStage {
		a.downloadCancelSeq++
		seq := a.downloadCancelSeq
		a.downloadCancel = interrupt
		if a.pauseRequested && a.pausedTaskID == taskID && interrupt != nil {
			go interrupt()
		}
		a.probeControlMu.Unlock()
		return func() {
			a.probeControlMu.Lock()
			if a.currentTaskID == taskID && a.downloadCancelSeq == seq {
				a.downloadCancel = nil
			}
			a.probeControlMu.Unlock()
		}
	}
	a.probeControlMu.Unlock()
	return func() {}
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
	} else {
		snapshot = sanitizeDesktopConfigSnapshot(snapshot)
	}
	profiles, err := loadProfileStore()
	if err != nil {
		return desktopCommandResult("CONFIG_EXPORT_PROFILE_FAILED", nil, err.Error(), false, nil, nil)
	}
	sourceProfiles, err := loadSourceProfileStoreForSnapshot(snapshot)
	if err != nil {
		return desktopCommandResult("CONFIG_EXPORT_SOURCE_PROFILE_FAILED", nil, err.Error(), false, nil, nil)
	}
	body := map[string]any{
		"app_version":     version,
		"config_snapshot": snapshot,
		"exported_at":     time.Now().Format(time.RFC3339),
		"profiles":        profiles,
		"source_profiles": sourceProfiles,
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
	} else {
		snapshot = sanitizeDesktopConfigSnapshot(snapshot)
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
	snapshot = sanitizeDesktopConfigSnapshot(snapshot)
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

func (a *App) UpdateCurrentProfile(payload map[string]any) DesktopCommandResult {
	snapshot := mapValue(firstNonNil(payload["config_snapshot"], payload["configSnapshot"]))
	if len(snapshot) == 0 {
		return desktopCommandResult("PROFILE_INVALID", nil, "缺少 config_snapshot。", false, nil, nil)
	}
	snapshot = sanitizeDesktopConfigSnapshot(snapshot)
	store, err := loadProfileStore()
	if err != nil {
		return desktopCommandResult("PROFILE_LOAD_FAILED", nil, err.Error(), false, nil, nil)
	}
	now := time.Now().Format(time.RFC3339)
	profileID := strings.TrimSpace(stringValue(firstNonNil(payload["profile_id"], payload["profileId"], payload["id"], store.ActiveProfileID), ""))
	name := strings.TrimSpace(stringValue(payload["name"], ""))
	store, _ = probecore.UpdateCurrentProfileStore(probecore.CurrentProfileUpdateOptions[profileStore, profileItem, map[string]any]{
		Store:       store,
		Value:       snapshot,
		ProfileID:   profileID,
		Name:        name,
		Now:         now,
		DefaultName: "当前配置",
		Items: func(store profileStore) []profileItem {
			return store.Items
		},
		SetItems: func(store *profileStore, items []profileItem) {
			store.Items = items
		},
		ActiveID: func(store profileStore) string {
			return store.ActiveProfileID
		},
		SetActiveID: func(store *profileStore, profileID string) {
			store.ActiveProfileID = profileID
		},
		ItemID: func(item profileItem) string {
			return item.ID
		},
		UpdateItem: func(item *profileItem, patch probecore.ProfileItemPatch[map[string]any]) {
			item.ConfigSnapshot = patch.Value
			if patch.Name != "" {
				item.Name = patch.Name
			}
			if strings.TrimSpace(item.Name) == "" {
				item.Name = "当前配置"
			}
			if item.CreatedAt == "" {
				item.CreatedAt = patch.Now
			}
			item.UpdatedAt = patch.Now
		},
		NewItem: func(patch probecore.ProfileItemPatch[map[string]any]) profileItem {
			return profileItem{
				ConfigSnapshot: patch.Value,
				CreatedAt:      patch.Now,
				ID:             patch.ID,
				Name:           patch.Name,
				UpdatedAt:      patch.Now,
			}
		},
		NewProfileID: func() string {
			return fmt.Sprintf("profile-%d", time.Now().UnixNano())
		},
	})
	if err := saveProfileStore(store); err != nil {
		return desktopCommandResult("PROFILE_SAVE_FAILED", nil, err.Error(), false, nil, nil)
	}
	return desktopCommandResult("PROFILE_UPDATE_OK", store, "当前配置档案已更新并保存。", true, nil, nil)
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
		snapshot := sanitizeDesktopConfigSnapshot(item.ConfigSnapshot)
		return desktopCommandResult("PROFILE_SWITCH_OK", map[string]any{
			"configPath":      desktopConfigFilePath(),
			"config_snapshot": snapshot,
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

func (a *App) LoadSourceProfiles() DesktopCommandResult {
	snapshot, err := loadDesktopConfigSnapshotFromDisk()
	if err != nil {
		return desktopCommandResult("SOURCE_PROFILE_LOAD_FAILED", nil, err.Error(), false, nil, nil)
	}
	store, err := loadSourceProfileStoreForSnapshot(snapshot)
	if err != nil {
		return desktopCommandResult("SOURCE_PROFILE_LOAD_FAILED", nil, err.Error(), false, nil, nil)
	}
	return desktopCommandResult("SOURCE_PROFILE_LOAD_OK", store, "输入源配置档案已加载。", true, nil, nil)
}

func (a *App) SaveSourceProfile(payload map[string]any) DesktopCommandResult {
	snapshot, err := loadDesktopConfigSnapshotFromDisk()
	if err != nil {
		return desktopCommandResult("SOURCE_PROFILE_LOAD_FAILED", nil, err.Error(), false, nil, nil)
	}
	store, err := loadSourceProfileStoreForSnapshot(snapshot)
	if err != nil {
		return desktopCommandResult("SOURCE_PROFILE_LOAD_FAILED", nil, err.Error(), false, nil, nil)
	}

	sources := desktopSourcesFromAny(firstNonNil(payload["sources"], payload["Sources"]))
	name := strings.TrimSpace(stringValue(payload["name"], ""))
	profileID := strings.TrimSpace(stringValue(firstNonNil(payload["profile_id"], payload["profileId"], payload["id"]), ""))
	if name == "" {
		name = "输入源档案"
	}
	if profileID == "" {
		profileID = fmt.Sprintf("source-profile-%d", time.Now().UnixNano())
	}
	if profileID != defaultSourceProfileID && isBlankDefaultSourceProfilePlaceholder(store) {
		store.Items = []sourceProfileItem{}
	}

	now := time.Now().Format(time.RFC3339)
	updated := false
	for index := range store.Items {
		if store.Items[index].ID != profileID {
			continue
		}
		store.Items[index].Name = name
		store.Items[index].Sources = cloneDesktopSources(sources)
		if store.Items[index].CreatedAt == "" {
			store.Items[index].CreatedAt = now
		}
		store.Items[index].UpdatedAt = now
		updated = true
		break
	}
	if !updated {
		store.Items = append(store.Items, sourceProfileItem{
			CreatedAt: now,
			ID:        profileID,
			Name:      name,
			Sources:   cloneDesktopSources(sources),
			UpdatedAt: now,
		})
	}
	setActive := boolValue(firstNonNil(payload["set_active"], payload["setActive"]), true)
	if setActive {
		store.ActiveProfileID = profileID
	}
	if err := saveSourceProfileStore(store); err != nil {
		return desktopCommandResult("SOURCE_PROFILE_SAVE_FAILED", nil, err.Error(), false, nil, nil)
	}
	if setActive {
		snapshot["sources"] = cloneDesktopSources(sources)
		if err := writeDesktopConfigSnapshot(desktopConfigFilePath(), snapshot); err != nil {
			return desktopCommandResult("SOURCE_PROFILE_SAVE_FAILED", nil, err.Error(), false, nil, nil)
		}
	}
	return desktopCommandResult("SOURCE_PROFILE_SAVE_OK", store, "输入源配置档案已保存。", true, nil, nil)
}

func (a *App) UpdateCurrentSourceProfile(payload map[string]any) DesktopCommandResult {
	snapshot, err := loadDesktopConfigSnapshotFromDisk()
	if err != nil {
		return desktopCommandResult("SOURCE_PROFILE_LOAD_FAILED", nil, err.Error(), false, nil, nil)
	}
	store, err := loadSourceProfileStoreForSnapshot(snapshot)
	if err != nil {
		return desktopCommandResult("SOURCE_PROFILE_LOAD_FAILED", nil, err.Error(), false, nil, nil)
	}
	sources := desktopSourcesFromAny(firstNonNil(payload["sources"], payload["Sources"], snapshot["sources"]))
	now := time.Now().Format(time.RFC3339)
	profileID := strings.TrimSpace(stringValue(firstNonNil(payload["profile_id"], payload["profileId"], payload["id"], store.ActiveProfileID), ""))
	name := strings.TrimSpace(stringValue(payload["name"], ""))
	store, _ = probecore.UpdateCurrentProfileStore(probecore.CurrentProfileUpdateOptions[sourceProfileStore, sourceProfileItem, []DesktopSource]{
		Store:       store,
		Value:       cloneDesktopSources(sources),
		ProfileID:   profileID,
		Name:        name,
		Now:         now,
		DefaultName: "当前输入源",
		Items: func(store sourceProfileStore) []sourceProfileItem {
			return store.Items
		},
		SetItems: func(store *sourceProfileStore, items []sourceProfileItem) {
			store.Items = items
		},
		ActiveID: func(store sourceProfileStore) string {
			return store.ActiveProfileID
		},
		SetActiveID: func(store *sourceProfileStore, profileID string) {
			store.ActiveProfileID = profileID
		},
		ItemID: func(item sourceProfileItem) string {
			return item.ID
		},
		UpdateItem: func(item *sourceProfileItem, patch probecore.ProfileItemPatch[[]DesktopSource]) {
			if patch.Name != "" {
				item.Name = patch.Name
			}
			if strings.TrimSpace(item.Name) == "" {
				item.Name = "当前输入源"
			}
			item.Sources = cloneDesktopSources(patch.Value)
			if item.CreatedAt == "" {
				item.CreatedAt = patch.Now
			}
			item.UpdatedAt = patch.Now
		},
		NewItem: func(patch probecore.ProfileItemPatch[[]DesktopSource]) sourceProfileItem {
			return sourceProfileItem{
				CreatedAt: patch.Now,
				ID:        patch.ID,
				Name:      patch.Name,
				Sources:   cloneDesktopSources(patch.Value),
				UpdatedAt: patch.Now,
			}
		},
		NewProfileID: func() string {
			return fmt.Sprintf("source-profile-%d", time.Now().UnixNano())
		},
		ForceNewID: func(profileID string) bool {
			return profileID == defaultSourceProfileID
		},
		DropPlaceholder: func(store sourceProfileStore, profileID string) bool {
			return profileID != defaultSourceProfileID && isBlankDefaultSourceProfilePlaceholder(store)
		},
	})
	if err := saveSourceProfileStore(store); err != nil {
		return desktopCommandResult("SOURCE_PROFILE_SAVE_FAILED", nil, err.Error(), false, nil, nil)
	}
	snapshot["sources"] = cloneDesktopSources(sources)
	if err := writeDesktopConfigSnapshot(desktopConfigFilePath(), snapshot); err != nil {
		return desktopCommandResult("SOURCE_PROFILE_SAVE_FAILED", nil, err.Error(), false, nil, nil)
	}
	return desktopCommandResult("SOURCE_PROFILE_UPDATE_OK", map[string]any{
		"config_snapshot": snapshot,
		"source_profiles": store,
		"sources":         cloneDesktopSources(sources),
	}, "当前输入源档案已更新并保存。", true, nil, nil)
}

func (a *App) SaveSourceProfileStore(payload map[string]any) DesktopCommandResult {
	rawStore := firstNonNil(payload["source_profiles"], payload["sourceProfiles"], payload["store"])
	store := sourceProfileStoreFromAny(rawStore)
	if len(store.Items) == 0 {
		store = blankSourceProfileStore()
	}
	store = normalizeSourceProfileStoreForSave(store)
	if err := saveSourceProfileStore(store); err != nil {
		return desktopCommandResult("SOURCE_PROFILE_STORE_SAVE_FAILED", nil, err.Error(), false, nil, nil)
	}
	return desktopCommandResult("SOURCE_PROFILE_STORE_SAVE_OK", store, "输入源配置档案列表已恢复。", true, nil, nil)
}

func (a *App) SwitchSourceProfile(payload map[string]any) DesktopCommandResult {
	profileID := strings.TrimSpace(stringValue(firstNonNil(payload["profile_id"], payload["profileId"], payload["id"]), ""))
	if profileID == "" {
		return desktopCommandResult("SOURCE_PROFILE_INVALID", nil, "缺少 profile_id。", false, nil, nil)
	}
	snapshot, err := loadDesktopConfigSnapshotFromDisk()
	if err != nil {
		return desktopCommandResult("SOURCE_PROFILE_LOAD_FAILED", nil, err.Error(), false, nil, nil)
	}
	store, err := loadSourceProfileStoreForSnapshot(snapshot)
	if err != nil {
		return desktopCommandResult("SOURCE_PROFILE_LOAD_FAILED", nil, err.Error(), false, nil, nil)
	}
	for _, item := range store.Items {
		if item.ID != profileID {
			continue
		}
		store.ActiveProfileID = profileID
		if err := saveSourceProfileStore(store); err != nil {
			return desktopCommandResult("SOURCE_PROFILE_SAVE_FAILED", nil, err.Error(), false, nil, nil)
		}
		snapshot["sources"] = cloneDesktopSources(item.Sources)
		if err := writeDesktopConfigSnapshot(desktopConfigFilePath(), snapshot); err != nil {
			return desktopCommandResult("SOURCE_PROFILE_SWITCH_FAILED", nil, err.Error(), false, nil, nil)
		}
		return desktopCommandResult("SOURCE_PROFILE_SWITCH_OK", map[string]any{
			"config_snapshot": snapshot,
			"source_profiles": store,
			"sources":         cloneDesktopSources(item.Sources),
		}, "输入源配置档案已切换。", true, nil, nil)
	}
	return desktopCommandResult("SOURCE_PROFILE_NOT_FOUND", nil, "未找到输入源配置档案。", false, nil, nil)
}

func (a *App) DeleteSourceProfile(payload map[string]any) DesktopCommandResult {
	profileID := strings.TrimSpace(stringValue(firstNonNil(payload["profile_id"], payload["profileId"], payload["id"]), ""))
	if profileID == "" {
		return desktopCommandResult("SOURCE_PROFILE_INVALID", nil, "缺少 profile_id。", false, nil, nil)
	}
	snapshot, err := loadDesktopConfigSnapshotFromDisk()
	if err != nil {
		return desktopCommandResult("SOURCE_PROFILE_LOAD_FAILED", nil, err.Error(), false, nil, nil)
	}
	store, err := loadSourceProfileStoreForSnapshot(snapshot)
	if err != nil {
		return desktopCommandResult("SOURCE_PROFILE_LOAD_FAILED", nil, err.Error(), false, nil, nil)
	}
	deletedActiveProfile := store.ActiveProfileID == profileID
	nextItems := make([]sourceProfileItem, 0, len(store.Items))
	deleted := false
	for _, item := range store.Items {
		if item.ID == profileID {
			deleted = true
			continue
		}
		nextItems = append(nextItems, item)
	}
	if !deleted {
		return desktopCommandResult("SOURCE_PROFILE_NOT_FOUND", nil, "未找到输入源配置档案。", false, nil, nil)
	}
	store.Items = nextItems
	if len(store.Items) == 0 {
		store = blankSourceProfileStore()
	} else if store.ActiveProfileID == profileID {
		store.ActiveProfileID = store.Items[0].ID
	}
	if err := saveSourceProfileStore(store); err != nil {
		return desktopCommandResult("SOURCE_PROFILE_DELETE_FAILED", nil, err.Error(), false, nil, nil)
	}
	if deletedActiveProfile {
		snapshot["sources"] = cloneDesktopSources(activeSourceProfileSources(store))
		if err := writeDesktopConfigSnapshot(desktopConfigFilePath(), snapshot); err != nil {
			return desktopCommandResult("SOURCE_PROFILE_DELETE_FAILED", nil, err.Error(), false, nil, nil)
		}
	}
	return desktopCommandResult("SOURCE_PROFILE_DELETE_OK", store, "输入源配置档案已删除。", true, nil, nil)
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
	if err := appcore.WriteFileAtomic(path, raw, 0o600); err != nil {
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
	closeDebugLog := func() {}
	debugWarnings := []string{}
	debugLogPath := ""
	if req.DisableDebugLog {
		debugLogPath = debugLogPathForProbeConfig(cfg)
	} else {
		closeDebugLog, debugWarnings, debugLogPath = configureProbeDebugRuntime(cfg)
		utils.SetDebugLogContext(taskID)
	}
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
		logProbeFailed(taskID, currentStage, start, completedStages, err, false, nil)
		return ProbeRunResult{}, err
	}
	if source.ValidCount == 0 {
		err := errors.New("没有可用的 IP/CIDR/域名输入")
		logProbeFailed(taskID, currentStage, start, completedStages, err, false, nil)
		return ProbeRunResult{}, err
	}

	cfg.IPText = strings.Join(source.Valid, ",")
	applyProbeConfig(cfg)
	task.SourceColoFilters = task.CloneSourceColoFilterMap(req.SourceColoFilters)
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

	task.HeadProgressHook = nil
	task.LatencyProgressHook = nil
	task.TraceProgressHook = nil
	oldTraceDiagnosticHook := task.TraceDiagnosticHook
	traceDiagnostics := appcore.NewTraceDiagnostics(cfg.TraceColoMode, cfg.TraceURL)
	task.TraceDiagnosticHook = traceDiagnostics.Record
	task.DownloadProgressHook = nil
	task.DownloadSpeedSampleHook = nil
	task.DownloadInterruptHook = nil
	task.ProbePauseHook = nil
	defer func() {
		task.LatencyProgressHook = nil
		task.HeadProgressHook = nil
		task.TraceProgressHook = nil
		task.TraceDiagnosticHook = oldTraceDiagnosticHook
		task.DownloadProgressHook = nil
		task.DownloadSpeedSampleHook = nil
		task.DownloadInterruptHook = nil
		task.ProbePauseHook = nil
	}()
	if taskID != "" {
		task.ProbePauseHook = func(stage, ip string) {
			a.waitIfProbePaused(taskID, stage, ip, emitter)
		}
		task.DownloadInterruptHook = func(stage, ip string, interrupt func()) func() {
			return a.registerDownloadInterrupt(taskID, stage, ip, interrupt)
		}
	}

	stageResult, err := probecore.RunProbeStages(probecore.StageWorkflowRequest{
		Config: probecore.StageWorkflowConfig{
			DisableDownload:     cfg.DisableDownload,
			DisableResultLimit:  req.DisableExport,
			DownloadSpeedMetric: cfg.DownloadSpeedMetric,
			PrintNum:            cfg.PrintNum,
			Stage3Limit:         cfg.Stage3Limit,
			TCPPort:             cfg.TCPPort,
		},
		ConfigWarnings: configWarnings,
		DebugWarnings:  debugWarnings,
		SourcePorts:    req.SourcePorts,
		Source:         source,
		TaskContext:    req.TaskContext,
	}, probecore.StageWorkflowAdapter{
		ConfigureProgress: func(info probecore.StageInfo) {
			configureDesktopStageProgress(emitter, info)
		},
		BeforeStage: func(info probecore.StageInfo) error {
			beforeDesktopStage(cfg, taskID, emitter, info)
			return nil
		},
		AfterStage: func(info probecore.StageInfo) error {
			afterDesktopStage(cfg, taskID, info)
			return nil
		},
		RunTCP: func() utils.PingDelaySet {
			task.Httping = false
			return desktopTCPProbeRunner()
		},
		RunTrace:    desktopTraceProbeRunner,
		RunDownload: desktopDownloadProbeRunner,
	})
	completedStages = append(completedStages, stageResult.CompletedStages...)
	if err != nil {
		failureStage := ""
		tracePayload := traceDiagnostics.Payload()
		if !traceDiagnostics.Empty() && stageResult.CurrentStage == probecore.StageTrace {
			failureStage = probecore.StageTrace
			rawError := err.Error()
			summary := appcore.StageTraceFailureMessage(traceDiagnostics.Summary(), rawError)
			if tracePayload == nil {
				tracePayload = map[string]any{}
			}
			tracePayload["raw_error"] = rawError
			tracePayload["summary"] = summary
			err = errors.New(summary)
		}
		logProbeFailed(taskID, stageResult.CurrentStage, start, completedStages, err, false, tracePayload)
		return ProbeRunResult{
			DebugLogPath:     debugLogPath,
			FailureStage:     failureStage,
			TraceDiagnostics: tracePayload,
			Warnings:         dedupeStrings(stageResult.Warnings),
		}, err
	}

	resultData := stageResult.RawResults
	warnings := append([]string(nil), stageResult.Warnings...)

	outputFile := ""
	if len(resultData) > 0 && !req.DisableExport {
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
					emitter.emit("probe.partial_export", withDebugLogPath(map[string]any{
						"target_path": outputFile,
						"written":     len(resultData),
					}, debugLogPath))
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

	result := ProbeRunResult{
		Config:           cfg,
		DebugLogPath:     debugLogPath,
		DurationMS:       time.Since(start).Milliseconds(),
		OutputFile:       outputFile,
		Results:          stageResult.Results,
		Source:           source,
		StartedAt:        start.Format(time.RFC3339),
		Summary:          stageResult.Summary,
		TaskContext:      stageResult.TaskContext,
		TraceDiagnostics: traceDiagnostics.Payload(),
		Warnings:         dedupeStrings(warnings),
		SchemaVersion:    guiSchemaVersion,
		RawResults:       append([]utils.CloudflareIPData(nil), resultData...),
	}
	if appcore.ShouldMarkTraceFailureStage(stageResult.CompletedStages, traceDiagnostics, resultData) {
		result.FailureStage = probecore.StageTrace
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

func configureDesktopStageProgress(emitter *desktopProbeEmitter, info probecore.StageInfo) {
	switch info.Stage {
	case probecore.StageTCP:
		task.LatencyProgressHook = func(processed, passed, failed, _ int) {
			if emitter == nil {
				return
			}
			emitter.emitProgress(probecore.StageTCP, processed, passed, failed, info.Total)
		}
	case probecore.StageTrace:
		task.TraceProgressHook = func(processed, passed, failed, total int) {
			if emitter == nil {
				return
			}
			emitter.emitProgress(probecore.StageTrace, processed, passed, failed, total)
		}
	case probecore.StageDownload:
		if info.Total <= 0 {
			return
		}
		task.DownloadProgressHook = func(processed, qualified, _ int) {
			if emitter == nil {
				return
			}
			emitter.emitProgress(probecore.StageDownload, processed, qualified, processed-qualified, info.Total)
		}
		task.DownloadSpeedSampleHook = func(sample task.DownloadSpeedSample) {
			if emitter == nil {
				return
			}
			emitter.emitSpeed(sample)
		}
	}
}

func beforeDesktopStage(cfg ProbeConfig, taskID string, emitter *desktopProbeEmitter, info probecore.StageInfo) {
	switch info.Stage {
	case probecore.StageTCP:
		if emitter != nil {
			emitter.emitProgress(probecore.StageTCP, 0, 0, 0, info.Total)
		}
		task.CheckProbePause(probecore.StageTCP, "")
		utils.DebugEvent("stage.start", map[string]any{
			"config": map[string]any{
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
			"counts":  map[string]any{"total": info.Total},
			"message": "开始 TCP 测延迟。",
			"stage":   info.Stage,
			"task_id": taskID,
		})
	case probecore.StageTrace:
		if emitter != nil {
			emitter.emitProgress(probecore.StageTrace, 0, 0, 0, info.Total)
		}
		task.CheckProbePause(probecore.StageTrace, "")
		utils.DebugEvent("stage.start", map[string]any{
			"config": map[string]any{
				"accepted_status_code": cfg.HttpingStatusCode,
				"cf_colo_filter":       cfg.HttpingCFColo,
				"cf_colo_filter_mode":  cfg.HttpingCFColoMode,
				"source_colo_filter":   cfg.SourceColoFilterPhase,
				"trace_colo_mode":      cfg.TraceColoMode,
				"trace_concurrency":    cfg.HeadRoutines,
				"trace_max_latency_ms": cfg.HeadMaxDelayMS,
				"trace_routines_limit": task.MaxTraceRoutines,
				"trace_url":            cfg.TraceURL,
				"retry_backoff_ms":     cfg.RetryBackoffMS,
				"retry_max_attempts":   cfg.RetryMaxAttempts,
				"timeout_ms":           cfg.Stage2TimeoutMS,
			},
			"counts":  map[string]any{"input": info.Input, "total": info.Total},
			"message": "开始追踪探测。",
			"stage":   info.Stage,
			"task_id": taskID,
		})
	case probecore.StageDownload:
		utils.DebugEvent("stage.start", map[string]any{
			"config": map[string]any{
				"concurrency":                  cfg.Stage3Concurrency,
				"download_time_seconds_per_ip": cfg.DownloadTimeSeconds,
				"legacy_download_count":        cfg.TestCount,
				"min_download_mbps":            cfg.MinSpeedMB,
				"min_download_speed_metric":    cfg.DownloadSpeedMetric,
				"retry_backoff_ms":             cfg.RetryBackoffMS,
				"retry_max_attempts":           cfg.RetryMaxAttempts,
				"stage3_limit":                 cfg.Stage3Limit,
			},
			"counts":  map[string]any{"input": info.Input, "total": info.Total},
			"message": "开始文件测速。",
			"stage":   info.Stage,
			"task_id": taskID,
		})
		if emitter != nil && info.Total > 0 {
			emitter.emitProgress(probecore.StageDownload, 0, 0, 0, info.Total)
		}
		task.CheckProbePause(probecore.StageDownload, "")
	}
}

func afterDesktopStage(cfg ProbeConfig, taskID string, info probecore.StageInfo) {
	switch info.Stage {
	case probecore.StageTCP:
		utils.DebugEvent("stage.complete", map[string]any{
			"counts":      debugStageCounts(info.Total, info.Passed, info.Failed),
			"duration_ms": info.DurationMS,
			"message":     "TCP 测延迟完成。",
			"stage":       info.Stage,
			"task_id":     taskID,
			"tcp": map[string]any{
				"delay_column":              "TCP延迟(ms)",
				"max_latency_ms":            cfg.MaxDelayMS,
				"ping_times":                cfg.PingTimes,
				"skip_first_latency_sample": cfg.SkipFirstLatency,
			},
		})
	case probecore.StageTrace:
		utils.DebugEvent("stage.complete", map[string]any{
			"counts":      debugStageCounts(info.Total, info.Passed, info.Failed),
			"duration_ms": info.DurationMS,
			"trace": map[string]any{
				"accepted_status_code": cfg.HttpingStatusCode,
				"cf_colo_filter":       cfg.HttpingCFColo,
				"cf_colo_filter_mode":  cfg.HttpingCFColoMode,
				"source_colo_filter":   cfg.SourceColoFilterPhase,
				"trace_colo_mode":      cfg.TraceColoMode,
				"concurrency":          cfg.HeadRoutines,
				"max_latency_ms":       cfg.HeadMaxDelayMS,
				"url":                  cfg.TraceURL,
			},
			"message": "追踪探测完成。",
			"stage":   info.Stage,
			"task_id": taskID,
		})
	case probecore.StageDownload:
		utils.DebugEvent("stage.complete", map[string]any{
			"counts":      debugStageCounts(info.Total, info.Passed, info.Failed),
			"duration_ms": info.DurationMS,
			"get": map[string]any{
				"concurrency":                  cfg.Stage3Concurrency,
				"download_time_seconds_per_ip": cfg.DownloadTimeSeconds,
				"min_download_mbps":            cfg.MinSpeedMB,
				"min_download_speed_metric":    cfg.DownloadSpeedMetric,
			},
			"message": "文件测速完成。",
			"stage":   info.Stage,
			"task_id": taskID,
		})
	}
}

func logDesktopProbePreparationFailure(cfg ProbeConfig, taskID string, source SourceSummary, invalidCount int, statuses []DesktopSourceStatus, duration time.Duration, err error) {
	utils.Debug = cfg.Debug
	closeDebugLog, _, _ := configureProbeDebugRuntime(cfg)
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
	logProbeFailed(taskID, "stage0_pool", time.Now().Add(-duration), nil, err, false, nil)
}

func logProbeFailed(taskID, stage string, startedAt time.Time, completedStages []string, err error, recoverable bool, extras map[string]any) {
	message := "探测任务失败。"
	errText := ""
	if err != nil {
		message = err.Error()
		errText = err.Error()
	}
	fields := map[string]any{
		"completed_stages": completedStages,
		"duration_ms":      time.Since(startedAt).Milliseconds(),
		"error":            errText,
		"message":          message,
		"recoverable":      recoverable,
		"stage":            stage,
		"task_id":          taskID,
	}
	for key, value := range extras {
		fields[key] = value
	}
	utils.DebugEvent("probe.failed", fields)
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
		"debug_capture_address":             cfg.DebugCaptureAddress,
		"debug_capture_enabled":             cfg.DebugCaptureEnabled,
		"debug_log_mode":                    cfg.DebugLogMode,
		"debug_log_verbosity":               cfg.DebugLogVerbosity,
		"disable_download":                  cfg.DisableDownload,
		"download_buffer_kb":                cfg.DownloadBufferKB,
		"download_count":                    cfg.TestCount,
		"download_concurrency":              cfg.Stage3Concurrency,
		"download_get_concurrency":          cfg.DownloadGetConcurrency,
		"download_http_protocol":            cfg.DownloadHTTPProtocol,
		"download_speed_metric":             cfg.DownloadSpeedMetric,
		"download_speed_sample_interval_ms": cfg.DownloadSpeedSampleIntervalMS,
		"download_time_seconds_per_ip":      cfg.DownloadTimeSeconds,
		"download_warmup_seconds":           cfg.DownloadWarmupSeconds,
		"stage3_limit":                      cfg.Stage3Limit,
		"event_throttle_ms":                 cfg.EventThrottleMS,
		"export_append":                     cfg.ExportAppend,
		"csv_encoding":                      cfg.CSVEncoding,
		"cooldown_failures":                 cfg.CooldownFailures,
		"cooldown_ms":                       cfg.CooldownMS,
		"trace_concurrency":                 cfg.HeadRoutines,
		"trace_max_latency_ms":              cfg.HeadMaxDelayMS,
		"trace_timeout_ms":                  cfg.Stage2TimeoutMS,
		"trace_colo_mode":                   cfg.TraceColoMode,
		"trace_url":                         cfg.TraceURL,
		"source_colo_filter_phase":          cfg.SourceColoFilterPhase,
		"host_header":                       cfg.HostHeader,
		"httping_cf_colo":                   cfg.HttpingCFColo,
		"httping_cf_colo_mode":              cfg.HttpingCFColoMode,
		"httping_status_code":               cfg.HttpingStatusCode,
		"max_loss_rate":                     cfg.MaxLossRate,
		"max_tcp_latency_ms":                cfg.MaxDelayMS,
		"min_delay_ms":                      cfg.MinDelayMS,
		"min_download_mbps":                 cfg.MinSpeedMB,
		"ping_times":                        cfg.PingTimes,
		"retry_backoff_ms":                  cfg.RetryBackoffMS,
		"retry_max_attempts":                cfg.RetryMaxAttempts,
		"request_headers_count":             httpcfg.RequestHeadersCount(cfg.RequestHeaders),
		"routines":                          cfg.Routines,
		"skip_first_latency_sample":         cfg.SkipFirstLatency,
		"tcp_timeout_ms":                    cfg.Stage1TimeoutMS,
		"sni":                               cfg.SNI,
		"strategy":                          cfg.Strategy,
		"tcp_port":                          cfg.TCPPort,
		"url":                               cfg.URL,
		"user_agent":                        cfg.UserAgent,
		"write_output":                      cfg.WriteOutput,
	}
}

func defaultProbeConfig() ProbeConfig {
	return probecore.DefaultProbeConfig()
}

const (
	maxDesktopTCPRoutines         = probecore.DefaultMaxProbeTCPRoutines
	maxDesktopStage3Routines      = probecore.DefaultMaxProbeStage3Routines
	defaultDesktopSourceIPLimit   = 500
	sourceColoFilterPhasePrecheck = probecore.SourceColoFilterPhasePrecheck
	sourceColoFilterPhaseStage2   = probecore.SourceColoFilterPhaseStage2
)

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

func normalizeProbeConfig(cfg ProbeConfig) (ProbeConfig, []string) {
	return probecore.NormalizeProbeConfig(cfg, probecore.ProbeConfigNormalizeOptions{
		MaxTCPRoutines:    maxDesktopTCPRoutines,
		MaxStage3Routines: maxDesktopStage3Routines,
	})
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
	task.ColoDictionaryPath = desktopColoDictionaryPaths().Colo
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
	task.HttpingCFColomap = task.MapColoMap()
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
	utils.Output = currentOutputFile(cfg)
	utils.OutputAppend = cfg.ExportAppend
	utils.OutputCSVEncoding = cfg.CSVEncoding
	utils.Debug = cfg.Debug
}

func effectiveDebugCaptureAddress(cfg ProbeConfig) string {
	if !cfg.Debug || !cfg.DebugCaptureEnabled || strings.TrimSpace(cfg.DebugCaptureAddress) == "" {
		return ""
	}
	return httpcfg.Resolve("", "", "", cfg.DebugCaptureAddress, true).CaptureAddress
}

func configureProbeDebugRuntime(cfg ProbeConfig) (func(), []string, string) {
	path, err := utils.ConfigureDebugLog(cfg.Debug, debugLogFilePath(), cfg.DebugLogMode, cfg.DebugLogFormat, cfg.DebugLogVerbosity)
	if err != nil {
		return func() {}, []string{fmt.Sprintf("初始化调试日志失败：%v", err)}, ""
	}

	warnings := make([]string, 0, 2)
	if cfg.Debug && path != "" {
		warnings = append(warnings, fmt.Sprintf("调试日志已写入 %s", path))
	}
	if captureAddress := effectiveDebugCaptureAddress(cfg); captureAddress != "" {
		warnings = append(warnings, fmt.Sprintf("调试模式已将请求拨号目标覆盖为 %s", captureAddress))
	}

	return func() {
		_ = utils.CloseDebugLog()
	}, warnings, path
}

func currentOutputFile(cfg ProbeConfig) string {
	if !cfg.WriteOutput {
		return ""
	}
	return cfg.OutputFile
}

func debugLogPathForProbeConfig(cfg ProbeConfig) string {
	if !cfg.Debug {
		return ""
	}
	return debugLogFilePath()
}

func withDebugLogPath(payload map[string]any, debugLogPath string) map[string]any {
	if strings.TrimSpace(debugLogPath) != "" {
		payload["debug_log_path"] = strings.TrimSpace(debugLogPath)
	}
	return payload
}

func resolveDesktopResultFilePath(payload map[string]any, cfg ProbeConfig) string {
	for _, key := range []string{"path", "source_path", "sourcePath", "export_path", "exportPath"} {
		if path := strings.TrimSpace(stringValue(payload[key], "")); path != "" {
			return path
		}
	}
	if outputFile := currentOutputFile(cfg); strings.TrimSpace(outputFile) != "" {
		return outputFile
	}
	if path := filepath.Join(storageRoot(), "result.csv"); strings.TrimSpace(path) != "" {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return "result.csv"
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
	return probecore.SummarizeSource(raw, sourceParseResolver)
}

func readProbeResultRowsFromCSV(path string) ([]ProbeResultRow, error) {
	return appcore.ReadProbeResultRowsFromCSV(path)
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
	return probecore.DefaultConfigSnapshot(desktopConfigSnapshotOptions())
}

func loadDesktopConfigSnapshotFromDisk() (map[string]any, error) {
	return appcore.LoadConfigSnapshotFromDisk(desktopConfigFilePath(), defaultDesktopConfigSnapshot, sanitizeDesktopConfigSnapshot)
}

func desktopDraftStatusPayload() map[string]any {
	path := desktopDraftFilePath()
	payload := map[string]any{
		"exists":              false,
		"is_newer_than_saved": false,
		"path":                path,
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			payload["error"] = err.Error()
		}
		return payload
	}
	var saved map[string]any
	if err := json.Unmarshal(raw, &saved); err != nil {
		payload["exists"] = true
		payload["error"] = err.Error()
		return payload
	}
	snapshot := mapValue(saved["config_snapshot"])
	if len(snapshot) == 0 {
		snapshot = saved
	}
	draftSavedAt := parseTimeValue(saved["saved_at"])
	configSavedAt := desktopConfigSavedAt()
	payload["exists"] = true
	payload["config_snapshot"] = sanitizeDesktopConfigSnapshot(snapshot)
	if !draftSavedAt.IsZero() {
		payload["saved_at"] = draftSavedAt.Format(time.RFC3339)
	}
	if !configSavedAt.IsZero() {
		payload["config_saved_at"] = configSavedAt.Format(time.RFC3339)
	}
	payload["is_newer_than_saved"] = !draftSavedAt.IsZero() && (configSavedAt.IsZero() || draftSavedAt.After(configSavedAt))
	return payload
}

func desktopConfigSavedAt() time.Time {
	raw, err := os.ReadFile(desktopConfigFilePath())
	if err != nil {
		return time.Time{}
	}
	var saved map[string]any
	if err := json.Unmarshal(raw, &saved); err != nil {
		return time.Time{}
	}
	return parseTimeValue(saved["saved_at"])
}

func parseTimeValue(value any) time.Time {
	raw := strings.TrimSpace(stringValue(value, ""))
	if raw == "" {
		return time.Time{}
	}
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}
	}
	return parsed
}

func loadSourceProfileStoreForSnapshot(_ map[string]any) (sourceProfileStore, error) {
	store, err := loadSourceProfileStore()
	if err != nil {
		return store, err
	}
	if len(store.Items) == 0 {
		return blankSourceProfileStore(), nil
	}
	if strings.TrimSpace(store.ActiveProfileID) == "" {
		store.ActiveProfileID = store.Items[0].ID
	}
	return store, nil
}

func blankSourceProfileStore() sourceProfileStore {
	return appcore.BlankSourceProfileStore(time.Now().Format(time.RFC3339), sourceProfilesSchemaVersion)
}

func defaultSourceProfileStoreFromSnapshot(snapshot map[string]any) sourceProfileStore {
	return appcore.DefaultSourceProfileStoreFromSnapshot(snapshot, defaultDesktopConfigSnapshot(), sourceProfilesSchemaVersion)
}

func normalizeSourceProfileStoreForSave(store sourceProfileStore) sourceProfileStore {
	return appcore.NormalizeSourceProfileStoreForSave(store, sourceProfilesSchemaVersion, time.Now().Format(time.RFC3339), func(index int) string {
		return fmt.Sprintf("source-profile-%d", time.Now().UnixNano()+int64(index))
	})
}

func activeSourceProfileSources(store sourceProfileStore) []DesktopSource {
	return appcore.ActiveSourceProfileSources(store)
}

func isBlankDefaultSourceProfilePlaceholder(store sourceProfileStore) bool {
	return appcore.IsBlankSourceProfilePlaceholder(store, defaultSourceProfileID)
}

func sourceProfileStoreFromAny(value any) sourceProfileStore {
	return appcore.SourceProfileStoreFromAny(value)
}

func desktopSourcesFromAny(value any) []DesktopSource {
	return appcore.SourcesFromAny(value)
}

func cloneDesktopSources(sources []DesktopSource) []DesktopSource {
	return appcore.CloneSources(sources)
}

func writeDesktopConfigSnapshot(path string, snapshot map[string]any) error {
	return appcore.WriteConfigSnapshot(path, snapshot, guiSchemaVersion, sanitizeDesktopConfigSnapshot)
}

func desktopConfigToProbeConfig(config map[string]any) (ProbeConfig, []string) {
	options := desktopConfigSnapshotOptions()
	options.DefaultExportTargetDir = storageRoot()
	options.ProfileName = activeProfileName()
	return probecore.ConfigSnapshotToProbeConfig(config, options)
}

func probeDownloadSpeedSampleIntervalMS(probe map[string]any, fallback ProbeConfig) int {
	return probecore.ProbeDownloadSpeedSampleIntervalMS(probe, fallback)
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
	return probecore.ExportFileName(exportCfg, taskID, profileName, now)
}

func desktopExportPath(exportCfg map[string]any, fileName string) string {
	return probecore.ExportPath(exportCfg, fileName, storageRoot())
}

func prepareDesktopSources(cfg ProbeConfig, sources []DesktopSource) preparedDesktopSources {
	client := newDesktopSourceHTTPClient(cfg)
	now := time.Now()
	return appcore.PrepareSources(appcore.PrepareSourcesOptions{
		Config: cfg,
		ProcessSource: func(source DesktopSource) (appcore.SourceProcessResult, error) {
			return processDesktopSource(cfg, source, client, now)
		},
		Sources: sources,
	})
}

func loadDesktopSourceContent(source DesktopSource, cfg ProbeConfig, client *http.Client) (desktopSourceContentResult, error) {
	return appcore.LoadSourceContent(source, cfg, client, desktopSourceContentLoadOptions())
}

func loadDesktopRemoteSourceContent(source DesktopSource, cfg ProbeConfig, client *http.Client) (desktopSourceContentResult, error) {
	return appcore.LoadSourceContent(source, cfg, client, desktopSourceContentLoadOptions())
}

func fetchDesktopRemoteSourceURL(targetURL string, cfg ProbeConfig, client *http.Client) (string, int, error) {
	return appcore.FetchSourceURL(targetURL, cfg, client)
}

func isRetryableDesktopSourceReadError(statusCode int, err error) bool {
	if err == nil {
		return false
	}
	if statusCode == http.StatusTooManyRequests || statusCode >= 500 {
		return true
	}
	return statusCode == 0
}

func normalizeDesktopSourceURLInput(rawURL string) (string, error) {
	return appcore.NormalizeSourceURLInput(rawURL)
}

func desktopSourceContentLoadOptions() appcore.SourceContentLoadOptions {
	return appcore.SourceContentLoadOptions{
		BuildAttempts: func(primaryURL string, source appcore.Source) []appcore.RemoteSourceAttempt {
			if cdnURL, ok := githubRawToJSDelivrURL(primaryURL); ok && cdnURL != primaryURL {
				return []appcore.RemoteSourceAttempt{
					{URL: primaryURL},
					{URL: cdnURL},
				}
			}
			return []appcore.RemoteSourceAttempt{
				{URL: primaryURL},
				{URL: primaryURL},
			}
		},
		ShouldRetry: isRetryableDesktopSourceReadError,
		OnFallbackSuccess: func(primaryURL string, used appcore.RemoteSourceAttempt, source appcore.Source) []string {
			if used.URL == primaryURL {
				return nil
			}
			name := appcore.SourceName(source)
			if name == "" {
				name = "远程输入源"
			}
			return []string{fmt.Sprintf("输入源 %s 已通过 jsDelivr CDN 兜底读取。", name)}
		},
	}
}

func githubRawToJSDelivrURL(rawURL string) (string, bool) {
	parsed, err := url.Parse(rawURL)
	if err != nil || !strings.EqualFold(parsed.Host, "raw.githubusercontent.com") {
		return "", false
	}
	segments := pathSegments(parsed.Path)
	if len(segments) < 4 {
		return "", false
	}
	owner := segments[0]
	repo := segments[1]
	branchIndex := 2
	if len(segments) >= 6 && segments[2] == "refs" && segments[3] == "heads" {
		branchIndex = 4
	}
	branch := segments[branchIndex]
	fileSegments := segments[branchIndex+1:]
	if owner == "" || repo == "" || branch == "" || len(fileSegments) == 0 {
		return "", false
	}
	cdn := url.URL{
		Scheme: "https",
		Host:   "cdn.jsdelivr.net",
		Path:   "/gh/" + strings.Join(append([]string{owner, repo + "@" + branch}, fileSegments...), "/"),
	}
	return cdn.String(), true
}

func jsDelivrToGithubRawURL(cdnURL string) (string, bool) {
	parsed, err := url.Parse(cdnURL)
	if err != nil || !strings.EqualFold(parsed.Host, "cdn.jsdelivr.net") {
		return "", false
	}
	segments := pathSegments(parsed.Path)
	if len(segments) < 4 || segments[0] != "gh" {
		return "", false
	}
	owner := segments[1]
	repoBranch := segments[2]
	repo, branch, ok := strings.Cut(repoBranch, "@")
	fileSegments := segments[3:]
	if !ok || owner == "" || repo == "" || branch == "" || len(fileSegments) == 0 {
		return "", false
	}
	raw := url.URL{
		Scheme: "https",
		Host:   "raw.githubusercontent.com",
		Path:   "/" + strings.Join(append([]string{owner, repo, branch}, fileSegments...), "/"),
	}
	return raw.String(), true
}

func pathSegments(value string) []string {
	raw := strings.Trim(value, "/")
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, "/")
	segments := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			segments = append(segments, part)
		}
	}
	return segments
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
	snapshot = sanitizeDesktopConfigSnapshot(snapshot)
	body := map[string]any{
		"config_snapshot": snapshot,
		"saved_at":        time.Now().Format(time.RFC3339),
		"schema_version":  guiSchemaVersion,
	}
	encoded, err := json.MarshalIndent(body, "", "  ")
	if err != nil {
		return err
	}
	return appcore.WriteFileAtomic(path, encoded, 0o600)
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
