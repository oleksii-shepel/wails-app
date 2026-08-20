//go:build windows

package runtime

import (
	vfslib "wails-app/vfs"
	registryfs "wails-app/vfs/registry"
)

const registryMountPoint = "/registry"

func mountRegistryFS(vfs *vfslib.Unified) error {
	registry, err := registryfs.New()
	if err != nil {
		return err
	}
	_, err = vfs.Mount(registryMountPoint, registry, false, "Registry", vfslib.VolumeRegistry)
	return err
}
