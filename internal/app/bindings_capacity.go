package app

import (
	"fmt"

	contracts "wails-app/internal/contracts"
	"wails-app/internal/platform"
)

func (a *App) GetPathCapacity(path string) (contracts.PathCapacity, error) {
	resolved, err := a.State.Filesystem.RealPath(path)
	if err != nil {
		return contracts.PathCapacity{}, fmt.Errorf("capacity: resolve path: %w", err)
	}

	capacity, err := platform.GetPathCapacity(resolved)
	if err != nil {
		return contracts.PathCapacity{}, fmt.Errorf("capacity: %w", err)
	}
	return capacity, nil
}
