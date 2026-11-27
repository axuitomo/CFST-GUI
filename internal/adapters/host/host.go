package host

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/kernel/appcore"
	configvalue "github.com/axuitomo/CFST-GUI/internal/kernel/domain/config"
	"github.com/axuitomo/CFST-GUI/internal/kernel/domain/probe/probecore"
)

const legacyDesktopSchemaVersion = "cfst-gui-wails-v1"
const defaultFileTestURL = probecore.DefaultFileTestURL
const (
	defaultSchedulerConfigSource        = "draft_preferred"
	defaultSchedulerSourceProfileAction = appcore.SchedulerSourceProfileActionUpdate
)

type Host struct {
	Core           *appcore.Service
	DroppedSources DroppedSourcesState

	runMu           sync.Mutex
	schedulerMu     sync.Mutex
	schedulerCancel context.CancelFunc
	options         Options
}

type Options struct {
	InstallMode       func() string
	OnEmitEvent       func(appcore.ProbeEvent)
	OnQuitAfterUpdate func()
	StartupSources    []string
}

type HealthResult struct {
	ConfigPath     string `json:"configPath"`
	Online         bool   `json:"online"`
	SchemaVersion  string `json:"schemaVersion"`
	Service        string `json:"service"`
	Version        string `json:"version"`
	WailsTransport string `json:"wailsTransport"`
}

func NewHost(opts Options) *Host {
	probeOptions := probecore.DesktopConfigSnapshotOptions()
	host := &Host{
		Core: appcore.NewService(appcore.ServiceOptions{
			AppVersion:          version,
			ArchiveStorageState: func() any { return resolveStorageState() },
			SchedulerDefaults: appcore.SchedulerConfig{
				AutoDNSPush:                true,
				AutoGitHubExport:           true,
				SkipIfActive:               true,
				ConfigSource:               defaultSchedulerConfigSource,
				PostRunSourceProfileAction: defaultSchedulerSourceProfileAction,
			},
			ColoPaths:        desktopColoDictionaryPaths(),
			DefaultExportDir: defaultExportDir(),
			Storage: appcore.StorageLayout{
				Root:               storageRoot(),
				ConfigFileName:     "desktop-config.json",
				DraftFileName:      "desktop-draft.json",
				SchedulerFileName:  "scheduler-status.json",
				SourceProfilesFile: sourceProfilesFileName,
			},
			StorageCommands: appcore.StorageCommandHooks{
				Set: func(payload map[string]any) (appcore.StorageSetResult, error) {
					status, migration, err := setStorageDirectory(payload)
					return appcore.StorageSetResult{Migration: migration, Storage: status}, err
				},
				Health: func(map[string]any) (appcore.StorageHealthResult, error) {
					path := storageRoot()
					return appcore.StorageHealthResult{
						Health:  checkStorageHealthForPath(path, false),
						Storage: resolveStorageState(),
					}, nil
				},
			},
			ConfigPolicy:       appcore.NewConfigPolicy([]string{legacyDesktopSchemaVersion}, probeOptions),
			ProbeConfigOptions: probeOptions,
		}),
		options: opts,
	}
	host.Core.SetRuntimeCleanupHooks(func() bool {
		return appcore.SchedulerStageActive(host.CurrentSchedulerStatus().WorkflowStage)
	}, func() {
		closeUpdateIdleConnections()
	})
	host.Core.SetConfigSavedHook(host.ReloadSchedulerFromSnapshot)
	host.Core.SetEventSink(appcore.EventSinkFunc(func(_ context.Context, event appcore.ProbeEvent) {
		if opts.OnEmitEvent != nil {
			opts.OnEmitEvent(event)
		}
	}))
	host.DroppedSources.Add(opts.StartupSources)
	return host
}

func (h *Host) SetEventSink(fn func(appcore.ProbeEvent)) {
	h.options.OnEmitEvent = fn
	h.Core.SetEventSink(appcore.EventSinkFunc(func(_ context.Context, event appcore.ProbeEvent) {
		if fn != nil {
			fn(event)
		}
	}))
}

func (h *Host) installMode() string {
	if h.options.InstallMode != nil {
		return h.options.InstallMode()
	}
	return DefaultInstallMode(runtime.GOOS)
}

// PrepareRuntime 是桌面与 WebUI 共用的启动准备：迁移数据目录、启动运行时清理、检查 COLO 词典。
// 托盘和调度器各自留在调用方，测试可以只跑这一段而不创建系统托盘。
func (h *Host) PrepareRuntime(ctx context.Context) {
	_, _ = appcore.RunStorageMigration(h.Core.StorageLayout(), time.Now())
	h.Core.StartRuntimeCleanup(ctx)
	h.Core.StartColoDictionaryWarmup()
}

func (h *Host) Shutdown() {
	h.Core.StopRuntimeCleanup()
	h.StopScheduler()
}

func (h *Host) GetHealth() HealthResult {
	return HealthResult{
		ConfigPath:     ConfigFilePath(),
		Online:         true,
		SchemaVersion:  appcore.CommandSchemaVersion,
		Service:        "CFST Wails Bridge",
		Version:        AppVersion(),
		WailsTransport: "window.go.app.App",
	}
}

func (h *Host) GetAppInfo() appcore.CommandResult {
	return appcore.NewCommandResult("APP_INFO_READY", appInfoPayloadFor(h.installMode()), "应用信息已读取。", true, nil, nil)
}

func (h *Host) CheckForUpdates(payload map[string]any) appcore.CommandResult {
	_ = payload
	info, err := checkGitHubReleaseForUpdateWithInstallMode(context.Background(), h.installMode())
	if err != nil {
		return appcore.NewCommandResult("UPDATE_CHECK_FAILED", map[string]any{
			"current_version": AppVersion(),
			"release_url":     ReleasePageURL,
		}, err.Error(), false, nil, nil)

	}
	message := "当前已是最新版本。"
	if info.UpdateAvailable {
		message = fmt.Sprintf("发现新版本 %s。", info.LatestVersion)
	}
	return appcore.NewCommandResult("UPDATE_CHECK_OK", info, message, true, nil, nil)
}

func (h *Host) DownloadAndInstallUpdate(payload map[string]any) appcore.CommandResult {
	info, err := resolveGitHubReleaseUpdateWithInstallMode(context.Background(), h.installMode())
	if err != nil {
		return appcore.NewCommandResult("UPDATE_CHECK_FAILED", nil, err.Error(), false, nil, nil)
	}
	if !info.UpdateAvailable {
		return appcore.NewCommandResult("UPDATE_NOT_AVAILABLE", info, "当前已是最新版本。", true, nil, nil)
	}
	result, err := downloadAndInstallUpdate(context.Background(), info, configvalue.String(configvalue.FirstNonNil(payload["download_dir"], payload["downloadDir"]), ""))
	if err != nil {
		return appcore.NewCommandResult("UPDATE_INSTALL_FAILED", result, err.Error(), false, nil, nil)
	}
	if result.InstallStarted && h.options.OnQuitAfterUpdate != nil {
		h.options.OnQuitAfterUpdate()
	}
	message := "更新包已下载并触发安装流程。"
	if !result.InstallStarted && strings.TrimSpace(result.NextAction) == "manual" {
		message = "更新包已下载，请按当前平台的部署方式手动安装或替换。"
	}
	return appcore.NewCommandResult("UPDATE_INSTALL_READY", result, message, true, nil, nil)
}

func (h *Host) OpenReleasePage() appcore.CommandResult {
	if err := OpenExternalURL(ReleasePageURL); err != nil {
		return appcore.NewCommandResult("RELEASE_OPEN_FAILED", map[string]any{
			"release_url": ReleasePageURL,
		}, err.Error(), false, nil, nil)

	}
	return appcore.NewCommandResult("RELEASE_OPENED", map[string]any{
		"release_url": ReleasePageURL,
	}, "已打开发行页。", true, nil, nil)

}

func NormalizePathSelectionMode(mode string) string {
	mode = strings.ToLower(strings.TrimSpace(mode))
	mode = strings.ReplaceAll(mode, "-", "_")
	if mode == "" {
		return "source_file"
	}
	return mode
}

func SelectPathDefaultDirectory(path string) string {
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

func ConfigFilePath() string {
	return filepath.Join(storageRoot(), "config.json")
}

func DesktopConfigFilePath() string {
	return filepath.Join(storageRoot(), "desktop-config.json")
}

func DebugLogFilePath() string {
	return filepath.Join(LogDirectoryPath(), "cfip-log.txt")
}

func ErrorLogFilePath() string {
	return filepath.Join(LogDirectoryPath(), "error-log.txt")
}

func LogDirectoryPath() string {
	return filepath.Join(storageRoot(), "logs")
}
