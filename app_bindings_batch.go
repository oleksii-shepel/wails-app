package main

func (a *App) BatchCopy(req BatchRequest) (*BatchResult, error) {
	return a.batch.BatchCopy(req)
}

func (a *App) BatchMove(req BatchRequest) (*BatchResult, error) {
	return a.batch.BatchMove(req)
}

func (a *App) BatchDelete(req BatchDeleteRequest) (*BatchResult, error) {
	return a.batch.BatchDelete(req)
}

func (a *App) BatchExtract(req BatchExtractRequest) (*BatchExtractResult, error) {
	return a.batch.BatchExtract(req)
}

func (a *App) BatchSelectiveExtract(req BatchSelectiveExtractRequest) (*BatchSelectiveExtractResult, error) {
	return a.batch.BatchSelectiveExtract(req)
}

func (a *App) ResolveConflict(conflictID string, resolution ConflictResolution, applyToAll bool) bool {
	return a.batch.ResolveConflict(conflictID, resolution, applyToAll)
}
