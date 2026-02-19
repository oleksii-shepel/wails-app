package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// WorkspaceManager handles persistence of workspaces and preferences.
type WorkspaceManager struct {
	mu          sync.RWMutex
	storageDir  string
	workspaces  map[string]WorkspaceConfig
	activeID    string
	preferences UserPreferences
}

func NewWorkspaceManager() (*WorkspaceManager, error) {
	dir, err := appDataDir()
	if err != nil {
		return nil, err
	}

	wm := &WorkspaceManager{
		storageDir: dir,
		workspaces: make(map[string]WorkspaceConfig),
		preferences: defaultPreferences(),
	}

	_ = wm.load() // ignore error on first run
	return wm, nil
}

func appDataDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		base, err = os.UserHomeDir()
		if err != nil {
			return "", err
		}
	}
	dir := filepath.Join(base, "wails-file-manager")
	return dir, os.MkdirAll(dir, 0755)
}

// ============================================================================
// Workspace CRUD
// ============================================================================

func (wm *WorkspaceManager) ListWorkspaces() WorkspaceList {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	list := make([]WorkspaceConfig, 0, len(wm.workspaces))
	for _, ws := range wm.workspaces {
		list = append(list, ws)
	}

	return WorkspaceList{
		Workspaces:        list,
		ActiveWorkspaceID: wm.activeID,
	}
}

func (wm *WorkspaceManager) SaveWorkspace(ws WorkspaceConfig) error {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	if ws.ID == "" {
		return fmt.Errorf("workspace ID must not be empty")
	}

	ws.LastAccessedAt = time.Now().Unix()
	wm.workspaces[ws.ID] = ws

	if wm.activeID == "" {
		wm.activeID = ws.ID
	}

	return wm.persist()
}

func (wm *WorkspaceManager) LoadWorkspace(id string) (*WorkspaceConfig, error) {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	ws, ok := wm.workspaces[id]
	if !ok {
		return nil, fmt.Errorf("workspace %q not found", id)
	}

	ws.LastAccessedAt = time.Now().Unix()
	wm.workspaces[id] = ws
	wm.activeID = id

	_ = wm.persist()
	return &ws, nil
}

func (wm *WorkspaceManager) DeleteWorkspace(id string) error {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	if _, ok := wm.workspaces[id]; !ok {
		return fmt.Errorf("workspace %q not found", id)
	}

	delete(wm.workspaces, id)

	if wm.activeID == id {
		wm.activeID = ""
		// Set first available as active
		for k := range wm.workspaces {
			wm.activeID = k
			break
		}
	}

	return wm.persist()
}

func (wm *WorkspaceManager) SetActiveWorkspace(id string) error {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	if _, ok := wm.workspaces[id]; !ok {
		return fmt.Errorf("workspace %q not found", id)
	}
	wm.activeID = id
	return wm.persist()
}

// ============================================================================
// User Preferences
// ============================================================================

func (wm *WorkspaceManager) GetPreferences() UserPreferences {
	wm.mu.RLock()
	defer wm.mu.RUnlock()
	return wm.preferences
}

func (wm *WorkspaceManager) SavePreferences(prefs UserPreferences) error {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	wm.preferences = prefs
	return wm.persist()
}

// ============================================================================
// Persistence (JSON on disk)
// ============================================================================

type storedData struct {
	Workspaces        map[string]WorkspaceConfig `json:"workspaces"`
	ActiveWorkspaceID string                     `json:"activeWorkspaceId"`
	Preferences       UserPreferences            `json:"preferences"`
}

func (wm *WorkspaceManager) persist() error {
	data := storedData{
		Workspaces:        wm.workspaces,
		ActiveWorkspaceID: wm.activeID,
		Preferences:       wm.preferences,
	}

	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	path := filepath.Join(wm.storageDir, "state.json")
	return os.WriteFile(path, b, 0644)
}

func (wm *WorkspaceManager) load() error {
	path := filepath.Join(wm.storageDir, "state.json")
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var data storedData
	if err := json.Unmarshal(b, &data); err != nil {
		return err
	}

	if data.Workspaces != nil {
		wm.workspaces = data.Workspaces
	}
	wm.activeID = data.ActiveWorkspaceID

	// Only replace preferences if they were actually persisted
	if data.Preferences.Theme != "" {
		wm.preferences = data.Preferences
	}

	return nil
}

func defaultPreferences() UserPreferences {
	return UserPreferences{
		Theme:                   "auto",
		DefaultShowHidden:       false,
		DefaultSortBy:           "name",
		DefaultSortOrder:        "asc",
		EnableKeyboardShortcuts: true,
		DoubleClickAction:       "open",
		ConfirmDelete:           true,
	}
}
