//go:build !windows

package cli

import (
	"wails-app/internal/contracts"
	"wails-app/internal/platform"
)

func listTrash() ([]contracts.TrashEntry, error) {
	return platform.ListTrashEntries()
}

func restoreTrash(entry contracts.TrashEntry) (string, error) {
	return platform.RestoreTrashEntry(entry.ID)
}

func emptyTrash() error {
	return platform.EmptyTrash()
}
