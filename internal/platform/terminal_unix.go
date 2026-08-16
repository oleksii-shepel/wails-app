//go:build !windows

package platform

import (
	"fmt"
	"os/exec"
	"runtime"
)

func OpenTerminalAt(path string) error {
	if runtime.GOOS == "darwin" {
		return startDetached(exec.Command("open", "-a", "Terminal", path))
	}

	candidates := []*exec.Cmd{
		exec.Command("x-terminal-emulator", "--working-directory="+path),
		exec.Command("gnome-terminal", "--working-directory="+path),
		exec.Command("konsole", "--workdir", path),
		exec.Command("xfce4-terminal", "--working-directory="+path),
		exec.Command("kitty", "--directory", path),
		exec.Command("alacritty", "--working-directory", path),
	}

	var lastErr error
	for _, cmd := range candidates {
		if _, err := exec.LookPath(cmd.Path); err != nil {
			lastErr = err
			continue
		}
		if err := startDetached(cmd); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}

	if lastErr != nil {
		return fmt.Errorf("terminal: no supported terminal emulator could be started: %w", lastErr)
	}
	return fmt.Errorf("terminal: no supported terminal emulator found")
}
