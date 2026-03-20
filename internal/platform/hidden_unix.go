//go:build !windows

package platform

func EnsureHiddenPath(path string) error {
	return nil
}
