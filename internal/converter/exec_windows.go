//go:build windows

package converter

import (
	"os/exec"
	"syscall"
)

// hideWindow suppresses the console window that would otherwise flash
// briefly on screen each time an external tool (ImageMagick, LibreOffice,
// Calibre, …) is spawned on Windows.
func hideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}
