//go:build windows

package elevation

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"
)

func IsProcessElevated() bool {
	return windows.GetCurrentProcessToken().IsElevated()
}

func LaunchProcess(exePath, reqB64, resultPath string) error {
	return launchElevatedArgs(exePath, []string{
		HelperFlag,
		"--request", reqB64,
		"--result", resultPath,
	})
}

func launchElevatedArgs(exePath string, args []string) error {
	exePath = strings.TrimSpace(exePath)
	if exePath == "" {
		return fmt.Errorf("elevation failed: executable path is empty")
	}

	quoted := make([]string, 0, len(args))
	for _, arg := range args {
		quoted = append(quoted, quoteWindowsArg(arg))
	}
	argLine := strings.Join(quoted, " ")

	psCommand := fmt.Sprintf(
		`$ErrorActionPreference='Stop'; $p = Start-Process -FilePath %s -ArgumentList %s -Verb RunAs -WindowStyle Hidden -PassThru -Wait; exit $p.ExitCode`,
		quotePowerShellLiteral(exePath),
		quotePowerShellLiteral(argLine),
	)

	cmd := exec.Command("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", psCommand)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("elevation failed: %s", msg)
	}
	return nil
}

func LaunchService(exePath string, port int, secretB64 string) (int, error) {
	exePath = strings.TrimSpace(exePath)
	if exePath == "" {
		return 0, fmt.Errorf("elevation failed: executable path is empty")
	}

	args := []string{
		ServiceFlag,
		"--port", fmt.Sprintf("%d", port),
		"--secret", secretB64,
	}
	quoted := make([]string, 0, len(args))
	for _, arg := range args {
		quoted = append(quoted, quoteWindowsArg(arg))
	}
	argLine := strings.Join(quoted, " ")

	psCommand := fmt.Sprintf(
		`$p = Start-Process -FilePath %s -ArgumentList %s -Verb RunAs -WindowStyle Hidden -PassThru; Write-Output $p.Id`,
		quotePowerShellLiteral(exePath),
		quotePowerShellLiteral(argLine),
	)

	cmd := exec.Command("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", psCommand)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("elevation failed: %w", err)
	}

	pid, parseErr := strconv.Atoi(strings.TrimSpace(string(out)))
	if parseErr != nil {
		return 0, fmt.Errorf("elevation: unexpected launcher output %q: %w", strings.TrimSpace(string(out)), parseErr)
	}
	return pid, nil
}

func quotePowerShellLiteral(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

func quoteWindowsArg(s string) string {
	if s == "" {
		return `""`
	}
	needsQuotes := strings.ContainsAny(s, " \t\n\"")
	if !needsQuotes {
		return s
	}

	var b strings.Builder
	b.WriteByte('"')
	backslashes := 0
	for _, r := range s {
		if r == '\\' {
			backslashes++
			continue
		}
		if r == '"' {
			b.WriteString(strings.Repeat(`\`, backslashes*2+1))
			b.WriteRune(r)
			backslashes = 0
			continue
		}
		if backslashes > 0 {
			b.WriteString(strings.Repeat(`\`, backslashes))
			backslashes = 0
		}
		b.WriteRune(r)
	}
	if backslashes > 0 {
		b.WriteString(strings.Repeat(`\`, backslashes*2))
	}
	b.WriteByte('"')
	return b.String()
}