//go:build windows

package app

import (
	"strconv"

	contracts "wails-app/internal/contracts"
	internalelevation "wails-app/internal/elevation"
	"wails-app/internal/platform"
)

func (a *App) NetworkListComputers() ([]contracts.SystemSurfaceRecord, error) {
	return platform.ListNetworkComputers()
}

func (a *App) NetworkListShares(computer string) ([]contracts.SystemSurfaceRecord, error) {
	return platform.ListNetworkShares(computer)
}

func (a *App) NetworkAdapterSetEnabled(index uint32, enabled bool) error {
	err := platform.SetNetworkAdapterEnabled(index, enabled)
	if err == nil {
		return nil
	}
	if !internalelevation.IsPermissionDenied(err) {
		return err
	}

	op := "network-adapter-disable"
	if enabled {
		op = "network-adapter-enable"
	}

	_, elevatedErr := runElevatedRequest(internalelevation.Request{
		Op:   op,
		Path: strconv.FormatUint(uint64(index), 10),
	})
	return elevatedErr
}

func (a *App) NetworkAdapterRenewDHCP(index uint32) error {
	err := platform.RenewNetworkAdapterDHCP(index)
	if err == nil {
		return nil
	}
	if !internalelevation.IsPermissionDenied(err) {
		return err
	}

	_, elevatedErr := runElevatedRequest(internalelevation.Request{
		Op:   "network-adapter-renew-dhcp",
		Path: strconv.FormatUint(uint64(index), 10),
	})
	return elevatedErr
}

func (a *App) NetworkAdapterDiagnose(index uint32) (contracts.SystemSurfaceRecord, error) {
	return platform.DiagnoseNetworkAdapter(index)
}

func (a *App) NetworkOpenSettings() error {
	return platform.OpenNetworkSettings()
}
