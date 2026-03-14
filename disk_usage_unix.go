//go:build !windows

package main

import "os"

// isLink returns true for symbolic links on non-Windows platforms.
func isLink(e os.DirEntry) bool {
	return e.Type()&os.ModeSymlink != 0
}
