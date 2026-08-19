//go:build !windows

package filesystem

import "os"

func isLink(entry os.DirEntry) bool {
	return entry.Type()&os.ModeSymlink != 0
}