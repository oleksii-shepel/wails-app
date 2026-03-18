package main

import (
	"os"

	internalcontracts "wails-app/internal/contracts"
	internalelevation "wails-app/internal/elevation"
	internalops "wails-app/internal/ops"
	internalruntime "wails-app/internal/runtime"
)

func (a *App) startElevatedClient(exePath string) (internalelevation.Client, error) {
	return StartElevatedServiceClient(exePath)
}

func (a *App) createPaneWatch() {
	a.DirWatch = internalruntime.NewPaneWatchManager(a.emitEvent)
}

func (a *App) closePaneWatch() {
	if a.DirWatch != nil {
		a.DirWatch.Close()
	}
}

func (a *App) cancelDirStreams() {
	if a.DirStream != nil {
		a.DirStream.CancelAll()
	}
}

func (a *App) stopElevationSession() {
	if a.Elevation != nil {
		a.Elevation.StopSession()
	}
}

func (a *App) closeArchiveCache() {
	a.ArchCache.CloseAll()
}

func (a *App) buildFileInfo(name, fullPath string, info os.FileInfo, hidden bool) internalcontracts.FileInfo {
	return buildFileInfoWithHidden(name, fullPath, info, hidden)
}

func (a *App) startOperation(kind internalcontracts.OperationType, title, source, dest string) *internalops.ActiveOperation {
	return a.Operations.StartOperation(kind, title, source, dest)
}

func (a *App) promptConflict(event internalcontracts.ConflictEvent) {
	a.emitEvent("operation:conflict", event)
}

func (a *App) extractNestedArchive(archivePath string, nestedChain []string) ([]byte, error) {
	return a.Archive.ExtractNestedArchive(archivePath, nestedChain)
}
