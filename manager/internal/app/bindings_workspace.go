package app

import contracts "wails-app/internal/contracts"

func (a *App) GetWorkspaces() contracts.WorkspaceList {
	return a.Workspace.GetWorkspaces()
}

func (a *App) SaveWorkspace(ws contracts.WorkspaceConfig) error {
	return a.Workspace.SaveWorkspace(ws)
}

func (a *App) LoadWorkspace(id string) (*contracts.WorkspaceConfig, error) {
	return a.Workspace.LoadWorkspace(id)
}

func (a *App) DeleteWorkspace(id string) error {
	return a.Workspace.DeleteWorkspace(id)
}

func (a *App) GetPreferences() contracts.UserPreferences {
	return a.Workspace.GetPreferences()
}

func (a *App) SavePreferences(prefs contracts.UserPreferences) error {
	return a.Workspace.SavePreferences(prefs)
}

func (a *App) NormalizeWindowStateForSave(state contracts.WindowState) contracts.WindowState {
	return normalizeWindowStateForSave(state)
}

func (a *App) GetMaxWindowViewport(state contracts.WindowState) contracts.WindowViewport {
	return maxWindowViewport(state)
}
