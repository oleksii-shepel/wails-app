//go:build !windows

package app

func fitStartupWindowBounds(x, y, width, height int, hasPosition bool) (int, int, int, int) {
	return x, y, width, height
}
