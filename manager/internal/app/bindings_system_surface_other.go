//go:build !windows

package app

import (
	"strings"

	contracts "wails-app/internal/contracts"
	"wails-app/internal/platform"
)

func (a *App) SystemSurfaceList(kind string) ([]contracts.SystemSurfaceRecord, error) {
	if strings.EqualFold(strings.TrimSpace(kind), "environment") {
		return platform.ListEnvironmentSurface()
	}
	return []contracts.SystemSurfaceRecord{}, nil
}
