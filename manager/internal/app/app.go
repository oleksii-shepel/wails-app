package app

import (
	"context"

	"wails-app/vfs/archive/cache"
)

// App is the Wails binding surface and thin wrapper around internal state.
type App struct {
	*State
	isProcessElevated func() bool
	hostResources     *hostResources
}

func NewApp(isProcessElevated func() bool) *App {
	return &App{
		isProcessElevated: isProcessElevated,
		hostResources:     newHostResources(),
	}
}

func (a *App) bindState(state *State) {
	a.State = state
}

func (a *App) Startup(ctx context.Context) {
	if a.State != nil {
		a.State.Startup(ctx)
	}
}

func (a *App) Shutdown() {
	a.hostResourcesOrCreate().shutdown()
	if a.State != nil {
		a.State.Shutdown()
	}
	if a.State != nil && a.State.PluginRegistry != nil {
		a.State.PluginRegistry.Shutdown()
	}
	cleanupSegmentedVideoPlaybackCache()
}

func (a *App) BeforeClose(ctx context.Context) bool {
	return false
}

func (a *App) EmitEvent(name string, data interface{}) {
	if a.State != nil {
		a.State.EmitEvent(name, data)
	}
}

func (a *App) InitVFS() {
	if a.State != nil {
		a.State.InitVFS()
	}
}

func (a *App) IsProcessElevated() bool {
	return a.isProcessElevated != nil && a.isProcessElevated()
}

// ArchCache returns the shared archive LRU cache so the stream middleware
// can open archive entries without buffering them in JS.
func (a *App) ArchCache() *cache.Cache {
	if a.State == nil {
		return nil
	}
	return a.State.archCache
}
