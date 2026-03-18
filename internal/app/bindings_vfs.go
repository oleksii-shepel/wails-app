package app

import contracts "wails-app/internal/contracts"

func (a *App) VFSListDir(path string) (*contracts.DirectoryListing, error) {
	return a.State.VFSListDir(path)
}

func (a *App) VFSOpenArchive(path string, nested []string) (contracts.ArchiveListing, error) {
	return a.State.VFSOpenArchive(path, nested)
}

func (a *App) VFSMountArchive(path string) (string, error) {
	return a.State.VFSMountArchive(path)
}

func (a *App) VFSUnmount(mountPoint string) error {
	return a.State.VFSUnmount(mountPoint)
}
