//go:build !windows

package app

import (
	"os/exec"
	"syscall"
)

func configureLogMonitorCommand(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}
