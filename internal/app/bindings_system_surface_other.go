//go:build !windows

package app

import contracts "wails-app/internal/contracts"

func (a *App) SystemSurfaceList(kind string) ([]contracts.SystemSurfaceRecord, error) {
	return []contracts.SystemSurfaceRecord{}, nil
}
