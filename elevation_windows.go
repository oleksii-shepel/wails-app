//go:build windows

package main

import (
	"fmt"
	"os/exec"
	"strings"
)

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
		`$ErrorActionPreference='Stop'; $p = Start-Process -FilePath %s -ArgumentList %s -Verb RunAs -PassThru -Wait; exit $p.ExitCode`,
		quotePowerShellLiteral(exePath),
		quotePowerShellLiteral(argLine),
	)

	cmd := exec.Command("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", psCommand)
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
