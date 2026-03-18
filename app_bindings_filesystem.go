package main

import internalplatform "wails-app/internal/platform"

func (a *App) GetOSInfo() OSInfoResponse {
	return internalplatform.GetOSInfo()
}

func (a *App) ListDrives() DrivesList {
	return internalplatform.ListDrives()
}

func (a *App) GetDrives() DrivesList {
	return a.ListDrives()
}

func (a *App) ListDirectory(path string, showHidden bool) (*DirectoryListing, error) {
	return a.filesystem.ListDirectory(path, showHidden)
}

func (a *App) ListDir(path string, showHidden bool) (*DirectoryListing, error) {
	return a.ListDirectory(path, showHidden)
}

func (a *App) ListDirFiltered(path string, showHidden bool, filter FilterQuery) (*DirectoryListing, error) {
	listing, err := a.ListDirectory(path, showHidden)
	if err != nil {
		return nil, err
	}
	listing.Entries = filterEntries(listing.Entries, filter)
	return listing, nil
}

func (a *App) DirSizeRecursive(path string) (int64, error) {
	return a.filesystem.DirSizeRecursive(path)
}

func (a *App) GetFileInfo(path string) (*FileInfo, error) {
	return a.filesystem.GetFileInfo(path)
}

func (a *App) FileInfo(path string) (*FileInfo, error) {
	return a.GetFileInfo(path)
}

func (a *App) ReadFile(path string, encoding string) (*FileContent, error) {
	return a.filesystem.ReadFile(path, encoding)
}

func (a *App) ReadFileContent(path string, encoding string) (*FileContent, error) {
	return a.ReadFile(path, encoding)
}

func (a *App) WriteFile(path string, content string, encoding string) (*OperationResult, error) {
	return a.filesystem.WriteFile(path, content, encoding)
}

func (a *App) WriteFileContent(path string, content string, encoding string) (*OperationResult, error) {
	return a.WriteFile(path, content, encoding)
}

func (a *App) DeleteFile(path string, recursive bool, useTrash *bool) (*OperationResult, error) {
	return a.filesystem.DeleteFile(path, recursive, useTrash)
}

func (a *App) Delete(path string, recursive bool, useTrash *bool) (*OperationResult, error) {
	return a.DeleteFile(path, recursive, useTrash)
}

func (a *App) CreateDirectory(path string, recursive bool) (*OperationResult, error) {
	return a.filesystem.CreateDirectory(path, recursive)
}

func (a *App) CreateDir(path string, recursive bool) (*OperationResult, error) {
	return a.CreateDirectory(path, recursive)
}

func (a *App) MoveFile(source, destination string) (*OperationResult, error) {
	return a.filesystem.MoveFile(source, destination)
}

func (a *App) Move(source, destination string) (*OperationResult, error) {
	return a.MoveFile(source, destination)
}

func (a *App) CopyFile(source, destination string, recursive bool) (*OperationResult, error) {
	return a.filesystem.CopyFile(source, destination, recursive)
}

func (a *App) Copy(source, destination string, recursive bool) (*OperationResult, error) {
	return a.CopyFile(source, destination, recursive)
}

func (a *App) SearchFiles(path, pattern string, recursive bool) (*SearchResult, error) {
	return a.filesystem.SearchFiles(path, pattern, recursive)
}

func (a *App) Search(path, pattern string, recursive bool) (*SearchResult, error) {
	return a.SearchFiles(path, pattern, recursive)
}

func (a *App) AnalyzeDiskUsage(path string) (*DiskUsageResult, error) {
	return a.filesystem.AnalyzeDiskUsage(path)
}

func (a *App) StartDiskUsageStream(path string) (string, error) {
	return a.filesystem.StartDiskUsageStream(path)
}

func (a *App) CancelDiskUsageStream(scanId string) {
	a.filesystem.CancelDiskUsageStream(scanId)
}
