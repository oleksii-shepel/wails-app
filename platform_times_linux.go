//go:build linux

package main

import (
	"os"
	"syscall"
)

func getCreatedTime(info os.FileInfo) int64 {
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		// Ctim is the ctime (metadata-change time — closest to "created" on Linux)
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
