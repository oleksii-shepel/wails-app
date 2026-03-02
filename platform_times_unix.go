//go:build !windows

package main

import (
	"strings"
)

func checkHidden(name, _ string) bool {
	// On Unix/macOS, hidden files start with '.'
	return strings.HasPrefix(name, ".")
}
