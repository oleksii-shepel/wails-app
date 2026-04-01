package memory

import (
	"errors"
	"testing"
)

func TestDefaultRootProviderReconnectsToRunningSidecar(t *testing.T) {
	t.Setenv("WAILS_MEMFS_ROOT", "")
	t.Setenv("MEMFS_HIDDEN_PATH", "")

	prev := connectionInfoFn
	connectionInfoFn = func() (*ConnectionInfo, error) {
		return &ConnectionInfo{HiddenPath: "memory://ignored", Pipe: "test", PID: 42, Platform: "test"}, nil
	}
	t.Cleanup(func() {
		connectionInfoFn = prev
	})

	got := DefaultRootProvider()
	if got != "memory://ignored" {
		t.Fatalf("DefaultRootProvider should use running sidecar hidden path, got %q", got)
	}
}

func TestDefaultRootProviderPrefersExplicitEnv(t *testing.T) {
	t.Setenv("WAILS_MEMFS_ROOT", `C:\explicit-root`)
	t.Setenv("MEMFS_HIDDEN_PATH", `C:\hidden-root`)

	prev := connectionInfoFn
	connectionInfoFn = func() (*ConnectionInfo, error) {
		return nil, errors.New("should not be called when env overrides are set")
	}
	t.Cleanup(func() {
		connectionInfoFn = prev
	})

	got := DefaultRootProvider()
	if got != `C:\explicit-root` {
		t.Fatalf("DefaultRootProvider should prefer WAILS_MEMFS_ROOT, got %q", got)
	}
}
