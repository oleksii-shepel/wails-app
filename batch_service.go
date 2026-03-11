package main

// BatchService owns top-level batch orchestration while App remains the
// Wails-facing boundary.
type BatchService struct {
	operations            *OperationManager
	emitEvent             func(string, interface{})
	runElevatedWithEvents func(op, target string, req elevatedRequest) (elevatedResponse, error)
	extractNestedArchive  func(archivePath string, nestedChain []string) ([]byte, error)
}

func NewBatchService(
	operations *OperationManager,
	emitEvent func(string, interface{}),
	runElevatedWithEvents func(op, target string, req elevatedRequest) (elevatedResponse, error),
	extractNestedArchive func(archivePath string, nestedChain []string) ([]byte, error),
) *BatchService {
	return &BatchService{
		operations:            operations,
		emitEvent:             emitEvent,
		runElevatedWithEvents: runElevatedWithEvents,
		extractNestedArchive:  extractNestedArchive,
	}
}

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
