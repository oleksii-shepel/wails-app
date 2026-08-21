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

func (a *App) NetworkAdapterSetEnabled(index uint32, enabled bool) error {
	return platform.SetNetworkAdapterEnabled(index, enabled)
}

func (a *App) NetworkAdapterRenewDHCP(index uint32) error {
	return platform.RenewNetworkAdapterDHCP(index)
}

func (a *App) NetworkAdapterDiagnose(index uint32) (contracts.SystemSurfaceRecord, error) {
	return platform.DiagnoseNetworkAdapter(index)
}

func (a *App) NetworkOpenSettings() error {
	return platform.OpenNetworkSettings()
}
