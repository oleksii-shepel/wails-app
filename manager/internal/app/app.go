package app

import (
	"context"
	"sync"

	internalbrowserpane "wails-app/internal/browserpane"
	"wails-app/vfs/archive/cache"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App is the Wails binding surface and thin wrapper around internal state.
type App struct {
	*State
	isProcessElevated func() bool
	streamBaseURL     string
	streamBaseURLMu   sync.RWMutex
	browserPanes      *internalbrowserpane.Service
	browserPanesMu    sync.Mutex
}

func NewApp(isProcessElevated func() bool) *App {
	return &App{
		isProcessElevated: isProcessElevated,
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
	a.browserPanesMu.Lock()
	service := a.browserPanes
	a.browserPanesMu.Unlock()
	if service != nil {
		_ = service.Shutdown()
	}
	if a.State != nil {
		a.State.Shutdown()
	}
	if a.State != nil && a.State.PluginRegistry != nil {
		a.State.PluginRegistry.Shutdown()
	}
	cleanupSegmentedVideoPlaybackCache()
}

func (a *App) BeforeClose(ctx context.Context) bool {
	if a.State == nil || a.State.Workspace == nil {
		return false
	}

	prefs := a.State.Workspace.GetPreferences()
	if !prefs.ConfirmOnClose {
		return false
	}

	choice, err := wailsruntime.MessageDialog(ctx, wailsruntime.MessageDialogOptions{
		Type:    wailsruntime.QuestionDialog,
		Title:   "Close File Manager?",
		Message: "Are you sure you want to close the application?",
	})
	if err != nil {
		// A dialog failure must not trap the application in a non-closable state.
		return false
	}

	return beforeCloseChoicePreventsShutdown(choice)
}

func beforeCloseChoicePreventsShutdown(choice string) bool {
	// Wails QuestionDialog returns the canonical "Yes"/"No" result even when
	// Windows renders localized button captions.
	return choice != "Yes"
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

func (a *App) browserPaneService() *internalbrowserpane.Service {
	a.browserPanesMu.Lock()
	defer a.browserPanesMu.Unlock()
	if a.browserPanes == nil {
		a.browserPanes = internalbrowserpane.NewService(a.EmitEvent)
	}
	return a.browserPanes
}