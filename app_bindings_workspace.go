package main

func (a *App) GetWorkspaces() WorkspaceList {
	return a.Workspace.GetWorkspaces()
}

func (a *App) SaveWorkspace(ws WorkspaceConfig) error {
	return a.Workspace.SaveWorkspace(ws)
}

func (a *App) LoadWorkspace(id string) (*WorkspaceConfig, error) {
	return a.Workspace.LoadWorkspace(id)
}

func (a *App) DeleteWorkspace(id string) error {
	return a.Workspace.DeleteWorkspace(id)
}

func (a *App) GetPreferences() UserPreferences {
	return a.Workspace.GetPreferences()
}

func (a *App) SavePreferences(prefs UserPreferences) error {
	return a.Workspace.SavePreferences(prefs)
}
