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
