//go:build windows

package app

import (
	"os/exec"
	"syscall"
)

const windowsCreateNewProcessGroup = 0x00000200

func configureLogMonitorCommand(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windowsCreateNewProcessGroup}
}
