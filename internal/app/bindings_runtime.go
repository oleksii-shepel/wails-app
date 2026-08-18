package app

import internalruntime "wails-app/internal/runtime"

func (a *App) SetPaneDirectory(paneID, path string) error {
	if a.Navigation != nil {
		a.Navigation.SyncPath(paneID, path)
	}
	if a.DirWatch == nil {
		return nil
	}
	return a.DirWatch.SetPaneDirectory(paneID, path)
}

func (a *App) StartListDirStream(paneID, path string, showHidden bool) (string, error) {
	return a.DirStream.Start(paneID, path, showHidden)
}

func (a *App) StartListDirStreamWithOptions(paneID, path string, showHidden bool, options internalruntime.DirStreamOptions) (string, error) {
	return a.DirStream.StartWithOptions(paneID, path, showHidden, options)
}

func (a *App) ReadListDirStreamWindow(streamID string, offset, limit int) (*internalruntime.DirStreamWindow, error) {
	return a.DirStream.Window(streamID, offset, limit)
}

func (a *App) CancelListDirStream(streamID string) {
	a.DirStream.Cancel(streamID)
}

func (a *App) HandleFileDrop(x int, y int, paths []string) error {
	return a.Utility.HandleFileDrop(x, y, paths)
}
