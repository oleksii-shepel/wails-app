//go:build !windows

package platform

import (
	"os"
	"os/user"
	"strconv"
	"syscall"
)

func GetFileOwner(_ string, info os.FileInfo) string {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return ""
	}
	id := strconv.FormatUint(uint64(stat.Uid), 10)
	owner, err := user.LookupId(id)
	if err != nil {
		return id
	}
	if owner.Username != "" {
		return owner.Username
	}
	return id
}

func FormatFileAttributes(_ string, _ os.FileInfo) string {
	return ""
}
