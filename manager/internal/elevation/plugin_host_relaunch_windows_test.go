//go:build windows

package elevation

import (
	"os"
	"strings"
	"testing"
)

func TestExecutableSubsystemRejectsInvalidHeaders(t *testing.T) {
	path := t.TempDir() + `\tiny.exe`
	if err := os.WriteFile(path, []byte{1, 2, 3}, 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := executableSubsystem(path); err == nil {
		t.Fatal("expected invalid executable header to fail")
	}
}

func TestExecutableSubsystemReadsPEValue(t *testing.T) {
	path := t.TempDir() + `\gui.exe`
	data := make([]byte, 0x200)
	data[0x3C] = 0x80
	data[0x80+0x5C] = 2
	data[0x80+0x5D] = 0
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}

	subsystem, err := executableSubsystem(path)
	if err != nil {
		t.Fatal(err)
	}
	if subsystem != 2 {
		t.Fatalf("expected subsystem 2, got %d", subsystem)
	}
}

func TestRelaunchSupportsGuiAndConsolePaths(t *testing.T) {
	data, err := os.ReadFile("plugin_host_relaunch_windows.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	if !strings.Contains(source, `syscall.UTF16PtrFromString("runas")`) {
		t.Fatal("relaunch must support native runas elevation")
	}
	if !strings.Contains(source, "ShellExecuteW") {
		t.Fatal("relaunch must use ShellExecuteW for GUI builds")
	}
	if !strings.Contains(source, "Start-Process") || !strings.Contains(source, "powershell.exe") {
		t.Fatal("relaunch must preserve hidden fallback for console-subsystem builds")
	}
}
