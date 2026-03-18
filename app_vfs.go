package main

// initVFS sets up the unified VFS. Call at the end of App.startup().
func (a *App) initVFS() {
	a.State.InitVFS()
}

// VFSListDir lists a directory through the VFS and works for real directories
// and paths inside mounted archives.
func (a *App) VFSListDir(path string) (*DirectoryListing, error) {
	return a.State.VFSListDir(path)
}

// VFSOpenArchive mounts an archive chain into the VFS and returns the
// top-level listing for the mounted archive.
func (a *App) VFSOpenArchive(path string, nested []string) (ArchiveListing, error) {
	return a.State.VFSOpenArchive(path, nested)
}

// VFSMountArchive mounts a top-level archive and returns its VFS mount point.
func (a *App) VFSMountArchive(path string) (string, error) {
	return a.State.VFSMountArchive(path)
}

// VFSUnmount unmounts a previously mounted archive and frees its resources.
func (a *App) VFSUnmount(mountPoint string) error {
	return a.State.VFSUnmount(mountPoint)
}
