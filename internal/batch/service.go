package batch

import contracts "wails-app/internal/contracts"

// Service is the batch-domain boundary used by App while the heavier batch
// implementation is still being peeled out of the root package incrementally.
type Service struct {
	batchCopy             func(req contracts.BatchRequest) (*contracts.BatchResult, error)
	batchMove             func(req contracts.BatchRequest) (*contracts.BatchResult, error)
	batchDelete           func(req contracts.BatchDeleteRequest) (*contracts.BatchResult, error)
	batchExtract          func(req contracts.BatchExtractRequest) (*contracts.BatchExtractResult, error)
	batchSelectiveExtract func(req contracts.BatchSelectiveExtractRequest) (*contracts.BatchSelectiveExtractResult, error)
	resolveConflict       func(conflictID string, resolution contracts.ConflictResolution, applyToAll bool) bool
}

func NewService(
	batchCopy func(req contracts.BatchRequest) (*contracts.BatchResult, error),
	batchMove func(req contracts.BatchRequest) (*contracts.BatchResult, error),
	batchDelete func(req contracts.BatchDeleteRequest) (*contracts.BatchResult, error),
	batchExtract func(req contracts.BatchExtractRequest) (*contracts.BatchExtractResult, error),
	batchSelectiveExtract func(req contracts.BatchSelectiveExtractRequest) (*contracts.BatchSelectiveExtractResult, error),
	resolveConflict func(conflictID string, resolution contracts.ConflictResolution, applyToAll bool) bool,
) *Service {
	return &Service{
		batchCopy:             batchCopy,
		batchMove:             batchMove,
		batchDelete:           batchDelete,
		batchExtract:          batchExtract,
		batchSelectiveExtract: batchSelectiveExtract,
		resolveConflict:       resolveConflict,
	}
}

func (s *Service) BatchCopy(req contracts.BatchRequest) (*contracts.BatchResult, error) {
	return s.batchCopy(req)
}

func (s *Service) BatchMove(req contracts.BatchRequest) (*contracts.BatchResult, error) {
	return s.batchMove(req)
}

func (s *Service) BatchDelete(req contracts.BatchDeleteRequest) (*contracts.BatchResult, error) {
	return s.batchDelete(req)
}

func (s *Service) BatchExtract(req contracts.BatchExtractRequest) (*contracts.BatchExtractResult, error) {
	return s.batchExtract(req)
}

func (s *Service) BatchSelectiveExtract(req contracts.BatchSelectiveExtractRequest) (*contracts.BatchSelectiveExtractResult, error) {
	return s.batchSelectiveExtract(req)
}

func (s *Service) ResolveConflict(conflictID string, resolution contracts.ConflictResolution, applyToAll bool) bool {
	return s.resolveConflict(conflictID, resolution, applyToAll)
}