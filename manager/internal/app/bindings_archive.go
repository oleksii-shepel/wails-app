package app

import contracts "wails-app/internal/contracts"

func (a *App) ArchiveBrowse(fsPath string, nestedChain []string, innerPath string) (contracts.ArchiveListing, error) {
	return a.Archive.Browse(fsPath, nestedChain, innerPath)
}

func (a *App) ArchiveRead(fsPath string, nestedChain []string, entryPath string, encoding string) (*contracts.FileContent, error) {
	return a.Archive.ReadFile(fsPath, nestedChain, entryPath, encoding)
}

func (a *App) ProbeArchiveFile(fsPath string, nestedChain []string, entryPath string) (*contracts.FileProbe, error) {
	return a.Archive.ProbeFile(fsPath, nestedChain, entryPath)
}

func (a *App) ArchiveReadStream(fsPath string, nestedChain []string, entryPath string, chunkSize int) (string, error) {
	streamID, err := a.Archive.ReadFileStream(fsPath, nestedChain, entryPath, chunkSize, a.EmitEvent)
	return streamID, err
}

func (a *App) CancelArchiveReadStream(streamID string) {
	a.Archive.CancelReadStream(streamID)
}

func (a *App) ArchiveCopyEntries(req contracts.ArchiveEntriesRequest) (*contracts.ArchiveEntriesResult, error) {
	return a.Archive.CopyEntries(req)
}

func (a *App) ArchiveMoveEntries(req contracts.ArchiveEntriesRequest) (*contracts.ArchiveEntriesResult, error) {
	return a.Archive.MoveEntries(req)
}

func (a *App) ArchiveDeleteEntries(req contracts.ArchiveEntriesRequest) (*contracts.ArchiveEntriesResult, error) {
	return a.Archive.DeleteEntries(req)
}