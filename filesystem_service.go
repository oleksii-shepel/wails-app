package main

import (
	"fmt"

	internalelevation "wails-app/internal/elevation"
	internalfilesystem "wails-app/internal/filesystem"
	vfslib "wails-app/vfs"
)

type filesystemElevation interface {
	Client() internalelevation.Client
}

// FilesystemService owns filesystem orchestration, including permission
// fallback through the elevated session service.
type FilesystemService struct {
	elevation            filesystemElevation
	emitEvent            func(string, interface{})
	startElevatedSession func() (*ElevatedSessionStatus, error)
	vfs                  *vfslib.Unified
	duStreams            *duStreamRegistry
	duCache              *internalfilesystem.DiskUsageCache
}

func NewFilesystemService(
	elevation filesystemElevation,
	emitEvent func(string, interface{}),
	startElevatedSession func() (*ElevatedSessionStatus, error),
) *FilesystemService {
	return &FilesystemService{
		elevation:            elevation,
		emitEvent:            emitEvent,
		startElevatedSession: startElevatedSession,
		duStreams:            newDuStreamRegistry(),
		duCache:              internalfilesystem.NewDiskUsageCache(internalfilesystem.DefaultDiskUsageCacheCapacity),
	}
}

// SetVFS wires the unified VFS so ListDirectory can route mounted paths through it.
func (s *FilesystemService) SetVFS(v *vfslib.Unified) {
	s.vfs = v
}

func (s *FilesystemService) emit(name string, data interface{}) {
	if s.emitEvent != nil {
		s.emitEvent(name, data)
	}
}

func (s *FilesystemService) elevationTargetSingle(path string) string {
	return path
}

func (s *FilesystemService) elevationTargetPair(source, destination string) string {
	return fmt.Sprintf("%s -> %s", source, destination)
}
