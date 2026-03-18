package main

func (a *App) SetPaneDirectory(paneID, path string) error {
	if a.DirWatch == nil {
		return nil
	}
	return a.DirWatch.SetPaneDirectory(paneID, path)
}

func (a *App) StartListDirStream(paneID, path string, showHidden bool) (string, error) {
	return a.DirStream.Start(paneID, path, showHidden)
}

func (a *App) CancelListDirStream(streamID string) {
	a.DirStream.Cancel(streamID)
}

func (a *App) HandleFileDrop(x int, y int, paths []string) error {
	return a.Utility.HandleFileDrop(x, y, paths)
}
