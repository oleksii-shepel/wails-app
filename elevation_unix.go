//go:build linux || darwin

package main

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

func launchElevatedProcess(exePath, reqB64, resultPath string) error {
	args := []string{elevatedHelperFlag, "--request", reqB64, "--result", resultPath}

	// Linux: prefer pkexec (GUI prompt), fallback sudo.
	if runtime.GOOS == "linux" {
		if out, err := exec.Command("pkexec", append([]string{exePath}, args...)...).CombinedOutput(); err == nil {
			return nil
		} else {
			// If sudo succeeds, accept it. If both fail, return the sudo error below.
			_, _ = out, err
		}

		out, err := exec.Command("sudo", append([]string{exePath}, args...)...).CombinedOutput()
		if err != nil {
			msg := strings.TrimSpace(string(out))
			if msg == "" {
				msg = err.Error()
			}
			return fmt.Errorf("elevation failed: %s", msg)
		}
		return nil
	}

	// macOS: AppleScript privilege prompt.
	if runtime.GOOS == "darwin" {
		command := shellEscape(exePath)
		for _, a := range args {
			command += " " + shellEscape(a)
		}

		script := fmt.Sprintf(`do shell script %q with administrator privileges`, command)
		out, err := exec.Command("osascript", "-e", script).CombinedOutput()
		if err != nil {
			msg := strings.TrimSpace(string(out))
			if msg == "" {
				msg = err.Error()
			}
			return fmt.Errorf("elevation failed: %s", msg)
		}
		return nil
	}

	return fmt.Errorf("unsupported platform for elevation")
}

func shellEscape(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
