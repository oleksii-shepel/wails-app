package main

import (
	"os"
	"runtime"

	internalfilesystem "wails-app/internal/filesystem"
	internalplatform "wails-app/internal/platform"
	vfsarchive "wails-app/vfs/archive"
)

var rawFilesystemOps = internalfilesystem.NewRawService(internalfilesystem.RawCallbacks{
	Metadata: rawFilesystemMetadataCallbacks(),
	MoveToTrash: func(path string) error {
		return internalplatform.MoveToTrash(path)
	},
})

func rawFilesystemMetadataCallbacks() internalfilesystem.MetadataCallbacks {
	return internalfilesystem.MetadataCallbacks{
		ToSlash:                      internalplatform.ToSlash,
		IsHiddenFile:                 isHiddenFile,
		IsArchive:                    vfsarchive.IsArchiveExt,
		SymlinkPointsToDir:           internalplatform.SymlinkPointsToDir,
		IsUntrackedDirectoryJunction: internalplatform.IsUntrackedDirectoryJunction,
		GetCreatedTime:               internalplatform.GetCreatedTime,
		GetAccessedTime:              internalplatform.GetAccessedTime,
		FormatPermissions:            formatPermissions,
	}
}

func buildFileInfoWithHidden(name, fullPath string, info os.FileInfo, hidden bool) FileInfo {
	return internalfilesystem.BuildFileInfoWithHidden(rawFilesystemMetadataCallbacks(), name, fullPath, info, hidden)
}

func isHiddenFile(name, path string) bool {
	return internalplatform.CheckHidden(name, path)
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
