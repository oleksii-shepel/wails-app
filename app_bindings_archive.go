package main

func (a *App) ArchiveBrowse(fsPath string, nestedChain []string, innerPath string) (ArchiveListing, error) {
	return a.archive.Browse(fsPath, nestedChain, innerPath)
}

func (a *App) ArchiveRead(fsPath string, nestedChain []string, entryPath string, encoding string) (*FileContent, error) {
	return a.archive.ReadFile(fsPath, nestedChain, entryPath, encoding)
}

func (a *App) ArchiveCopyEntries(req ArchiveEntriesRequest) (*ArchiveEntriesResult, error) {
	return a.archive.CopyEntries(req)
}

func (a *App) ArchiveMoveEntries(req ArchiveEntriesRequest) (*ArchiveEntriesResult, error) {
	return a.archive.MoveEntries(req)
}

func (a *App) ArchiveDeleteEntries(req ArchiveEntriesRequest) (*ArchiveEntriesResult, error) {
	return a.archive.DeleteEntries(req)
}