//go:build !windows

package osfs

import (
	"strings"
)

func isHiddenName(name string) bool {
	return strings.HasPrefix(name, ".")
}