package app

import internalruntime "wails-app/internal/runtime"

func (s *State) CreatePaneWatch() {
	s.DirWatch = internalruntime.NewPaneWatchManager(s.EmitEvent)
}

func (s *State) ClosePaneWatch() {
	if s.DirWatch != nil {
		s.DirWatch.Close()
	}
}

func (s *State) CancelDirStreams() {
	if s.DirStream != nil {
		s.DirStream.CancelAll()
	}
}

func (s *State) StopElevationSession() {
	if s.Elevation != nil {
		s.Elevation.StopSession()
	}
}

func (s *State) CloseArchiveCache() {
	s.ArchCache.CloseAll()
}
