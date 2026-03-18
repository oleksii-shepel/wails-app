package main

import (
	"context"
	"fmt"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	internalruntime "wails-app/internal/runtime"
)

// AppRuntimeService owns Wails lifecycle/runtime bridge concerns that don't
// belong to a domain service: startup/shutdown wiring, native event hookup,
// and app-level event emission.
type AppRuntimeService struct{}

func NewAppRuntimeService() *AppRuntimeService {
	return &AppRuntimeService{}
}

func (s *AppRuntimeService) Startup(app *App, ctx context.Context) {
	app.ctx = ctx
	app.operations.SetEmitter(app.emitEvent)
	app.dirWatch = internalruntime.NewPaneWatchManager(app.emitEvent)
	app.initVFS()
	s.bindNativeFileDrop(app, ctx)
}

func (s *AppRuntimeService) Shutdown(app *App) {
	if app.dirWatch != nil {
		app.dirWatch.Close()
	}
	app.dirStream.CancelAll()
	if app.elevation != nil {
		app.elevation.StopSession()
	}
	app.archCache.CloseAll()
}

func (s *AppRuntimeService) EmitEvent(ctx context.Context, name string, data interface{}) {
	if ctx != nil {
		runtime.EventsEmit(ctx, name, data)
	}
}

func (s *AppRuntimeService) bindNativeFileDrop(app *App, ctx context.Context) {
	runtime.EventsOn(ctx, "wails:file-drop", func(optionalData ...interface{}) {
		fmt.Println("[wails:file-drop] Backend listener fired!")
		x, y, paths, ok := s.parseNativeFileDrop(optionalData)
		if !ok {
			return
		}

		fmt.Printf("[wails:file-drop] x=%d, y=%d, paths=%v\n", x, y, paths)
		if err := app.HandleFileDrop(x, y, paths); err != nil {
			fmt.Printf("[wails:file-drop] HandleFileDrop error: %v\n", err)
		}
	})
}

func (s *AppRuntimeService) parseNativeFileDrop(optionalData []interface{}) (int, int, []string, bool) {
	if len(optionalData) == 0 {
		return 0, 0, nil, false
	}

	data, ok := optionalData[0].(map[string]interface{})
	if !ok {
		return 0, 0, nil, false
	}

	fmt.Printf("[wails:file-drop] Native event received: %+v\n", data)

	x := 0
	y := 0
	if xVal, ok := data["x"].(float64); ok {
		x = int(xVal)
	}
	if yVal, ok := data["y"].(float64); ok {
		y = int(yVal)
	}

	paths := make([]string, 0)
	if pathsVal, ok := data["paths"].([]interface{}); ok {
		for _, pathValue := range pathsVal {
			if path, ok := pathValue.(string); ok {
				paths = append(paths, path)
			}
		}
	}

	return x, y, paths, true
}
