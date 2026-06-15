package app

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/utils"
)

const (
	defaultLogMonitorPollInterval = 2 * time.Second
	defaultLogMonitorStaleAfter   = 10 * time.Second
)

type logMonitorOptions struct {
	HeartbeatPath string
	LogDir        string
	ParentPID     int
	PollInterval  time.Duration
	RetentionDays int
	StaleAfter    time.Duration
}

type logMonitorState struct {
	heartbeatUnavailable bool
	hung                 bool
}

var (
	logMonitorNow          = time.Now
	logMonitorProcessAlive = processAlive
)

func shouldRunLogMonitor(args []string) bool {
	for _, arg := range args {
		if arg == "--log-monitor" {
			return true
		}
	}
	return false
}

func runLogMonitorFromArgs(ctx context.Context, args []string) int {
	flags := flag.NewFlagSet("log-monitor", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	enabled := flags.Bool("log-monitor", false, "")
	parentPID := flags.Int("parent-pid", 0, "")
	logDir := flags.String("log-dir", "", "")
	heartbeatPath := flags.String("heartbeat", "", "")
	retentionDays := flags.Int("retention-days", utils.DefaultRuntimeLogRetentionDays, "")
	staleAfterRaw := flags.String("stale-after", defaultLogMonitorStaleAfter.String(), "")
	if err := flags.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "log monitor args invalid: %v\n", err)
		return 2
	}
	staleAfter, err := time.ParseDuration(strings.TrimSpace(*staleAfterRaw))
	if err != nil || staleAfter <= 0 {
		fmt.Fprintf(os.Stderr, "log monitor stale-after invalid: %s\n", *staleAfterRaw)
		return 2
	}
	if !*enabled {
		fmt.Fprintln(os.Stderr, "log monitor flag is required")
		return 2
	}
	if err := runLogMonitor(ctx, logMonitorOptions{
		HeartbeatPath: strings.TrimSpace(*heartbeatPath),
		LogDir:        strings.TrimSpace(*logDir),
		ParentPID:     *parentPID,
		PollInterval:  defaultLogMonitorPollInterval,
		RetentionDays: *retentionDays,
		StaleAfter:    staleAfter,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "log monitor failed: %v\n", err)
		return 1
	}
	return 0
}

func runLogMonitor(ctx context.Context, options logMonitorOptions) error {
	if options.ParentPID <= 0 {
		return fmt.Errorf("parent pid is required")
	}
	if strings.TrimSpace(options.LogDir) == "" {
		return fmt.Errorf("log dir is required")
	}
	if strings.TrimSpace(options.HeartbeatPath) == "" {
		return fmt.Errorf("heartbeat path is required")
	}
	if options.PollInterval <= 0 {
		options.PollInterval = defaultLogMonitorPollInterval
	}
	if options.StaleAfter <= 0 {
		options.StaleAfter = defaultLogMonitorStaleAfter
	}
	options.RetentionDays = utils.NormalizeRuntimeLogRetentionDays(options.RetentionDays)

	if err := utils.AppendMonitorLogWithRetention(options.LogDir, options.RetentionDays, "monitor.started", map[string]any{
		"heartbeat_path": options.HeartbeatPath,
		"pid":            options.ParentPID,
		"retention_days": options.RetentionDays,
		"stale_after_ms": options.StaleAfter.Milliseconds(),
	}); err != nil {
		return err
	}
	state := &logMonitorState{}
	if stop, err := state.check(options); stop || err != nil {
		return err
	}
	ticker := time.NewTicker(options.PollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			stop, err := state.check(options)
			if stop || err != nil {
				return err
			}
		}
	}
}

func (s *logMonitorState) check(options logMonitorOptions) (bool, error) {
	now := logMonitorNow()
	alive := logMonitorProcessAlive(options.ParentPID)
	heartbeat, heartbeatErr := utils.ReadMainHeartbeat(options.HeartbeatPath)

	if !alive {
		event := "main.crashed_or_killed"
		if heartbeatErr == nil && strings.TrimSpace(heartbeat.State) == utils.MainHeartbeatStateShutdown {
			event = "main.exited"
		}
		return true, utils.AppendMonitorLogWithRetention(options.LogDir, options.RetentionDays, event, map[string]any{
			"heartbeat_error": heartbeatErrorMessage(heartbeatErr),
			"last_seen_at":    heartbeat.LastSeenAt,
			"pid":             options.ParentPID,
			"state":           heartbeat.State,
		})
	}

	if heartbeatErr != nil {
		if s.heartbeatUnavailable {
			return false, nil
		}
		s.heartbeatUnavailable = true
		return false, utils.AppendMonitorLogWithRetention(options.LogDir, options.RetentionDays, "main.heartbeat_unavailable", map[string]any{
			"message": heartbeatErr.Error(),
			"pid":     options.ParentPID,
		})
	}
	s.heartbeatUnavailable = false

	if utils.MainHeartbeatStale(heartbeat, now, options.StaleAfter) {
		if s.hung {
			return false, nil
		}
		s.hung = true
		return false, utils.AppendMonitorLogWithRetention(options.LogDir, options.RetentionDays, "main.hung", map[string]any{
			"last_seen_at":      heartbeat.LastSeenAt,
			"pid":               options.ParentPID,
			"stale_after_ms":    options.StaleAfter.Milliseconds(),
			"stale_duration_ms": now.Sub(mustHeartbeatLastSeen(heartbeat)).Milliseconds(),
			"state":             heartbeat.State,
		})
	}
	if s.hung {
		s.hung = false
		return false, utils.AppendMonitorLogWithRetention(options.LogDir, options.RetentionDays, "main.recovered", map[string]any{
			"last_seen_at": heartbeat.LastSeenAt,
			"pid":          options.ParentPID,
			"state":        heartbeat.State,
		})
	}
	return false, nil
}

func heartbeatErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func mustHeartbeatLastSeen(heartbeat utils.MainHeartbeat) time.Time {
	parsed, err := utils.MainHeartbeatLastSeen(heartbeat)
	if err != nil {
		return logMonitorNow()
	}
	return parsed
}

func logMonitorArgs(parentPID int, logDir string, heartbeatPath string, staleAfter time.Duration, retentionDays int) []string {
	return []string{
		"--log-monitor",
		"--parent-pid", strconv.Itoa(parentPID),
		"--log-dir", logDir,
		"--heartbeat", heartbeatPath,
		"--retention-days", strconv.Itoa(utils.NormalizeRuntimeLogRetentionDays(retentionDays)),
		"--stale-after", staleAfter.String(),
	}
}
