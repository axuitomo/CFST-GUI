package mobileapi

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/utils"
)

const mobileMainHeartbeatFileName = "main-heartbeat.json"

var (
	mobileProcessMonitorAllowed = defaultMobileProcessMonitorAllowed
	mobileProcessMonitorNow     = time.Now
	mobileProcessMonitorPID     = os.Getpid
)

func (s *Service) configureProcessMonitor(enabled bool) []string {
	if !enabled || !mobileProcessMonitorAllowed() {
		s.stopProcessMonitorHeartbeat()
		return nil
	}
	if err := s.ensureProcessMonitorHeartbeat(); err != nil {
		_ = utils.AppendErrorLog(s.errorLogPath(), "monitor.heartbeat_start_failed", map[string]any{
			"heartbeat_path": s.mainHeartbeatPath(),
			"message":        err.Error(),
		})
		return []string{"初始化主进程心跳失败：" + err.Error()}
	}
	return nil
}

func (s *Service) ensureProcessMonitorHeartbeat() error {
	s.processMonitorMu.Lock()
	if s.heartbeatCancel != nil {
		startedAt := s.heartbeatStartedAt
		s.processMonitorMu.Unlock()
		return s.writeMainHeartbeat(startedAt, utils.MainHeartbeatStateRunning)
	}
	startedAt := mobileProcessMonitorNow()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	s.heartbeatCancel = cancel
	s.heartbeatDone = done
	s.heartbeatStartedAt = startedAt
	s.processMonitorMu.Unlock()

	if err := s.writeMainHeartbeat(startedAt, utils.MainHeartbeatStateRunning); err != nil {
		cancel()
		s.processMonitorMu.Lock()
		if s.heartbeatDone == done {
			s.heartbeatCancel = nil
			s.heartbeatDone = nil
			s.heartbeatStartedAt = time.Time{}
		}
		s.processMonitorMu.Unlock()
		return err
	}

	go func() {
		defer close(done)
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_ = s.writeMainHeartbeat(startedAt, utils.MainHeartbeatStateRunning)
			}
		}
	}()
	return nil
}

func (s *Service) stopProcessMonitorHeartbeat() {
	s.processMonitorMu.Lock()
	cancel := s.heartbeatCancel
	done := s.heartbeatDone
	startedAt := s.heartbeatStartedAt
	s.heartbeatCancel = nil
	s.heartbeatDone = nil
	s.heartbeatStartedAt = time.Time{}
	s.processMonitorMu.Unlock()
	if cancel == nil {
		return
	}
	cancel()
	if done != nil {
		select {
		case <-done:
		case <-time.After(2 * time.Second):
		}
	}
	_ = s.writeMainHeartbeat(startedAt, utils.MainHeartbeatStateShutdown)
}

func (s *Service) writeMainHeartbeat(startedAt time.Time, state string) error {
	if startedAt.IsZero() {
		startedAt = mobileProcessMonitorNow()
	}
	return utils.WriteMainHeartbeat(s.mainHeartbeatPath(), utils.NewMainHeartbeat(
		mobileProcessMonitorPID(),
		startedAt,
		mobileProcessMonitorNow(),
		state,
		s.logDirectoryPath(),
	))
}

func (s *Service) mainHeartbeatPath() string {
	return filepath.Join(s.logDirectoryPath(), mobileMainHeartbeatFileName)
}

func defaultMobileProcessMonitorAllowed() bool {
	return !strings.HasSuffix(filepath.Base(os.Args[0]), ".test")
}
