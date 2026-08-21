//go:build !windows

package app

import contracts "wails-app/internal/contracts"

func (a *App) NetworkListComputers() ([]contracts.SystemSurfaceRecord, error) {
	return []contracts.SystemSurfaceRecord{}, nil
}

func (a *App) NetworkListShares(_ string) ([]contracts.SystemSurfaceRecord, error) {
	return []contracts.SystemSurfaceRecord{}, nil
}

func (a *App) NetworkAdapterSetEnabled(_ uint32, _ bool) error { return nil }
func (a *App) NetworkAdapterRenewDHCP(_ uint32) error          { return nil }

func (a *App) NetworkAdapterDiagnose(index uint32) (contracts.SystemSurfaceRecord, error) {
	return contracts.SystemSurfaceRecord{
		ID:   "adapter-diagnostic",
		Name: "Network adapter",
		Fields: map[string]any{
			"healthy": false,
			"summary": "Adapter diagnostics are available on Windows.",
			"index":   index,
		},
	}, nil
}

func (a *App) NetworkOpenSettings() error { return nil }
