package main

func (a *App) NavNavigate(paneID, path string) (*DirectoryListing, error) {
	return a.navigation.Navigate(paneID, path)
}

func (a *App) NavNavigateShowHidden(paneID, path string, showHidden bool) (*DirectoryListing, error) {
	return a.navigation.NavigateShowHidden(paneID, path, showHidden)
}

func (a *App) NavBack(paneID string) (*DirectoryListing, error) {
	return a.navigation.Back(paneID)
}

func (a *App) NavForward(paneID string) (*DirectoryListing, error) {
	return a.navigation.Forward(paneID)
}

func (a *App) NavGoUp(paneID string) (*DirectoryListing, error) {
	return a.navigation.GoUp(paneID)
}
