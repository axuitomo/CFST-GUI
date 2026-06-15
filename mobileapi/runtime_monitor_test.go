package mobileapi

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/utils"
)

func preserveMobileProcessMonitorForTest(t *testing.T) {
	t.Helper()
	oldAllowed := mobileProcessMonitorAllowed
	oldNow := mobileProcessMonitorNow
	oldPID := mobileProcessMonitorPID
	t.Cleanup(func() {
		mobileProcessMonitorAllowed = oldAllowed
		mobileProcessMonitorNow = oldNow
		mobileProcessMonitorPID = oldPID
	})
}

func TestMobileProcessMonitorHeartbeatFollowsConfig(t *testing.T) {
	preserveMobileProcessMonitorForTest(t)

	service := NewService()
	t.Cleanup(service.stopProcessMonitorHeartbeat)
	baseDir := t.TempDir()
	now := time.Date(2026, 6, 14, 13, 0, 0, 0, time.UTC)
	mobileProcessMonitorAllowed = func() bool { return true }
	mobileProcessMonitorNow = func() time.Time { return now }
	mobileProcessMonitorPID = func() int { return 5252 }

	service.Init(baseDir)
	heartbeat, err := utils.ReadMainHeartbeat(filepath.Join(baseDir, "logs", "main-heartbeat.json"))
	if err != nil {
		t.Fatalf("read heartbeat: %v", err)
	}
	if heartbeat.PID != 5252 || heartbeat.State != utils.MainHeartbeatStateRunning || heartbeat.LogDir != filepath.Join(baseDir, "logs") {
		t.Fatalf("heartbeat = %#v", heartbeat)
	}

	service.configureRuntimeLog(map[string]any{
		"logging": map[string]any{
			"monitor_enabled": false,
		},
	})
	heartbeat, err = utils.ReadMainHeartbeat(filepath.Join(baseDir, "logs", "main-heartbeat.json"))
	if err != nil {
		t.Fatalf("read shutdown heartbeat: %v", err)
	}
	if heartbeat.State != utils.MainHeartbeatStateShutdown {
		t.Fatalf("heartbeat state = %q, want shutdown", heartbeat.State)
	}
	if _, err := os.Stat(filepath.Join(baseDir, "logs")); err != nil {
		t.Fatalf("log directory missing: %v", err)
	}
}
