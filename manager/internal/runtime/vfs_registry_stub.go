//go:build !windows

package runtime

import vfslib "wails-app/vfs"

func mountRegistryFS(_ *vfslib.Unified) error {
	return nil
}
