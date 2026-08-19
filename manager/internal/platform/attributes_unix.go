//go:build !windows

package platform

import (
	"fmt"
	"os"

	contracts "wails-app/internal/contracts"
)

func GetPathAttributes(path string) (contracts.PathAttributes, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return contracts.PathAttributes{}, err
	}
	mode := uint32(info.Mode().Perm())
	return contracts.PathAttributes{
		Permissions: info.Mode().Perm().String(),
		UnixMode:    mode,
		ReadOnly:    mode&0222 == 0,
		Hidden:      len(info.Name()) > 0 && info.Name()[0] == '.',
	}, nil
}

func SetPathAttributes(path string, update contracts.PathAttributesUpdate) error {
	if update.UnixMode == nil {
		return fmt.Errorf("attributes: unix mode is required")
	}
	return os.Chmod(path, os.FileMode(*update.UnixMode&07777))
}