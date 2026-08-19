package workspace

import contracts "wails-app/internal/contracts"

// Service owns workspace and preference orchestration while delegating persistence details to Manager.
type Service struct {
	manager *Manager
}

func NewService(manager *Manager) *Service {
	return &Service{manager: manager}
}

func (s *Service) GetWorkspaces() contracts.WorkspaceList {
	return s.manager.ListWorkspaces()
}

func (s *Service) SaveWorkspace(workspace contracts.WorkspaceConfig) error {
	return s.manager.SaveWorkspace(workspace)
}

func (s *Service) LoadWorkspace(id string) (*contracts.WorkspaceConfig, error) {
	return s.manager.LoadWorkspace(id)
}

func (s *Service) DeleteWorkspace(id string) error {
	return s.manager.DeleteWorkspace(id)
}

func (s *Service) GetPreferences() contracts.UserPreferences {
	return s.manager.GetPreferences()
}

func (s *Service) SavePreferences(preferences contracts.UserPreferences) error {
	return s.manager.SavePreferences(preferences)
}