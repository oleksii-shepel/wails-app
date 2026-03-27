package app

import (
	contracts "wails-app/internal/contracts"
	"wails-app/internal/pathref"
	internalplatform "wails-app/internal/platform"
)

func (a *App) GetOSInfo() contracts.OSInfoResponse {
	return internalplatform.GetOSInfo()
}

func (a *App) ListDrives() contracts.DrivesList {
	return internalplatform.ListDrives()
}

func (a *App) GetDrives() contracts.DrivesList {
	return a.ListDrives()
}

func (a *App) ListDirectory(path string, showHidden bool) (*contracts.DirectoryListing, error) {
	return a.Filesystem.ListDirectory(path, showHidden)
}

func (a *App) ListDir(path string, showHidden bool) (*contracts.DirectoryListing, error) {
	return a.ListDirectory(path, showHidden)
}

func (a *App) ListDirFiltered(path string, showHidden bool, filter contracts.FilterQuery) (*contracts.DirectoryListing, error) {
	listing, err := a.ListDirectory(path, showHidden)
	if err != nil {
		return nil, err
	}
	listing.Entries = filterEntries(listing.Entries, filter)
	return listing, nil
}

func (a *App) DirSizeRecursive(path string) (int64, error) {
	return a.Filesystem.DirSizeRecursive(path)
}

func (a *App) GetFileInfo(path string) (*contracts.FileInfo, error) {
	return a.Filesystem.GetFileInfo(path)
}

func (a *App) FileInfo(path string) (*contracts.FileInfo, error) {
	return a.GetFileInfo(path)
}

func (a *App) ReadFile(path string, encoding string) (*contracts.FileContent, error) {
	return a.Filesystem.ReadFile(path, encoding)
}

func (a *App) ReadFileContent(path string, encoding string) (*contracts.FileContent, error) {
	return a.ReadFile(path, encoding)
}

func (a *App) ProbeFile(path string) (*contracts.FileProbe, error) {
	return a.Filesystem.ProbeFile(path)
}

func (a *App) ReadFileStream(path string, chunkSize int) (string, error) {
	streamID, err := a.Filesystem.ReadFileStream(path, chunkSize, a.EmitEvent)
	return streamID, err
}

func (a *App) CancelReadFileStream(streamID string) {
	a.Filesystem.CancelReadStream(streamID)
}

func (a *App) WriteFile(path string, content string, encoding string) (*contracts.OperationResult, error) {
	return a.Filesystem.WriteFile(path, content, encoding)
}

func (a *App) WriteFileContent(path string, content string, encoding string) (*contracts.OperationResult, error) {
	return a.WriteFile(path, content, encoding)
}

func (a *App) DeleteFile(path string, recursive bool, useTrash *bool) (*contracts.OperationResult, error) {
	return a.Filesystem.DeleteFile(path, recursive, useTrash)
}

func (a *App) Delete(path string, recursive bool, useTrash *bool) (*contracts.OperationResult, error) {
	return a.DeleteFile(path, recursive, useTrash)
}

func (a *App) CreateDirectory(path string, recursive bool) (*contracts.OperationResult, error) {
	return a.Filesystem.CreateDirectory(path, recursive)
}

func (a *App) CreateDir(path string, recursive bool) (*contracts.OperationResult, error) {
	return a.CreateDirectory(path, recursive)
}

func (a *App) MoveFile(source, destination string) (*contracts.OperationResult, error) {
	return a.Filesystem.MoveFile(source, destination)
}

func (a *App) Move(source, destination string) (*contracts.OperationResult, error) {
	return a.MoveFile(source, destination)
}

func (a *App) CopyFile(source, destination string, recursive bool) (*contracts.OperationResult, error) {
	return a.Filesystem.CopyFile(source, destination, recursive)
}

func (a *App) Copy(source, destination string, recursive bool) (*contracts.OperationResult, error) {
	return a.CopyFile(source, destination, recursive)
}

func (a *App) SearchFiles(path, pattern string, recursive bool) (*contracts.SearchResult, error) {
	return a.Filesystem.SearchFiles(path, pattern, recursive)
}

func (a *App) Search(path, pattern string, recursive bool) (*contracts.SearchResult, error) {
	return a.SearchFiles(path, pattern, recursive)
}

func (a *App) BuildSearchAlias(path, pattern string, recursive bool) (string, error) {
	return pathref.BuildSearch(path, pattern, recursive)
}

func (a *App) AnalyzeDiskUsage(path string) (*contracts.DiskUsageResult, error) {
	return a.Filesystem.AnalyzeDiskUsage(path)
}

func (a *App) StartDiskUsageStream(path string) (string, error) {
	return a.Filesystem.StartDiskUsageStream(path)
}

// JoinMountNamespace instructs the backend to join a mount namespace (Unix only).
// The expected argument is the namespace path printed by memfs, e.g. /proc/<pid>/ns/mnt.
func (a *App) JoinMountNamespace(nsPath string) error {
	return internalplatform.JoinMountNamespace(nsPath)
}

func (a *App) CancelDiskUsageStream(scanID string) {
	a.Filesystem.CancelDiskUsageStream(scanID)
}
