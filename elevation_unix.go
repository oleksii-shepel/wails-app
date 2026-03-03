//go:build linux || darwin

package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

// isProcessElevated returns true when the current process is running as root.
func isProcessElevated() bool {
	return os.Getuid() == 0
}

// launchElevatedService launches this executable as an elevated background
// service and returns the PID of the launched process.
// On Linux it uses pkexec (GUI prompt) or falls back to sudo.
// On macOS it uses osascript's administrator privileges prompt.
func launchElevatedService(exePath string, port int, secretB64 string) (int, error) {
	args := []string{
		elevatedServiceFlag,
		"--port", strconv.Itoa(port),
		"--secret", secretB64,
	}

	if runtime.GOOS == "linux" {
		cmd := exec.Command("pkexec", append([]string{exePath}, args...)...)
		if err := cmd.Start(); err == nil {
			return cmd.Process.Pid, nil
		}
		// Fall back to sudo (will show terminal password prompt if no GUI agent).
		cmd = exec.Command("sudo", append([]string{exePath}, args...)...)
		if err := cmd.Start(); err != nil {
			return 0, fmt.Errorf("elevation failed (pkexec and sudo both unavailable): %w", err)
		}
		return cmd.Process.Pid, nil
	}

	if runtime.GOOS == "darwin" {
		command := shellEscape(exePath)
		for _, a := range args {
			command += " " + shellEscape(a)
		}
		script := fmt.Sprintf(`do shell script %q with administrator privileges without altering line endings`, command)
		// osascript blocks until the script exits — run asynchronously.
		go exec.Command("osascript", "-e", script).Run() //nolint:errcheck
		return -1, nil
	}

	return 0, fmt.Errorf("launchElevatedService: unsupported platform %q", runtime.GOOS)
}

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
