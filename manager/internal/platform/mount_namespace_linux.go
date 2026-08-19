//go:build linux

package platform

import (
	"os"

	"golang.org/x/sys/unix"
)

// JoinMountNamespace makes the calling process join the specified mount namespace.
// The nsPath should be a path like "/proc/<pid>/ns/mnt".
func JoinMountNamespace(nsPath string) error {
	f, err := os.Open(nsPath)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := unix.Setns(int(f.Fd()), unix.CLONE_NEWNS); err != nil {
		return err
	}
	return nil
}