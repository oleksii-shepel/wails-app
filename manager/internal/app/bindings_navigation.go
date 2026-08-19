package app

import contracts "wails-app/internal/contracts"

func (a *App) NavNavigate(paneID, path string) (*contracts.DirectoryListing, error) {
	return a.Navigation.Navigate(paneID, path)
}

func (a *App) NavNavigateShowHidden(paneID, path string, showHidden bool) (*contracts.DirectoryListing, error) {
	return a.Navigation.NavigateShowHidden(paneID, path, showHidden)
}

func (a *App) NavBack(paneID string) (*contracts.DirectoryListing, error) {
	return a.Navigation.Back(paneID)
}

func (a *App) NavForward(paneID string) (*contracts.DirectoryListing, error) {
	return a.Navigation.Forward(paneID)
}

func (a *App) NavGoUp(paneID string) (*contracts.DirectoryListing, error) {
	return a.Navigation.GoUp(paneID)
}