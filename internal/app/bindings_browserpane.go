package app

import (
	internalbrowserpane "wails-app/internal/browserpane"
	contracts "wails-app/internal/contracts"
)

func (a *App) BrowserPaneSync(req contracts.BrowserPaneSyncRequest) error {
	return a.browserPaneService().Sync(internalbrowserpane.Update{
		SurfaceID:       req.SurfaceID,
		URL:             req.URL,
		Theme:           req.Theme,
		BackgroundColor: req.BackgroundColor,
		Visible:         req.Visible,
		X:               req.X,
		Y:               req.Y,
		Width:           req.Width,
		Height:          req.Height,
	})
}

func (a *App) BrowserPaneDispose(surfaceID string) error {
	return a.browserPaneService().Dispose(surfaceID)
}

// BrowserPaneCaptureScreenshot returns a base64-encoded PNG screenshot of the browser pane content.
func (a *App) BrowserPaneCaptureScreenshot(surfaceID string) (string, error) {
	return a.browserPaneService().CaptureScreenshot(surfaceID)
}

// BrowserPaneGetDocumentBackground returns the current page background color from the child webview.
func (a *App) BrowserPaneGetDocumentBackground(surfaceID string) (string, error) {
	return a.browserPaneService().GetDocumentBackground(surfaceID)
}

// BrowserPanePostMessage posts a serialized message into the child webview.
func (a *App) BrowserPanePostMessage(surfaceID string, message string) error {
	return a.browserPaneService().PostMessage(surfaceID, message)
}
