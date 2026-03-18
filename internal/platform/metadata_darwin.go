//go:build darwin

package platform

import (
	"os"
	"syscall"
)

func GetCreatedTime(info os.FileInfo) int64 {
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		return stat.Ctimespec.Sec
	}
	return info.ModTime().Unix()
}

func GetAccessedTime(info os.FileInfo) int64 {
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		return stat.Atimespec.Sec
	}
	return info.ModTime().Unix()
}
