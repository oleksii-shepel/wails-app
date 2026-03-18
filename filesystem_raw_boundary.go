package main

import (
	"os"
	"runtime"

	internalfilesystem "wails-app/internal/filesystem"
	internalplatform "wails-app/internal/platform"
)

var rawFilesystemOps = internalfilesystem.NewRawService(internalfilesystem.RawCallbacks{
	Metadata: rawFilesystemMetadataCallbacks(),
	MoveToTrash: func(path string) error {
		return internalplatform.MoveToTrash(path)
	},
})

func rawFilesystemMetadataCallbacks() internalfilesystem.MetadataCallbacks {
	return internalfilesystem.MetadataCallbacks{
		ToSlash:                      toSlash,
		IsHiddenFile:                 isHiddenFile,
		IsArchive:                    isArchive,
		SymlinkPointsToDir:           internalplatform.SymlinkPointsToDir,
		IsUntrackedDirectoryJunction: internalplatform.IsUntrackedDirectoryJunction,
		GetCreatedTime:               internalplatform.GetCreatedTime,
		GetAccessedTime:              internalplatform.GetAccessedTime,
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
