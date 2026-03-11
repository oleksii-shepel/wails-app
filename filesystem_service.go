package main

// FilesystemService owns filesystem orchestration, including permission
// fallback through the elevated session service.
type FilesystemService struct {
	elevation            *ElevationSessionService
	emitEvent            func(string, interface{})
	startElevatedSession func() (*ElevatedSessionStatus, error)
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

func (s *FilesystemService) emit(name string, data interface{}) {
	if s.emitEvent != nil {
		s.emitEvent(name, data)
	}
}
