package app

import contracts "wails-app/internal/contracts"

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
	return a.Batch.BatchExtract(req)
}

func (a *App) BatchSelectiveExtract(req contracts.BatchSelectiveExtractRequest) (*contracts.BatchSelectiveExtractResult, error) {
	return a.Batch.BatchSelectiveExtract(req)
}

func (a *App) ResolveConflict(conflictID string, resolution contracts.ConflictResolution, applyToAll bool) bool {
	return a.Batch.ResolveConflict(conflictID, resolution, applyToAll)
}
