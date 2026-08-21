//go:build !windows

package app

import contracts "wails-app/internal/contracts"

func (a *App) NetworkListComputers() ([]contracts.SystemSurfaceRecord, error) {
	return []contracts.SystemSurfaceRecord{}, nil
}

func (a *App) NetworkListShares(_ string) ([]contracts.SystemSurfaceRecord, error) {
	return []contracts.SystemSurfaceRecord{}, nil
}
