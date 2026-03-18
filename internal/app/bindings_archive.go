package app

import contracts "wails-app/internal/contracts"

func (a *App) ArchiveBrowse(fsPath string, nestedChain []string, innerPath string) (contracts.ArchiveListing, error) {
	return a.Archive.Browse(fsPath, nestedChain, innerPath)
}

func (a *App) ArchiveRead(fsPath string, nestedChain []string, entryPath string, encoding string) (*contracts.FileContent, error) {
	return a.Archive.ReadFile(fsPath, nestedChain, entryPath, encoding)
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
