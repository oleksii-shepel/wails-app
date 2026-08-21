//go:build windows

package app

import (
	"os"
	"strings"
	"testing"
)

func TestEnvironmentSurfaceIsNative(t *testing.T) {
	source, err := os.ReadFile("bindings_system_surface_windows.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)

	if !strings.Contains(text, `normalizedKind == "environment"`) {
		t.Fatal("SystemSurfaceList must special-case environment")
	}
	if !strings.Contains(text, "platform.ListEnvironmentSurface()") {
		t.Fatal("Environment must use native platform.ListEnvironmentSurface")
	}

	scriptStart := strings.Index(text, "func systemSurfaceScript")
	if scriptStart < 0 {
		t.Fatal("systemSurfaceScript missing")
	}
	if strings.Contains(text[scriptStart:], `"environment":`) {
		t.Fatal("Environment must not have a PowerShell script entry")
	}
}
