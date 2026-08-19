//go:build !windows

package platform

import "strings"

func CheckHidden(name, _ string) bool {
	return strings.HasPrefix(name, ".")
}