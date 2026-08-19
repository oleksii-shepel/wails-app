package app

import (
	contracts "wails-app/internal/contracts"
	internalops "wails-app/internal/ops"
)

func (s *State) StartOperation(kind contracts.OperationType, title, source, dest string) *internalops.ActiveOperation {
	return s.Operations.StartOperation(kind, title, source, dest)
}

func (s *State) PromptConflict(event contracts.ConflictEvent) {
	s.EmitEvent("operation:conflict", event)
}

func (s *State) ExtractNestedArchive(archivePath string, nestedChain []string) ([]byte, error) {
	return s.Archive.ExtractNestedArchive(archivePath, nestedChain)
}