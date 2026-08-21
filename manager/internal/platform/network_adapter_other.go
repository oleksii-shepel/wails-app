//go:build !windows

package platform

import contracts "wails-app/internal/contracts"

func NetworkAdapterMetadata() (map[uint32]map[string]any, error) {
	return map[uint32]map[string]any{}, nil
}

func SetNetworkAdapterEnabled(_ uint32, _ bool) error { return nil }
func RenewNetworkAdapterDHCP(_ uint32) error          { return nil }

func DiagnoseNetworkAdapter(index uint32) (contracts.SystemSurfaceRecord, error) {
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

func OpenNetworkSettings() error { return nil }
