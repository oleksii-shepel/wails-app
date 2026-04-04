package navigation

import (
	"fmt"
	"path/filepath"

	contracts "wails-app/internal/contracts"
	"wails-app/internal/pathref"
)

// Service owns pane navigation orchestration while delegating history storage
// to Manager and directory listing to the filesystem layer.
type Service struct {
	manager       *Manager
	listDirectory func(path string, showHidden bool) (*contracts.DirectoryListing, error)
}

func NewService(manager *Manager, listDirectory func(path string, showHidden bool) (*contracts.DirectoryListing, error)) *Service {
	return &Service{
		manager:       manager,
		listDirectory: listDirectory,
	}
}

func (s *Service) Navigate(paneID, path string) (*contracts.DirectoryListing, error) {
	s.manager.Navigate(paneID, path)
	return s.listDirectory(path, false)
}

func (s *Service) NavigateShowHidden(paneID, path string, showHidden bool) (*contracts.DirectoryListing, error) {
	s.manager.Navigate(paneID, path)
	return s.listDirectory(path, showHidden)
}

func (s *Service) Back(paneID string) (*contracts.DirectoryListing, error) {
	path := s.manager.GoBack(paneID)
	if path == "" {
		return nil, fmt.Errorf("no previous location for pane %q", paneID)
	}
	return s.listDirectory(path, false)
}

func (s *Service) Forward(paneID string) (*contracts.DirectoryListing, error) {
	path := s.manager.GoForward(paneID)
	if path == "" {
		return nil, fmt.Errorf("no forward location for pane %q", paneID)
	}
	return s.listDirectory(path, false)
}

func (s *Service) GoUp(paneID string) (*contracts.DirectoryListing, error) {
	current := s.manager.CurrentPath(paneID)
	if current == "" {
		return nil, fmt.Errorf("no current path for pane %q", paneID)
	}
	if parent, ok := pathref.Parent(current); ok {
		return s.Navigate(paneID, parent)
	}
	parent := filepath.ToSlash(filepath.Dir(current))
	if parent == filepath.ToSlash(current) {
		return nil, fmt.Errorf("already at root")
	}
	return s.Navigate(paneID, parent)
}

// SyncPath updates the backend navigation manager to match a pane's current
// path without triggering another directory read.
func (s *Service) SyncPath(paneID, path string) {
	if path == "" {
		return
	}
	s.manager.Navigate(paneID, path)
}
