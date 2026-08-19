//go:build !windows && !linux && !darwin

package platform

import (
	"fmt"
	"os"
	"runtime"
)

func GetCreatedTime(info os.FileInfo) int64 {
	return info.ModTime().Unix()
}

func GetAccessedTime(info os.FileInfo) int64 {
	return info.ModTime().Unix()
}

func JoinMountNamespace(nsPath string) error {
	return fmt.Errorf("join mount namespace not supported on %s: %s", runtime.GOOS, nsPath)
}