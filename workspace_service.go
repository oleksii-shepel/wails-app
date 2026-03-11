package main

// WorkspaceService owns workspace and preference orchestration while
// delegating persistence details to WorkspaceManager.
type WorkspaceService struct {
	manager *WorkspaceManager
}

func NewWorkspaceService(manager *WorkspaceManager) *WorkspaceService {
	return &WorkspaceService{manager: manager}
}

func (s *WorkspaceService) GetWorkspaces() WorkspaceList {
	return s.manager.ListWorkspaces()
}

func (s *WorkspaceService) SaveWorkspace(ws WorkspaceConfig) error {
	return s.manager.SaveWorkspace(ws)
}

func (s *WorkspaceService) LoadWorkspace(id string) (*WorkspaceConfig, error) {
	return s.manager.LoadWorkspace(id)
}

func (s *WorkspaceService) DeleteWorkspace(id string) error {
	return s.manager.DeleteWorkspace(id)
}

func (s *WorkspaceService) GetPreferences() UserPreferences {
	return s.manager.GetPreferences()
}

func (s *WorkspaceService) SavePreferences(prefs UserPreferences) error {
	return s.manager.SavePreferences(prefs)
}
