package app

import contracts "wails-app/internal/contracts"

func (a *App) VFSListDir(path string) (*contracts.DirectoryListing, error) {
	return a.State.VFSListDir(path)
}

func (a *App) VFSReadFile(path string, encoding string) (*contracts.FileContent, error) {
	return a.State.VFSReadFile(path, encoding)
}

func (a *App) VFSWriteFile(path string, content string, encoding string) (*contracts.OperationResult, error) {
	return a.State.VFSWriteFile(path, content, encoding)
}

func (a *App) VFSMkdir(path string) (*contracts.OperationResult, error) {
	return a.State.VFSMkdir(path)
}

func (a *App) VFSRemove(path string) (*contracts.OperationResult, error) {
	return a.State.VFSRemove(path)
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
