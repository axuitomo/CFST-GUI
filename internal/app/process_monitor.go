package app

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/probecore"
	"github.com/axuitomo/CFST-GUI/internal/utils"
)

const mainHeartbeatFileName = "main-heartbeat.json"

var (
	processMonitorExecutable = os.Executable
	processMonitorPID        = os.Getpid
	processMonitorNow        = time.Now
	processMonitorStart      = startLogMonitorProcess
	processMonitorAllowed    = defaultProcessMonitorAllowed
)

func (a *App) configureDesktopObservabilityFromDisk() []string {
	snapshot, err := loadDesktopConfigSnapshotFromDisk()
	if err != nil {
		snapshot = defaultDesktopConfigSnapshot()
	}
	return a.configureDesktopObservability(snapshot)
}

func (a *App) configureDesktopObservability(config map[string]any) []string {
	cfg, warnings := probecore.ConfigSnapshotToRuntimeLogConfig(config)
	warnings = append(warnings, configureDesktopRuntimeLogConfig(cfg)...)
	warnings = append(warnings, a.configureDesktopProcessMonitor(cfg)...)
	return probecore.DedupeStrings(warnings)
}

func (a *App) configureDesktopProcessMonitor(cfg probecore.RuntimeLogConfig) []string {
	if !cfg.MonitorEnabled {
		a.disableDesktopProcessMonitor()
		return nil
	}
	if !processMonitorAllowed() {
		return nil
	}
	warnings := make([]string, 0, 2)
	if err := a.ensureMainHeartbeatLoop(); err != nil {
		warnings = append(warnings, fmt.Sprintf("初始化主进程心跳失败：%v", err))
		_ = utils.AppendErrorLog(errorLogFilePath(), "monitor.heartbeat_start_failed", map[string]any{
			"heartbeat_path": mainHeartbeatPath(),
			"message":        err.Error(),
		})
		return warnings
	}
	if err := a.ensureLogMonitorProcess(cfg.RetentionDays); err != nil {
		warnings = append(warnings, fmt.Sprintf("启动进程监控失败：%v", err))
		_ = utils.AppendErrorLog(errorLogFilePath(), "monitor.start_failed", map[string]any{
			"heartbeat_path": mainHeartbeatPath(),
			"log_dir":        logDirectoryPath(),
			"message":        err.Error(),
		})
	}
	return warnings
}

func (a *App) ensureMainHeartbeatLoop() error {
	a.processMonitorMu.Lock()
	if a.heartbeatCancel != nil {
		startedAt := a.heartbeatStartedAt
		a.processMonitorMu.Unlock()
		return writeMainHeartbeat(startedAt, utils.MainHeartbeatStateRunning)
	}
	startedAt := processMonitorNow()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	a.heartbeatCancel = cancel
	a.heartbeatDone = done
	a.heartbeatStartedAt = startedAt
	a.processMonitorMu.Unlock()

	if err := writeMainHeartbeat(startedAt, utils.MainHeartbeatStateRunning); err != nil {
		cancel()
		a.processMonitorMu.Lock()
		if a.heartbeatDone == done {
			a.heartbeatCancel = nil
			a.heartbeatDone = nil
			a.heartbeatStartedAt = time.Time{}
		}
		a.processMonitorMu.Unlock()
		return err
	}

	go func() {
		defer close(done)
		ticker := time.NewTicker(defaultLogMonitorPollInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := writeMainHeartbeat(startedAt, utils.MainHeartbeatStateRunning); err != nil {
					_ = utils.AppendErrorLog(errorLogFilePath(), "monitor.heartbeat_write_failed", map[string]any{
						"heartbeat_path": mainHeartbeatPath(),
						"message":        err.Error(),
					})
				}
			}
		}
	}()
	return nil
}

func (a *App) ensureLogMonitorProcess(retentionDays int) error {
	a.processMonitorMu.Lock()
	current := a.logMonitorCommand
	if current != nil && current.Process != nil && processAlive(current.Process.Pid) {
		a.processMonitorMu.Unlock()
		return nil
	}
	a.processMonitorMu.Unlock()

	executable, err := processMonitorExecutable()
	if err != nil {
		return err
	}
	args := logMonitorArgs(processMonitorPID(), logDirectoryPath(), mainHeartbeatPath(), defaultLogMonitorStaleAfter, retentionDays)
	cmd, err := processMonitorStart(executable, args)
	if err != nil {
		return err
	}

	a.processMonitorMu.Lock()
	a.logMonitorCommand = cmd
	a.logMonitorLastWarning = ""
	a.processMonitorMu.Unlock()

	go func() {
		_ = cmd.Wait()
	}()
	return nil
}

func (a *App) disableDesktopProcessMonitor() {
	cancel, done, startedAt, cmd := a.takeProcessMonitorState()
	if cancel != nil {
		cancel()
		waitForHeartbeatLoop(done)
		_ = writeMainHeartbeat(startedAt, utils.MainHeartbeatStateShutdown)
	}
	if cmd != nil && cmd.Process != nil && processAlive(cmd.Process.Pid) {
		_ = cmd.Process.Kill()
	}
}

func (a *App) stopProcessMonitoringForShutdown() {
	cancel, done, startedAt, _ := a.takeProcessMonitorState()
	if cancel != nil {
		cancel()
		waitForHeartbeatLoop(done)
	}
	if startedAt.IsZero() {
		startedAt = processMonitorNow()
	}
	_ = writeMainHeartbeat(startedAt, utils.MainHeartbeatStateShutdown)
	_ = utils.AppendRuntimeLogAlways(utils.LogLevelWarn, "main.shutdown", map[string]any{
		"heartbeat_path": mainHeartbeatPath(),
		"log_dir":        logDirectoryPath(),
		"pid":            processMonitorPID(),
		"state":          utils.MainHeartbeatStateShutdown,
	})
}

func (a *App) takeProcessMonitorState() (context.CancelFunc, chan struct{}, time.Time, *exec.Cmd) {
	a.processMonitorMu.Lock()
	defer a.processMonitorMu.Unlock()
	cancel := a.heartbeatCancel
	done := a.heartbeatDone
	startedAt := a.heartbeatStartedAt
	cmd := a.logMonitorCommand
	a.heartbeatCancel = nil
	a.heartbeatDone = nil
	a.heartbeatStartedAt = time.Time{}
	a.logMonitorCommand = nil
	return cancel, done, startedAt, cmd
}

func waitForHeartbeatLoop(done chan struct{}) {
	if done == nil {
		return
	}
	select {
	case <-done:
	case <-time.After(2 * time.Second):
	}
}

func writeMainHeartbeat(startedAt time.Time, state string) error {
	if startedAt.IsZero() {
		startedAt = processMonitorNow()
	}
	return utils.WriteMainHeartbeat(mainHeartbeatPath(), utils.NewMainHeartbeat(
		processMonitorPID(),
		startedAt,
		processMonitorNow(),
		state,
		logDirectoryPath(),
	))
}

func mainHeartbeatPath() string {
	return filepath.Join(logDirectoryPath(), mainHeartbeatFileName)
}

func startLogMonitorProcess(executable string, args []string) (*exec.Cmd, error) {
	cmd := exec.Command(executable, args...)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	configureLogMonitorCommand(cmd)
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd, nil
}

func defaultProcessMonitorAllowed() bool {
	if strings.TrimSpace(os.Getenv("CFST_DISABLE_PROCESS_MONITOR")) == "1" {
		return false
	}
	return !strings.HasSuffix(filepath.Base(os.Args[0]), ".test")
}
