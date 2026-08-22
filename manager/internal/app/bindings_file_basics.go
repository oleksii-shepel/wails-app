package app

import (
	"fmt"

	contracts "wails-app/internal/contracts"
	"wails-app/internal/platform"
)

func (a *App) ListTrashEntries() ([]contracts.TrashEntry, error) {
	return platform.ListTrashEntries()
}

func (a *App) RestoreTrashEntry(id string) (string, error) {
	return platform.RestoreTrashEntry(id)
}

func (a *App) BatchTransferTrash(req contracts.TrashTransferRequest) (*contracts.TrashTransferResult, error) {
	if len(req.IDs) == 0 {
		return &contracts.TrashTransferResult{}, nil
	}
	if req.DestinationDir == "" {
		return nil, fmt.Errorf("trash transfer: destinationDir is required")
	}

	var opType contracts.OperationType
	var title string
	switch req.Operation {
	case contracts.TrashTransferCopy:
		opType = contracts.OpCopy
		title = "Copy from Recycle Bin"
	case contracts.TrashTransferRestore:
		opType = contracts.OpMove
		title = "Restore from Recycle Bin"
	default:
		return nil, fmt.Errorf("trash transfer: unsupported operation %q", req.Operation)
	}

	op := a.StartOperation(opType, title, "bin://", req.DestinationDir)
	op.SetTotal(int64(len(req.IDs)), 0)

	resolved, err := a.State.Filesystem.RealPath(req.DestinationDir)
	if err != nil {
		err = fmt.Errorf("trash transfer: resolve destination: %w", err)
		op.SetError(err)
		return nil, err
	}

	result := &contracts.TrashTransferResult{
		OperationID: op.ID,
		Total:       len(req.IDs),
	}

	for index, id := range req.IDs {
		if op.IsCancelled() {
			break
		}

		var destination string
		if req.Operation == contracts.TrashTransferCopy {
			destination, err = platform.CopyTrashEntryTo(id, resolved)
		} else {
			destination, err = platform.RestoreTrashEntryTo(id, resolved)
		}

		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, contracts.TrashTransferItemError{
				ID:    id,
				Error: err.Error(),
			})
		} else {
			result.Succeeded++
			result.Destinations = append(result.Destinations, destination)
		}

		op.UpdateProgress(int64(index+1), 0, id, 0)
	}

	if !op.IsCancelled() {
		op.Complete()
	}
	return result, nil
}

func (a *App) CopyTrashEntryTo(id, destinationDir string) (string, error) {
	resolved, err := a.State.Filesystem.RealPath(destinationDir)
	if err != nil {
		return "", fmt.Errorf("trash copy: resolve destination: %w", err)
	}
	return platform.CopyTrashEntryTo(id, resolved)
}

func (a *App) RestoreTrashEntryTo(id, destinationDir string) (string, error) {
	resolved, err := a.State.Filesystem.RealPath(destinationDir)
	if err != nil {
		return "", fmt.Errorf("trash restore: resolve destination: %w", err)
	}
	return platform.RestoreTrashEntryTo(id, resolved)
}

func (a *App) DeleteTrashEntry(id string) error {
	return platform.DeleteTrashEntry(id)
}

func (a *App) EmptyTrash() error {
	return platform.EmptyTrash()
}

func (a *App) GetPathAttributes(path string) (contracts.PathAttributes, error) {
	resolved, err := a.State.Filesystem.RealPath(path)
	if err != nil {
		return contracts.PathAttributes{}, fmt.Errorf("attributes: resolve path: %w", err)
	}
	return platform.GetPathAttributes(resolved)
}

func (a *App) SetPathAttributes(path string, update contracts.PathAttributesUpdate) error {
	resolved, err := a.State.Filesystem.RealPath(path)
	if err != nil {
		return fmt.Errorf("attributes: resolve path: %w", err)
	}
	return platform.SetPathAttributes(resolved, update)
}
