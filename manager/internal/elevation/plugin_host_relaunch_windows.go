//go:build windows

package elevation

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	shellExecuteSuccessThreshold = 32
	subsystemConsole             = 3
	swShowNormal                 = 1
)

func RelaunchCurrentProcessElevated() error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve current executable: %w", err)
	}

	quoted := make([]string, 0, len(os.Args)-1)
	for _, arg := range os.Args[1:] {
		quoted = append(quoted, quoteWindowsArg(arg))
	}
	argLine := strings.Join(quoted, " ")

	subsystem, err := executableSubsystem(exePath)
	if err != nil {
		return err
	}
	if subsystem == subsystemConsole {
		return relaunchConsoleBinaryElevated(exePath, argLine)
	}
	return relaunchGUIBinaryElevated(exePath, argLine)
}

func relaunchGUIBinaryElevated(exePath, argLine string) error {
	verb, err := syscall.UTF16PtrFromString("runas")
	if err != nil {
		return fmt.Errorf("encode elevation verb: %w", err)
	}
	target, err := syscall.UTF16PtrFromString(exePath)
	if err != nil {
		return fmt.Errorf("encode executable path: %w", err)
	}
	var parameters *uint16
	if argLine != "" {
		parameters, err = syscall.UTF16PtrFromString(argLine)
		if err != nil {
			return fmt.Errorf("encode command line: %w", err)
		}
	}
	var workingDir *uint16
	if cwd, cwdErr := os.Getwd(); cwdErr == nil && strings.TrimSpace(cwd) != "" {
		workingDir, err = syscall.UTF16PtrFromString(cwd)
		if err != nil {
			return fmt.Errorf("encode working directory: %w", err)
		}
	}

	shell32 := windows.NewLazySystemDLL("shell32.dll")
	shellExecute := shell32.NewProc("ShellExecuteW")
	ret, _, callErr := shellExecute.Call(
		0,
		uintptr(unsafe.Pointer(verb)),
		uintptr(unsafe.Pointer(target)),
		uintptr(unsafe.Pointer(parameters)),
		uintptr(unsafe.Pointer(workingDir)),
		swShowNormal,
	)
	if ret <= shellExecuteSuccessThreshold {
		if callErr != nil && callErr != syscall.Errno(0) {
			return fmt.Errorf("elevation failed: %w", callErr)
		}
		return fmt.Errorf("elevation failed with ShellExecuteW code %d", ret)
	}
	return nil
}

func relaunchConsoleBinaryElevated(exePath, argLine string) error {
	psCommand := fmt.Sprintf(
		`$ErrorActionPreference='Stop'; Start-Process -FilePath %s -Verb RunAs -WindowStyle Hidden`,
		quotePowerShellLiteral(exePath),
	)
	if argLine != "" {
		psCommand = fmt.Sprintf(
			`$ErrorActionPreference='Stop'; Start-Process -FilePath %s -ArgumentList %s -Verb RunAs -WindowStyle Hidden`,
			quotePowerShellLiteral(exePath),
			quotePowerShellLiteral(argLine),
		)
	}

	cmd := exec.Command(
		"powershell.exe",
		"-NoProfile",
		"-NonInteractive",
		"-ExecutionPolicy", "Bypass",
		"-Command", psCommand,
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if cwd, err := os.Getwd(); err == nil && strings.TrimSpace(cwd) != "" {
		cmd.Dir = cwd
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		message := strings.TrimSpace(string(out))
		if message == "" {
			message = err.Error()
		}
		return fmt.Errorf("elevation failed: %s", message)
	}
	return nil
}

func executableSubsystem(path string) (uint16, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("read executable header: %w", err)
	}
	if len(data) < 0x40 {
		return 0, fmt.Errorf("read executable header: file too small")
	}
	peOffset := int(uint32(data[0x3C]) | uint32(data[0x3D])<<8 | uint32(data[0x3E])<<16 | uint32(data[0x3F])<<24)
	subsystemOffset := peOffset + 0x5C
	if peOffset < 0 || subsystemOffset+1 >= len(data) {
		return 0, fmt.Errorf("read executable header: invalid PE header")
	}
	subsystem := uint16(data[subsystemOffset]) | uint16(data[subsystemOffset+1])<<8
	return subsystem, nil
}
