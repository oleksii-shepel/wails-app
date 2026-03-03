//go:build windows

package main

import (
	"fmt"
	"os/exec"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"
)

// isProcessElevated returns true when the current process token has admin privileges.
func isProcessElevated() bool {
	return windows.GetCurrentProcessToken().IsElevated()
}

func launchElevatedProcess(exePath, reqB64, resultPath string) error {
	return launchElevatedArgs(exePath, []string{
		elevatedHelperFlag,
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
	for _, a := range args {
		quoted = append(quoted, quoteWindowsArg(a))
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

func quotePowerShellLiteral(s string) string {
	// Single-quoted PowerShell literal; '' escapes a single quote.
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// launchElevatedService starts the elevated service helper exactly once (one
// UAC prompt) and returns immediately. The helper binds to 127.0.0.1:port and
// awaits a JWT-authenticated TCP connection from the parent process.
// Returns the PID of the elevated child process.
func launchElevatedService(exePath string, port int, secretB64 string) (int, error) {
	exePath = strings.TrimSpace(exePath)
	if exePath == "" {
		return 0, fmt.Errorf("elevation failed: executable path is empty")
	}

	args := []string{
		elevatedServiceFlag,
		"--port", fmt.Sprintf("%d", port),
		"--secret", secretB64,
	}
	quoted := make([]string, 0, len(args))
	for _, a := range args {
		quoted = append(quoted, quoteWindowsArg(a))
	}
	argLine := strings.Join(quoted, " ")

	// Start-Process -Verb RunAs triggers UAC, then returns immediately.
	// -PassThru gives us a Process object; we print its Id to stdout.
	psCommand := fmt.Sprintf(
		`$p = Start-Process -FilePath %s -ArgumentList %s -Verb RunAs -WindowStyle Hidden -PassThru; Write-Output $p.Id`,
		quotePowerShellLiteral(exePath),
		quotePowerShellLiteral(argLine),
	)

	cmd := exec.Command(
		"powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", psCommand,
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("elevation failed: %w", err)
	}

	pid := 0
	fmt.Sscanf(strings.TrimSpace(string(out)), "%d", &pid)
	return pid, nil
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
