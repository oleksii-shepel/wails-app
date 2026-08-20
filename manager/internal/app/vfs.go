package app

import (
	"context"

	contracts "wails-app/internal/contracts"
	"wails-app/internal/pathref"
	internalruntime "wails-app/internal/runtime"
)

func (s *State) InitVFS() {
	vfs, err := internalruntime.InitVFS()
	if err != nil {
		return
	}
	s.vfs = vfs
	if s.Filesystem != nil {
		s.Filesystem.SetVFS(s.vfs)
	}
	if s.Utility != nil {
		s.Utility.SetVFS(s.vfs)
	}
	if s.DirStream != nil {
		s.DirStream.SetVFS(
			func(path string) (*contracts.DirectoryListing, error) {
				return internalruntime.VFSListDir(context.Background(), s.vfs, path)
			},
			func(path string) bool {
				normalized := pathref.Normalize(path)
				if pathref.IsAlias(normalized) {
					return s.vfs.IsMounted(pathref.ToVFS(normalized))
				}
				return s.vfs.IsMounted(normalized)
			},
		)
		s.DirStream.SetVFSPaged(func(ctx context.Context, path, token string, limit int) (*contracts.DirectoryPage, error) {
			return internalruntime.VFSListDirPage(ctx, s.vfs, path, token, limit)
		})
	}
}

func (s *State) VFSListDir(path string) (*contracts.DirectoryListing, error) {
	if s.Filesystem != nil && pathref.IsRegistry(path) {
		return s.Filesystem.ListDirectory(path, false)
	}
	return internalruntime.VFSListDir(context.Background(), s.vfs, path)
}

func (s *State) VFSReadFile(path string, encoding string) (*contracts.FileContent, error) {
	if s.Filesystem != nil && pathref.IsRegistry(path) {
		return s.Filesystem.ReadFile(path, encoding)
	}
	return internalruntime.VFSReadFile(context.Background(), s.vfs, path, encoding)
}

func (s *State) VFSWriteFile(path string, content string, encoding string) (*contracts.OperationResult, error) {
	if s.Filesystem != nil && pathref.IsRegistry(path) {
		return s.Filesystem.WriteFile(path, content, encoding)
	}
	return internalruntime.VFSWriteFile(context.Background(), s.vfs, path, content, encoding)
}

func (s *State) VFSMkdir(path string) (*contracts.OperationResult, error) {
	if s.Filesystem != nil && pathref.IsRegistry(path) {
		return s.Filesystem.CreateDirectory(path, false)
	}
	return internalruntime.VFSMkdir(context.Background(), s.vfs, path)
}

func (s *State) VFSRemove(path string) (*contracts.OperationResult, error) {
	if s.Filesystem != nil && pathref.IsRegistry(path) {
		return s.Filesystem.DeleteFile(path, false, nil)
	}
	return internalruntime.VFSRemove(context.Background(), s.vfs, path)
}

func (s *State) VFSOpenArchive(path string, nested []string) (contracts.ArchiveListing, error) {
	return internalruntime.VFSOpenArchive(context.Background(), s.vfs, path, nested)
}

func (s *State) VFSMountArchive(path string) (string, error) {
	return internalruntime.VFSMountArchive(context.Background(), s.vfs, path)
}

func (s *State) VFSUnmount(mountPoint string) error {
	return internalruntime.VFSUnmount(context.Background(), s.vfs, mountPoint)
}
