package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestExecutePaneCommands(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PATH", "")
	if err := os.WriteFile(filepath.Join(dir, "pane-fixture.txt"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	cwd := dir
	if runtime.GOOS == "windows" {
		cwd = "filesystem://" + strings.ToLower(string(dir[0])) + filepath.ToSlash(dir[2:])
	}

	for _, tc := range []struct {
		command string
		output  string
	}{
		{"ls -l", "pane-fixture.txt"},
		{"doctor", "Aether / Reassembly Environment Doctor"},
	} {
		t.Run(tc.command, func(t *testing.T) {
			result := (Services{}).Execute(tc.command, cwd, false)
			if !result.OK || !strings.Contains(result.Stdout, tc.output) {
				t.Fatalf("unexpected command result: %+v", result)
			}
		})
	}
}
