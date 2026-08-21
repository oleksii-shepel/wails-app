//go:build windows

package app

import (
	"os"
	"strings"
	"testing"
)

func TestNetworkSurfaceIsNotPowerShellBacked(t *testing.T) {
	source, err := os.ReadFile("bindings_system_surface_windows.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)

	if !strings.Contains(text, `if normalizedKind == "network"`) {
		t.Fatal("SystemSurfaceList must special-case network")
	}
	if !strings.Contains(text, "platform.ListNetworkSurface()") {
		t.Fatal("network must call native platform.ListNetworkSurface")
	}

	scriptStart := strings.Index(text, "func systemSurfaceScript")
	if scriptStart < 0 {
		t.Fatal("systemSurfaceScript missing")
	}
	if strings.Contains(text[scriptStart:], `"network":`) {
		t.Fatal("network must not have a PowerShell script entry")
	}
}
