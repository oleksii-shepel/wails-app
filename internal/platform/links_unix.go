//go:build !windows

package platform

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
)

func OpenFileWithDefaultApp(path string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	return startDetached(cmd)
}

func SymlinkPointsToDir(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

func IsUntrackedDirectoryJunction(_ string) bool {
	return false
}

func ResolveLinkPath(symlinkPath string) (target string, isDir bool, err error) {
	resolved, err := filepath.EvalSymlinks(symlinkPath)
	if err != nil {
		return "", false, err
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", false, err
	}
	return resolved, info.IsDir(), nil
}

func startDetached(cmd *exec.Cmd) error {
	cmd.Stdin = nil
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
