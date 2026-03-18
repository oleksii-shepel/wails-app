package main

func (a *App) GetWorkspaces() WorkspaceList {
	return a.workspace.GetWorkspaces()
}

func (a *App) SaveWorkspace(ws WorkspaceConfig) error {
	return a.workspace.SaveWorkspace(ws)
}

func (a *App) LoadWorkspace(id string) (*WorkspaceConfig, error) {
	return a.workspace.LoadWorkspace(id)
}

func (a *App) DeleteWorkspace(id string) error {
	return a.workspace.DeleteWorkspace(id)
}

func (a *App) GetPreferences() UserPreferences {
	return a.workspace.GetPreferences()
}

func (a *App) SavePreferences(prefs UserPreferences) error {
	return a.workspace.SavePreferences(prefs)
}
