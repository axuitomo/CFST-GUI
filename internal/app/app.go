package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/appcore"
	"github.com/axuitomo/CFST-GUI/internal/configvalue"
	"github.com/axuitomo/CFST-GUI/internal/probecore"
)

const legacyDesktopSchemaVersion = "cfst-gui-wails-v1"
const defaultFileTestURL = probecore.DefaultFileTestURL
const (
	defaultPortPolicy                   = probecore.PortPolicySourceOverrideGlobal
	defaultThemeMode                    = "auto_system_time"
	defaultThemeLightStart              = "07:00"
	defaultThemeDarkStart               = "19:00"
	defaultSchedulerConfigSource        = "draft_preferred"
	defaultSchedulerSourceProfileAction = appcore.SchedulerSourceProfileActionUpdate
)

type App struct {
	ctx  context.Context
	core *appcore.Service

	runMu    sync.Mutex
	eventHub *webUIEventHub

	schedulerMu     sync.Mutex
	schedulerCancel context.CancelFunc

	trayStartOnce sync.Once
	trayStopOnce  sync.Once
	trayMu        sync.Mutex
	trayAvailable bool
	quitting      bool
}

type HealthResult struct {
	ConfigPath     string `json:"configPath"`
	Online         bool   `json:"online"`
	SchemaVersion  string `json:"schemaVersion"`
	Service        string `json:"service"`
	Version        string `json:"version"`
	WailsTransport string `json:"wailsTransport"`
}

var (
	cloudflareAPIBaseURL = appcore.CloudflareAPIBaseURL
	githubAPIBaseURL     = "https://api.github.com"
)

func NewApp() *App {
	app := &App{
		core: appcore.NewService(appcore.ServiceOptions{
			AppVersion:          version,
			ArchiveStorageState: func() any { return resolveStorageState() },
			SchedulerDefaults: appcore.SchedulerConfig{
				AutoDNSPush:                true,
				AutoGitHubExport:           true,
				SkipIfActive:               true,
				ConfigSource:               defaultSchedulerConfigSource,
				PostRunSourceProfileAction: defaultSchedulerSourceProfileAction,
			},
			CloudflareAPIBaseURL: cloudflareAPIBaseURL,
			ColoPaths:            desktopColoDictionaryPaths(),
			DefaultExportDir:     defaultExportDir(),
			GitHubAPIBaseURL:     githubAPIBaseURL,
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
			ConfigPolicy: appcore.ConfigPolicy{
				LegacySchemaVersions: []string{legacyDesktopSchemaVersion},
				DefaultSnapshot: func() map[string]any {
					return probecore.DefaultConfigSnapshot(desktopConfigSnapshotOptions())
				},
				SanitizeSnapshot: func(input map[string]any) map[string]any {
					return probecore.SanitizeConfigSnapshot(input, desktopConfigSnapshotOptions())
				},
			},
			ProbeConfigOptions: desktopConfigSnapshotOptions(),
			SourceResolver:     sourceParseResolver,
			SourceHTTPTimeout:  30 * time.Second,
		}),
		eventHub: newWebUIEventHub(),
	}
	app.core.SetRuntimeCleanupHooks(func() bool {
		return appcore.SchedulerStageActive(app.currentSchedulerStatus().WorkflowStage)
	}, func() {
		closeUpdateIdleConnections()
	})
	app.core.SetConfigSavedHook(app.reloadSchedulerFromSnapshot)
	app.core.SetEventSink(appcore.EventSinkFunc(func(_ context.Context, event appcore.ProbeEvent) {
		app.emitProbeEvent(event)
	}))
	return app
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.core.StartRuntimeCleanup(ctx)
	a.startTray()
	a.reloadSchedulerFromDisk()
}

func (a *App) GetHealth() HealthResult {
	return HealthResult{
		ConfigPath:     configFilePath(),
		Online:         true,
		SchemaVersion:  appcore.CommandSchemaVersion,
		Service:        "CFST Wails Bridge",
		Version:        appVersion(),
		WailsTransport: "window.go.app.App",
	}
}

func (a *App) GetAppInfo() appcore.CommandResult {
	return appcore.NewCommandResult("APP_INFO_READY", appInfoPayload(), "应用信息已读取。", true, nil, nil)
}

func (a *App) CheckForUpdates(payload map[string]any) appcore.CommandResult {
	_ = payload
	info, err := checkGitHubReleaseForUpdate(context.Background())
	if err != nil {
		return appcore.NewCommandResult("UPDATE_CHECK_FAILED", map[string]any{
			"current_version": appVersion(),
			"release_url":     releasePageURL,
		}, err.Error(), false, nil, nil)

	}
	message := "当前已是最新版本。"
	if info.UpdateAvailable {
		message = fmt.Sprintf("发现新版本 %s。", info.LatestVersion)
	}
	return appcore.NewCommandResult("UPDATE_CHECK_OK", info, message, true, nil, nil)
}

func (a *App) DownloadAndInstallUpdate(payload map[string]any) appcore.CommandResult {
	info, err := resolveGitHubReleaseUpdate(context.Background())
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
	if result.InstallStarted {
		a.scheduleQuitAfterUpdate()
	}
	message := "更新包已下载并触发安装流程。"
	if !result.InstallStarted && strings.TrimSpace(result.NextAction) == "manual" {
		message = "更新包已下载，请按当前平台的部署方式手动安装或替换。"
	}
	return appcore.NewCommandResult("UPDATE_INSTALL_READY", result, message, true, nil, nil)
}

func (a *App) OpenReleasePage() appcore.CommandResult {
	if err := openExternalURL(releasePageURL); err != nil {
		return appcore.NewCommandResult("RELEASE_OPEN_FAILED", map[string]any{
			"release_url": releasePageURL,
		}, err.Error(), false, nil, nil)

	}
	return appcore.NewCommandResult("RELEASE_OPENED", map[string]any{
		"release_url": releasePageURL,
	}, "已打开发行页。", true, nil, nil)

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

func configFilePath() string {
	return filepath.Join(storageRoot(), "config.json")
}

func desktopConfigFilePath() string {
	return filepath.Join(storageRoot(), "desktop-config.json")
}

func debugLogFilePath() string {
	return filepath.Join(logDirectoryPath(), "cfip-log.txt")
}

func errorLogFilePath() string {
	return filepath.Join(logDirectoryPath(), "error-log.txt")
}

func logDirectoryPath() string {
	return filepath.Join(storageRoot(), "logs")
}
