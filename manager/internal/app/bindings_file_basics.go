package app

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"

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

	trashEntries, listErr := platform.ListTrashEntries()
	if listErr != nil {
		op.SetError(listErr)
		return nil, listErr
	}
	entryByID := make(map[string]contracts.TrashEntry, len(trashEntries))
	for _, entry := range trashEntries {
		entryByID[entry.ID] = entry
	}

	result := &contracts.TrashTransferResult{
		OperationID: op.ID,
		Total:       len(req.IDs),
	}
	var blanket *contracts.ConflictResolution

	for index, id := range req.IDs {
		if op.IsCancelled() || (blanket != nil && *blanket == contracts.ConflictCancel) {
			break
		}

		entry, found := entryByID[id]
		if !found {
			result.Failed++
			result.Errors = append(result.Errors, contracts.TrashTransferItemError{
				ID: id, Error: "Recycle Bin item not found.",
			})
			op.UpdateProgress(int64(index+1), 0, id, 0)
			continue
		}

		requestedDestination := filepath.Join(resolved, entry.Name)
		sourceLabel := "bin://entry/" + url.PathEscape(id)
		finalDestination, skip, conflictErr := a.State.Batch.CheckConflict(
			op,
			sourceLabel,
			requestedDestination,
			&blanket,
		)
		if conflictErr != nil {
			// Cancel is a user decision, not a failed filesystem transfer.
			if strings.Contains(strings.ToLower(conflictErr.Error()), "cancel") {
				break
			}
			result.Failed++
			result.Errors = append(result.Errors, contracts.TrashTransferItemError{
				ID: id, Error: conflictErr.Error(),
			})
			op.UpdateProgress(int64(index+1), 0, id, 0)
			continue
		}
		if skip {
			op.UpdateProgress(int64(index+1), 0, id, 0)
			continue
		}

		// Overwrite resolves to the requested path. Remove the old destination
		// only after the user explicitly chose Replace.
		if sameNativePath(finalDestination, requestedDestination) {
			if _, statErr := os.Lstat(finalDestination); statErr == nil {
				if removeErr := os.RemoveAll(finalDestination); removeErr != nil {
					result.Failed++
					result.Errors = append(result.Errors, contracts.TrashTransferItemError{
						ID: id, Error: fmt.Sprintf("replace destination: %v", removeErr),
					})
					op.UpdateProgress(int64(index+1), 0, id, 0)
					continue
				}
			}
		}

		var destination string
		if req.Operation == contracts.TrashTransferCopy {
			destination, err = platform.CopyTrashEntryToPath(id, finalDestination)
		} else {
			destination, err = platform.RestoreTrashEntryToPath(id, finalDestination)
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

func sameNativePath(a, b string) bool {
	cleanA := filepath.Clean(a)
	cleanB := filepath.Clean(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(cleanA, cleanB)
	}
	return cleanA == cleanB
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
