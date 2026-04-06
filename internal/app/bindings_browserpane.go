package app

import (
	internalbrowserpane "wails-app/internal/browserpane"
	contracts "wails-app/internal/contracts"
)

func (a *App) BrowserPaneSync(req contracts.BrowserPaneSyncRequest) error {
	return a.browserPaneService().Sync(internalbrowserpane.Update{
		SurfaceID: req.SurfaceID,
		URL:       req.URL,
		Visible:   req.Visible,
		X:         req.X,
		Y:         req.Y,
		Width:     req.Width,
		Height:    req.Height,
	})
}

func (a *App) BrowserPaneDispose(surfaceID string) error {
	return a.browserPaneService().Dispose(surfaceID)
}
