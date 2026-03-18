package main

import internalruntime "wails-app/internal/runtime"

// initVFS sets up the unified VFS. Call at the end of App.startup().
func (a *App) initVFS() {
	vfs, err := internalruntime.InitVFS()
	if err != nil {
		return
	}
	a.vfs = vfs
	a.filesystem.SetVFS(a.vfs)
}

// VFSListDir lists a directory through the VFS and works for real directories
// and paths inside mounted archives.
func (a *App) VFSListDir(path string) (*DirectoryListing, error) {
	return internalruntime.VFSListDir(a.vfs, path)
}

// VFSOpenArchive mounts an archive chain into the VFS and returns the
// top-level listing for the mounted archive.
func (a *App) VFSOpenArchive(path string, nested []string) (ArchiveListing, error) {
	return internalruntime.VFSOpenArchive(a.vfs, path, nested)
}

// VFSMountArchive mounts a top-level archive and returns its VFS mount point.
func (a *App) VFSMountArchive(path string) (string, error) {
	return internalruntime.VFSMountArchive(a.vfs, path)
}

// VFSUnmount unmounts a previously mounted archive and frees its resources.
func (a *App) VFSUnmount(mountPoint string) error {
	return internalruntime.VFSUnmount(a.vfs, mountPoint)
}
