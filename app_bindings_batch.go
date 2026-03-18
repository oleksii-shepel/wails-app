package main

func (a *App) BatchCopy(req BatchRequest) (*BatchResult, error) {
	return a.Batch.BatchCopy(req)
}

func (a *App) BatchMove(req BatchRequest) (*BatchResult, error) {
	return a.Batch.BatchMove(req)
}

func (a *App) BatchDelete(req BatchDeleteRequest) (*BatchResult, error) {
	return a.Batch.BatchDelete(req)
}

func (a *App) BatchExtract(req BatchExtractRequest) (*BatchExtractResult, error) {
	return a.Batch.BatchExtract(req)
}

func (a *App) BatchSelectiveExtract(req BatchSelectiveExtractRequest) (*BatchSelectiveExtractResult, error) {
	return a.Batch.BatchSelectiveExtract(req)
}

func (a *App) ResolveConflict(conflictID string, resolution ConflictResolution, applyToAll bool) bool {
	return a.Batch.ResolveConflict(conflictID, resolution, applyToAll)
}
