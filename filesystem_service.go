package main

import vfslib "wails-app/vfs"

// FilesystemService owns filesystem orchestration, including permission
// fallback through the elevated session service.
type FilesystemService struct {
	elevation            *ElevationSessionService
	emitEvent            func(string, interface{})
	startElevatedSession func() (*ElevatedSessionStatus, error)
	vfs                  *vfslib.Unified
}

func NewFilesystemService(
	elevation *ElevationSessionService,
	emitEvent func(string, interface{}),
	startElevatedSession func() (*ElevatedSessionStatus, error),
) *FilesystemService {
	return &FilesystemService{
		elevation:            elevation,
		emitEvent:            emitEvent,
		startElevatedSession: startElevatedSession,
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
