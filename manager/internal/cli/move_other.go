//go:build !windows && !linux

package cli

import "os"

func moveNew(from, to string) error {
	if _, err := os.Lstat(to); !os.IsNotExist(err) {
		if err == nil {
			return os.ErrExist
		}
		return err
	}
	return os.Rename(from, to)
}
