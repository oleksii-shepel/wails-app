//go:build linux || darwin

package elevation

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

func IsProcessElevated() bool {
	return os.Getuid() == 0
}

func LaunchService(exePath string, port int, secretB64 string) (int, error) {
	args := []string{
		ServiceFlag,
		"--port", strconv.Itoa(port),
		"--secret", secretB64,
	}

	if runtime.GOOS == "linux" {
		script := detachedCommandScript(exePath, args)
		if out, err := exec.Command("pkexec", "sh", "-c", script).CombinedOutput(); err == nil {
			if pid, parseErr := parseLaunchedPID(out); parseErr == nil {
				return pid, nil
			}
			return 0, fmt.Errorf("elevation failed: could not parse elevated PID")
		}

		out, err := exec.Command("sudo", "sh", "-c", script).CombinedOutput()
		if err != nil {
			msg := strings.TrimSpace(string(out))
			if msg == "" {
				msg = err.Error()
			}
			return 0, fmt.Errorf("elevation failed (pkexec/sudo): %s", msg)
		}
		pid, parseErr := parseLaunchedPID(out)
		if parseErr != nil {
			return 0, fmt.Errorf("elevation failed: could not parse elevated PID")
		}
		return pid, nil
	}

	if runtime.GOOS == "darwin" {
		command := detachedCommandScript(exePath, args)
		script := fmt.Sprintf(`do shell script %q with administrator privileges without altering line endings`, command)
		out, err := exec.Command("osascript", "-e", script).CombinedOutput()
		if err != nil {
			msg := strings.TrimSpace(string(out))
			if msg == "" {
				msg = err.Error()
			}
			return 0, fmt.Errorf("elevation failed: %s", msg)
		}
		pid, parseErr := parseLaunchedPID(out)
		if parseErr != nil {
			return 0, fmt.Errorf("elevation failed: could not parse elevated PID")
		}
		return pid, nil
	}

	return 0, fmt.Errorf("launchElevatedService: unsupported platform %q", runtime.GOOS)
}

func LaunchProcess(exePath, reqB64, resultPath string) error {
	args := []string{HelperFlag, "--request", reqB64, "--result", resultPath}

	if runtime.GOOS == "linux" {
		if out, err := exec.Command("pkexec", append([]string{exePath}, args...)...).CombinedOutput(); err == nil {
			return nil
		} else {
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

	if runtime.GOOS == "darwin" {
		command := shellEscape(exePath)
		for _, arg := range args {
			command += " " + shellEscape(arg)
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

func detachedCommandScript(exePath string, args []string) string {
	command := shellEscape(exePath)
	for _, arg := range args {
		command += " " + shellEscape(arg)
	}
	return command + " >/dev/null 2>&1 & echo $!"
}

func parseLaunchedPID(out []byte) (int, error) {
	txt := strings.TrimSpace(string(out))
	if txt == "" {
		return 0, fmt.Errorf("empty output")
	}
	lines := strings.Split(txt, "\n")
	last := strings.TrimSpace(lines[len(lines)-1])
	pid, err := strconv.Atoi(last)
	if err != nil || pid <= 0 {
		return 0, fmt.Errorf("invalid pid output: %q", last)
	}
	return pid, nil
}