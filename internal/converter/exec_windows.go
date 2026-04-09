//go:build windows

package converter

import (
	"os/exec"
	"syscall"
)

// hideWindow suppresses the console window that would otherwise flash
// briefly on screen each time an external converter tool is spawned on Windows.
func hideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}
