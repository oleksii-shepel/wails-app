//go:build !windows

package memory

import (
	"fmt"
	"os"
	"runtime"
)

// CheckFUSEAvailable returns nil if a working FUSE installation is detected for
// the current platform, or a descriptive error if it cannot be found.
func CheckFUSEAvailable() error {
	switch runtime.GOOS {
	case "darwin":
		if _, err := os.Stat("/Library/Filesystems/macfuse.fs"); err == nil {
			return nil
		}
		if _, err := os.Stat("/Library/Filesystems/osxfuse.fs"); err == nil {
			return nil
		}
		return fmt.Errorf("macFUSE not found: install macFUSE from https://osxfuse.github.io/")
	default:
		// Linux and other Unix: check for /dev/fuse.
		if _, err := os.Stat("/dev/fuse"); err != nil {
			return fmt.Errorf("FUSE not available: /dev/fuse not found")
		}
		return nil
	}
}
