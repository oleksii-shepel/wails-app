//go:build !windows

package platform

import contracts "wails-app/internal/contracts"

// ListNetworkSurface currently exposes native Windows network tables.
// Non-Windows hosts keep the contract available without invoking a shell.
func ListNetworkSurface() ([]contracts.SystemSurfaceRecord, error) {
	return []contracts.SystemSurfaceRecord{}, nil
}
