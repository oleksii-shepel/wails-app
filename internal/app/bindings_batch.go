package app

import (
	contracts "wails-app/internal/contracts"
	"wails-app/internal/pathref"
)

func (a *App) BatchCopy(req contracts.BatchRequest) (*contracts.BatchResult, error) {
	return a.Batch.BatchCopy(req)
}

func (a *App) BatchMove(req contracts.BatchRequest) (*contracts.BatchResult, error) {
	return a.Batch.BatchMove(req)
}

func (a *App) BatchDelete(req contracts.BatchDeleteRequest) (*contracts.BatchResult, error) {
	return a.Batch.BatchDelete(req)
}

func (a *App) BatchExtract(req contracts.BatchExtractRequest) (*contracts.BatchExtractResult, error) {
	req.DestinationDir = pathref.ToNativePath(req.DestinationDir)
	for i, arch := range req.Archives {
		req.Archives[i] = pathref.ToNativePath(arch)
	}
	return a.Batch.BatchExtract(req)
}

func (a *App) BatchSelectiveExtract(req contracts.BatchSelectiveExtractRequest) (*contracts.BatchSelectiveExtractResult, error) {
	req.DestinationDir = pathref.ToNativePath(req.DestinationDir)
	for i, item := range req.Items {
		req.Items[i].ArchivePath = pathref.ToNativePath(item.ArchivePath)
		if item.DestinationDir != "" {
			req.Items[i].DestinationDir = pathref.ToNativePath(item.DestinationDir)
		}
	}
	return a.Batch.BatchSelectiveExtract(req)
}

func (a *App) ResolveConflict(conflictID string, resolution contracts.ConflictResolution, applyToAll bool) bool {
	return a.Batch.ResolveConflict(conflictID, resolution, applyToAll)
}
