package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"wails-app/internal/filesystem"
)

func TestExecuteCliCommandUsesHostZipService(t *testing.T) {
	t.Setenv("PATH", "")
	host := &App{State: &State{Filesystem: filesystem.NewService(filesystem.ServiceConfig{})}}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("from the file manager"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, command := range []string{"archive zip bundle.zip file.txt", "archive list bundle.zip"} {
		result, err := host.ExecuteCliCommand(command, dir, false)
		if err != nil || !result.OK || result.Stdout == "" {
			t.Fatalf("%s: %+v, %v", command, result, err)
		}
		if strings.HasPrefix(command, "archive list") && !strings.Contains(result.Stdout, "file.txt") {
			t.Fatalf("missing ZIP entry: %+v", result)
		}
	}
}
