package app

import (
	contracts "wails-app/internal/contracts"
	"wails-app/internal/pathref"
	internalruntime "wails-app/internal/runtime"
)

func (s *State) InitVFS() {
	memoryPlugin, err := getMemoryFS()
	if err != nil {
		return
	}
	vfs, err := internalruntime.InitVFS(memoryPlugin)
	if err != nil {
		return
	}
	s.vfs = vfs
	if s.Filesystem != nil {
		s.Filesystem.SetVFS(s.vfs)
	}
}

func (s *State) VFSListDir(path string) (*contracts.DirectoryListing, error) {
	if s.Filesystem != nil && pathref.IsRegistry(path) {
		return s.Filesystem.ListDirectory(path, false)
	}
	return internalruntime.VFSListDir(s.vfs, path)
}

func (s *State) VFSReadFile(path string, encoding string) (*contracts.FileContent, error) {
	if s.Filesystem != nil && pathref.IsRegistry(path) {
		return s.Filesystem.ReadFile(path, encoding)
	}
	return internalruntime.VFSReadFile(s.vfs, path, encoding)
}

func (s *State) VFSWriteFile(path string, content string, encoding string) (*contracts.OperationResult, error) {
	if s.Filesystem != nil && pathref.IsRegistry(path) {
		return s.Filesystem.WriteFile(path, content, encoding)
	}
	return internalruntime.VFSWriteFile(s.vfs, path, content, encoding)
}

func (s *State) VFSMkdir(path string) (*contracts.OperationResult, error) {
	if s.Filesystem != nil && pathref.IsRegistry(path) {
		return s.Filesystem.CreateDirectory(path, false)
	}
	return internalruntime.VFSMkdir(s.vfs, path)
}

func (s *State) VFSRemove(path string) (*contracts.OperationResult, error) {
	if s.Filesystem != nil && pathref.IsRegistry(path) {
		return s.Filesystem.DeleteFile(path, false, nil)
	}
	return internalruntime.VFSRemove(s.vfs, path)
}

func (s *State) VFSOpenArchive(path string, nested []string) (contracts.ArchiveListing, error) {
	return internalruntime.VFSOpenArchive(s.vfs, path, nested)
}

func (s *State) VFSMountArchive(path string) (string, error) {
	return internalruntime.VFSMountArchive(s.vfs, path)
}

func (s *State) VFSUnmount(mountPoint string) error {
	return internalruntime.VFSUnmount(s.vfs, mountPoint)
}
