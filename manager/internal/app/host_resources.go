package app

import (
	"sync"

	internalbrowserpane "wails-app/internal/browserpane"
)

// hostResources owns Wails-host runtime resources that are not domain state.
// Keeping them separate prevents App from becoming the lifecycle owner for
// unrelated infrastructure as new bindings are added.
type hostResources struct {
	streamMu      sync.RWMutex
	streamBaseURL string

	browserPaneMu sync.Mutex
	browserPanes  *internalbrowserpane.Service
}

func newHostResources() *hostResources {
	return &hostResources{}
}

func (r *hostResources) setStreamBaseURL(baseURL string) {
	if r == nil {
		return
	}
	r.streamMu.Lock()
	r.streamBaseURL = baseURL
	r.streamMu.Unlock()
}

func (r *hostResources) streamURL() string {
	if r == nil {
		return ""
	}
	r.streamMu.RLock()
	defer r.streamMu.RUnlock()
	return r.streamBaseURL
}

func (r *hostResources) browserPaneService(emitEvent func(string, interface{})) *internalbrowserpane.Service {
	r.browserPaneMu.Lock()
	defer r.browserPaneMu.Unlock()
	if r.browserPanes == nil {
		r.browserPanes = internalbrowserpane.NewService(emitEvent)
	}
	return r.browserPanes
}

func (r *hostResources) shutdown() {
	if r == nil {
		return
	}

	// Detach before shutting down so a repeated shutdown is idempotent and the
	// resource is no longer retained by the composition root.
	r.browserPaneMu.Lock()
	service := r.browserPanes
	r.browserPanes = nil
	r.browserPaneMu.Unlock()

	if service != nil {
		_ = service.Shutdown()
	}
}

func (a *App) SetStreamBaseURL(baseURL string) {
	a.hostResourcesOrCreate().setStreamBaseURL(baseURL)
}

func (a *App) StreamBaseURL() string {
	return a.hostResourcesOrCreate().streamURL()
}

func (a *App) browserPaneService() *internalbrowserpane.Service {
	return a.hostResourcesOrCreate().browserPaneService(a.EmitEvent)
}

func (a *App) hostResourcesOrCreate() *hostResources {
	if a.hostResources == nil {
		a.hostResources = newHostResources()
	}
	return a.hostResources
}
