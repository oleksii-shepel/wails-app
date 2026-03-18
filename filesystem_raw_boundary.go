package main

import (
	"os"
	"runtime"

	internalfilesystem "wails-app/internal/filesystem"
)

var rawFilesystemOps = internalfilesystem.NewRawService(internalfilesystem.RawCallbacks{
	Metadata: rawFilesystemMetadataCallbacks(),
	MoveToTrash: func(path string) error {
		return moveToTrash(path)
	},
})

func rawFilesystemMetadataCallbacks() internalfilesystem.MetadataCallbacks {
	return internalfilesystem.MetadataCallbacks{
		ToSlash:                      toSlash,
		IsHiddenFile:                 isHiddenFile,
		IsArchive:                    isArchive,
		SymlinkPointsToDir:           symlinkPointsToDir,
		IsUntrackedDirectoryJunction: isUntrackedDirectoryJunction,
		GetCreatedTime:               getCreatedTime,
		GetAccessedTime:              getAccessedTime,
		FormatPermissions:            formatPermissions,
	}
}

func formatPermissions(info os.FileInfo) string {
	if runtime.GOOS == "windows" {
		if info.Mode()&0200 != 0 {
			return "rw-"
		}
		return "r--"
	}
	return info.Mode().Perm().String()
}
