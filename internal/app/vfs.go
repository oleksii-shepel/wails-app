package app

import (
	contracts "wails-app/internal/contracts"
	internalruntime "wails-app/internal/runtime"
)

func (s *State) InitVFS() {
	vfs, err := internalruntime.InitVFS()
	if err != nil {
		return
	}
	s.VFS = vfs
	if s.Filesystem != nil {
		s.Filesystem.SetVFS(s.VFS)
	}
}

func (s *State) VFSListDir(path string) (*contracts.DirectoryListing, error) {
	return internalruntime.VFSListDir(s.VFS, path)
}

func (s *State) VFSOpenArchive(path string, nested []string) (contracts.ArchiveListing, error) {
	return internalruntime.VFSOpenArchive(s.VFS, path, nested)
}

func (s *State) VFSMountArchive(path string) (string, error) {
	return internalruntime.VFSMountArchive(s.VFS, path)
}

func (s *State) VFSUnmount(mountPoint string) error {
	return internalruntime.VFSUnmount(s.VFS, mountPoint)
}
