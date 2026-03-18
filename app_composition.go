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
	a.dirWatch = internalruntime.NewPaneWatchManager(a.emitEvent)
}

func (a *App) closePaneWatch() {
	if a.dirWatch != nil {
		a.dirWatch.Close()
	}
}

func (a *App) cancelDirStreams() {
	if a.dirStream != nil {
		a.dirStream.CancelAll()
	}
}

func (a *App) stopElevationSession() {
	if a.elevation != nil {
		a.elevation.StopSession()
	}
}

func (a *App) closeArchiveCache() {
	a.archCache.CloseAll()
}

func (a *App) buildFileInfo(name, fullPath string, info os.FileInfo, hidden bool) internalcontracts.FileInfo {
	return buildFileInfoWithHidden(name, fullPath, info, hidden)
}

func (a *App) startOperation(kind internalcontracts.OperationType, title, source, dest string) *internalops.ActiveOperation {
	return a.operations.StartOperation(kind, title, source, dest)
}

func (a *App) promptConflict(event internalcontracts.ConflictEvent) {
	a.emitEvent("operation:conflict", event)
}

func (a *App) extractNestedArchive(archivePath string, nestedChain []string) ([]byte, error) {
	return a.archive.ExtractNestedArchive(archivePath, nestedChain)
}
