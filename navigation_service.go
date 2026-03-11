package main

import (
	"fmt"
	"path/filepath"
)

// NavigationService owns pane navigation orchestration while delegating history
// storage to NavigationManager and directory listing to the filesystem layer.
type NavigationService struct {
	manager       *NavigationManager
	listDirectory func(path string, showHidden bool) (*DirectoryListing, error)
}

func NewNavigationService(manager *NavigationManager, listDirectory func(path string, showHidden bool) (*DirectoryListing, error)) *NavigationService {
	return &NavigationService{
		manager:       manager,
		listDirectory: listDirectory,
	}
}

func (s *NavigationService) Navigate(paneID, path string) (*DirectoryListing, error) {
	s.manager.Navigate(paneID, path)
	return s.listDirectory(path, false)
}

func (s *NavigationService) NavigateShowHidden(paneID, path string, showHidden bool) (*DirectoryListing, error) {
	s.manager.Navigate(paneID, path)
	return s.listDirectory(path, showHidden)
}

func (s *NavigationService) Back(paneID string) (*DirectoryListing, error) {
	path := s.manager.GoBack(paneID)
	if path == "" {
		return nil, fmt.Errorf("no previous location for pane %q", paneID)
	}
	return s.listDirectory(path, false)
}

func (s *NavigationService) Forward(paneID string) (*DirectoryListing, error) {
	path := s.manager.GoForward(paneID)
	if path == "" {
		return nil, fmt.Errorf("no forward location for pane %q", paneID)
	}
	return s.listDirectory(path, false)
}

func (s *NavigationService) GoUp(paneID string) (*DirectoryListing, error) {
	current := s.manager.CurrentPath(paneID)
	if current == "" {
		return nil, fmt.Errorf("no current path for pane %q", paneID)
	}
	parent := toSlash(filepath.Dir(current))
	if parent == toSlash(current) {
		return nil, fmt.Errorf("already at root")
	}
	return s.Navigate(paneID, parent)
}
