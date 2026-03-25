package app

import (
	"context"
	"sync"

	"wails-app/vfs/archive/cache"
)

// App is the Wails binding surface and thin wrapper around internal state.
type App struct {
	*State
	isProcessElevated func() bool
	streamBaseURL     string
	streamBaseURLMu   sync.RWMutex
}

func NewApp(isProcessElevated func() bool) *App {
	return &App{
		isProcessElevated: isProcessElevated,
	}
}

func (a *App) BindState(state *State) {
	a.State = state
}

func (a *App) Startup(ctx context.Context) {
	if a.State != nil {
		a.State.Startup(ctx)
	}
}

func (a *App) Shutdown() {
	if a.State != nil {
		a.State.Shutdown()
	}
	cleanupSegmentedVideoPlaybackCache()
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

func (a *App) SetStreamBaseURL(baseURL string) {
	a.streamBaseURLMu.Lock()
	a.streamBaseURL = baseURL
	a.streamBaseURLMu.Unlock()
}

func (a *App) StreamBaseURL() string {
	a.streamBaseURLMu.RLock()
	defer a.streamBaseURLMu.RUnlock()
	return a.streamBaseURL
}

// ArchCache returns the shared archive LRU cache so the stream middleware
// can open archive entries without buffering them in JS.
func (a *App) ArchCache() *cache.Cache {
	if a.State == nil {
		return nil
	}
	return a.State.archCache
}
