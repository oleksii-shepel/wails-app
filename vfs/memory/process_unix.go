//go:build !windows

package memory

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"time"
)

// ProcessManager manages the lifecycle of the memfs sidecar process on Unix.
type ProcessManager struct {
	execPath  string
	mountPath string
	cmd       *exec.Cmd
}

// NewUnixProcessManager creates a manager that will mount memfs at mountPath
// using the binary found at execPath.
func NewUnixProcessManager(execPath, mountPath string) *ProcessManager {
	return &ProcessManager{execPath: execPath, mountPath: mountPath}
}

// DefaultUnixMountPath returns a per-user temp directory path for the given app name.
func DefaultUnixMountPath(appName string) string {
	return filepath.Join(os.TempDir(), appName+"-memfs-"+strconv.Itoa(os.Getuid()))
}

// Start prepares the mount directory with 0700 permissions, launches the
// memfs process with -owner-only, and waits up to timeout for the FUSE
// mount to become visible.
func (m *ProcessManager) Start(timeout time.Duration) error {
	if err := os.MkdirAll(m.mountPath, 0700); err != nil {
		return fmt.Errorf("create memfs mount dir: %w", err)
	}
	if err := os.Chmod(m.mountPath, 0700); err != nil {
		return fmt.Errorf("chmod memfs mount dir: %w", err)
	}
	m.cmd = exec.Command(m.execPath, "-mount", m.mountPath, "-owner-only")
	// Put the sidecar in its own process group so a SIGTERM targets only it.
	m.cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := m.cmd.Start(); err != nil {
		return fmt.Errorf("start memfs process: %w", err)
	}
	return m.WaitReady(timeout)
}

// Stop sends SIGTERM to the memfs process group and waits up to 5 s for it
// to exit cleanly, then kills it if it has not exited.
func (m *ProcessManager) Stop() {
	if m.cmd == nil || m.cmd.Process == nil {
		return
	}
	_ = syscall.Kill(-m.cmd.Process.Pid, syscall.SIGTERM)
	done := make(chan struct{}, 1)
	go func() {
		_ = m.cmd.Wait()
		done <- struct{}{}
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		_ = m.cmd.Process.Kill()
	}
}

// WaitReady polls the mount directory until its device ID differs from the
// parent directory, which signals that a FUSE filesystem has been mounted.
func (m *ProcessManager) WaitReady(timeout time.Duration) error {
	parent := filepath.Dir(m.mountPath)
	parentInfo, err := os.Stat(parent)
	if err != nil {
		return fmt.Errorf("stat memfs mount parent %s: %w", parent, err)
	}
	parentDev := parentInfo.Sys().(*syscall.Stat_t).Dev

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if state := m.cmd.ProcessState; state != nil && state.Exited() {
			return fmt.Errorf("memfs process exited before mount became ready")
		}
		if info, statErr := os.Stat(m.mountPath); statErr == nil {
			if info.Sys().(*syscall.Stat_t).Dev != parentDev {
				return nil
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("memfs mount at %s did not become ready within %s", m.mountPath, timeout)
}
