//go:build windows

package pluginpackage

import (
	"os/exec"
	"syscall"
)

func configureBackendCommand(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}
