package main

func (a *App) SetPaneDirectory(paneID, path string) error {
	if a.dirWatch == nil {
		return nil
	}
	return a.dirWatch.SetPaneDirectory(paneID, path)
}

func (a *App) StartListDirStream(paneID, path string, showHidden bool) (string, error) {
	return a.dirStream.Start(paneID, path, showHidden)
}

func (a *App) CancelListDirStream(streamID string) {
	a.dirStream.Cancel(streamID)
}

func (a *App) HandleFileDrop(x int, y int, paths []string) error {
	return a.utility.HandleFileDrop(x, y, paths)
}