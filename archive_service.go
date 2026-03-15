package main

// ArchiveService is the backend boundary for archive-specific operations.
// App stays as the Wails-facing facade and delegates archive work here.
type ArchiveService struct {
	app *App
}

func NewArchiveService(app *App) *ArchiveService {
	return &ArchiveService{app: app}
}

func (s *ArchiveService) Browse(fsPath string, nestedChain []string, innerPath string) (ArchiveListing, error) {
	return s.app.vfsBrowseArchive(fsPath, nestedChain, innerPath)
}

func (s *ArchiveService) ReadFile(fsPath string, nestedChain []string, entryPath string, encoding string) (*FileContent, error) {
	return s.app.readArchiveEntry(fsPath, nestedChain, entryPath, encoding)
}

func (s *ArchiveService) CopyEntries(req ArchiveEntriesRequest) (*ArchiveEntriesResult, error) {
	return s.app.archiveCopyEntries(req)
}

func (s *ArchiveService) MoveEntries(req ArchiveEntriesRequest) (*ArchiveEntriesResult, error) {
	return s.app.archiveMoveEntries(req)
}

func (s *ArchiveService) DeleteEntries(req ArchiveEntriesRequest) (*ArchiveEntriesResult, error) {
	return s.app.archiveDeleteEntries(req)
}

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
