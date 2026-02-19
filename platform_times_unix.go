//go:build !windows

package main

import (
	"os"
	"strings"
	"syscall"
)

func checkHidden(name, _ string) bool {
	// On Unix/macOS, hidden files start with '.'
	return strings.HasPrefix(name, ".")
}

func getCreatedTime(info os.FileInfo) int64 {
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		// Ctim is change time (closest to "created" on Linux)
		return stat.Ctim.Sec
	}
	return info.ModTime().Unix()
}

func getAccessedTime(info os.FileInfo) int64 {
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		return stat.Atim.Sec
	}
	return info.ModTime().Unix()
}
