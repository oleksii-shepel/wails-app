//go:build darwin

package main

import (
	"os"
	"syscall"
)

func getCreatedTime(info os.FileInfo) int64 {
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		// Ctimespec is the ctime (closest to "created" on macOS)
		return stat.Ctimespec.Sec
	}
	return info.ModTime().Unix()
}

func getAccessedTime(info os.FileInfo) int64 {
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		return stat.Atimespec.Sec
	}
	return info.ModTime().Unix()
}
