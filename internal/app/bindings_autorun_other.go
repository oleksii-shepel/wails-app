//go:build !windows

package app

import (
	"fmt"
	contracts "wails-app/internal/contracts"
)

func (a *App) AutorunList() ([]contracts.AutorunEntry, error) { return []contracts.AutorunEntry{}, nil }
func (a *App) AutorunSetEnabled(_ string, _ bool) error {
	return fmt.Errorf("autorun management is available only on Windows")
}
func (a *App) AutorunDelete(_ string) error {
	return fmt.Errorf("autorun management is available only on Windows")
}
