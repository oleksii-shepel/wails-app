package app

import (
	"context"
	"fmt"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	internalruntime "wails-app/internal/runtime"
)

type RuntimeConfig struct {
	SetContext           func(context.Context)
	SetOperationEmitter  func(func(string, interface{}))
	CreatePaneWatch      func()
	InitVFS              func()
	HandleFileDrop       func(int, int, []string) error
	ClosePaneWatch       func()
	CancelDirStreams     func()
	StopElevationSession func()
	CloseArchiveCache    func()
}

type RuntimeService struct {
	config RuntimeConfig
}

func NewRuntimeService(config RuntimeConfig) *RuntimeService {
	return &RuntimeService{config: config}
}

func (s *RuntimeService) Startup(ctx context.Context) {
	if s.config.SetContext != nil {
		s.config.SetContext(ctx)
	}
	if s.config.SetOperationEmitter != nil {
		s.config.SetOperationEmitter(func(name string, data interface{}) {
			s.EmitEvent(ctx, name, data)
		})
	}
	if s.config.CreatePaneWatch != nil {
		s.config.CreatePaneWatch()
	}
	if s.config.InitVFS != nil {
		s.config.InitVFS()
	}
	internalruntime.BindNativeFileDrop(ctx, func(x, y int, paths []string) {
		if s.config.HandleFileDrop == nil {
			return
		}
		if err := s.config.HandleFileDrop(x, y, paths); err != nil {
			fmt.Printf("[wails:file-drop] HandleFileDrop error: %v\n", err)
		}
	})
}

func (s *RuntimeService) Shutdown() {
	if s.config.ClosePaneWatch != nil {
		s.config.ClosePaneWatch()
	}
	if s.config.CancelDirStreams != nil {
		s.config.CancelDirStreams()
	}
	if s.config.StopElevationSession != nil {
		s.config.StopElevationSession()
	}
	if s.config.CloseArchiveCache != nil {
		s.config.CloseArchiveCache()
	}
}

func (s *RuntimeService) EmitEvent(ctx context.Context, name string, data interface{}) {
	if ctx != nil {
		runtime.EventsEmit(ctx, name, data)
	}
}
