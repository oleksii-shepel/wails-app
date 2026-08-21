//go:build windows

package app

import (
	contracts "wails-app/internal/contracts"
	"wails-app/internal/platform"
)

func (a *App) NetworkListComputers() ([]contracts.SystemSurfaceRecord, error) {
	return platform.ListNetworkComputers()
}

func (a *App) NetworkListShares(computer string) ([]contracts.SystemSurfaceRecord, error) {
	return platform.ListNetworkShares(computer)
}
