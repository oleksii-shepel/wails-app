package runtime

import vfslib "wails-app/vfs"

func mountBuiltInFilesystems(vfs *vfslib.Unified) error {
	if err := mountRegistryFS(vfs); err != nil {
		return err
	}
	return nil
}
