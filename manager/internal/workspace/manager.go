package workspace

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	contracts "wails-app/internal/contracts"
	"wails-app/internal/pathref"
)

// Manager owns workspace persistence and converts between storage models and wire contracts.
type Manager struct {
	mu          sync.RWMutex
	store       *Store
	workspaces  map[string]Config
	activeID    string
	preferences Preferences
}

func NewManager() (*Manager, error) {
	store, err := NewStore("wails-file-manager")
	if err != nil {
		return nil, err
	}

	manager := newManager(store)
	_ = manager.load()
	return manager, nil
}

func NewTransientManager() *Manager {
	return newManager(nil)
}

func newManager(store *Store) *Manager {
	return &Manager{
		store:       store,
		workspaces:  make(map[string]Config),
		preferences: toStoragePreferences(DefaultPreferences()),
	}
}

func (m *Manager) ListWorkspaces() contracts.WorkspaceList {
	m.mu.RLock()
	defer m.mu.RUnlock()

	list := make([]contracts.WorkspaceConfig, 0, len(m.workspaces))
	for _, workspace := range m.workspaces {
		list = append(list, fromStorageWorkspaceConfig(workspace))
	}

	return contracts.WorkspaceList{
		Workspaces:        list,
		ActiveWorkspaceID: m.activeID,
	}
}

func (m *Manager) SaveWorkspace(workspace contracts.WorkspaceConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if workspace.ID == "" {
		return fmt.Errorf("workspace ID must not be empty")
	}

	stored := toStorageWorkspaceConfig(workspace)
	stored.LastAccessedAt = time.Now().Unix()
	m.workspaces[stored.ID] = stored

	if m.activeID == "" {
		m.activeID = stored.ID
	}

	return m.persist()
}

func (m *Manager) LoadWorkspace(id string) (*contracts.WorkspaceConfig, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	stored, ok := m.workspaces[id]
	if !ok {
		return nil, fmt.Errorf("workspace %q not found", id)
	}

	stored.LastAccessedAt = time.Now().Unix()
	m.workspaces[id] = stored
	m.activeID = id

	// The load itself succeeded, so keep returning the workspace; the
	// persistence failure still gets reported rather than silently
	// dropping the updated last-accessed timestamp.
	if err := m.persist(); err != nil {
		fmt.Printf("Warning: could not persist workspace %q after load: %v\n", id, err)
	}
	workspace := fromStorageWorkspaceConfig(stored)
	return &workspace, nil
}

func (m *Manager) DeleteWorkspace(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.workspaces[id]; !ok {
		return fmt.Errorf("workspace %q not found", id)
	}

	delete(m.workspaces, id)
	if m.activeID == id {
		// Pick a deterministic successor instead of relying on Go's
		// randomized map iteration.
		m.activeID = ""
		for key := range m.workspaces {
			if m.activeID == "" || key < m.activeID {
				m.activeID = key
			}
		}
	}

	return m.persist()
}

func (m *Manager) SetActiveWorkspace(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.workspaces[id]; !ok {
		return fmt.Errorf("workspace %q not found", id)
	}
	m.activeID = id
	return m.persist()
}

func (m *Manager) GetPreferences() contracts.UserPreferences {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return fromStoragePreferences(m.preferences)
}

func (m *Manager) SavePreferences(preferences contracts.UserPreferences) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.preferences = toStoragePreferences(preferences)
	return m.persist()
}

func (m *Manager) persist() error {
	if m.store == nil {
		return nil
	}

	state := State{
		SchemaVersion:     CurrentStateSchemaVersion,
		Workspaces:        m.workspaces,
		ActiveWorkspaceID: m.activeID,
		Preferences:       m.preferences,
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	return m.store.WriteState(data)
}

func (m *Manager) load() error {
	if m.store == nil {
		return nil
	}

	data, err := m.store.ReadState()
	if err != nil {
		return err
	}

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return err
	}

	state, err = migrateState(state, data)
	if err != nil {
		return err
	}

	if state.Workspaces != nil {
		m.workspaces = state.Workspaces
	}
	m.activeID = state.ActiveWorkspaceID
	if state.Preferences.Theme != "" {
		m.preferences = mergePreferenceDefaults(state.Preferences)
	}

	return nil
}

// stateMigrations upgrades a State of version N to version N+1. Add an entry
// here whenever the persisted shape changes incompatibly. The raw JSON is
// passed alongside so a migration can rescue fields that were dropped during
// the typed unmarshal.
var stateMigrations = map[int]func(state State, raw []byte) (State, error){
	0: func(state State, _ []byte) (State, error) {
		// Original unversioned shape. No reshaping needed; just stamp.
		state.SchemaVersion = 1
		return state, nil
	},
}

func migrateState(state State, raw []byte) (State, error) {
	for state.SchemaVersion < CurrentStateSchemaVersion {
		migrate, ok := stateMigrations[state.SchemaVersion]
		if !ok {
			return state, fmt.Errorf("workspace: no migration registered from version %d", state.SchemaVersion)
		}
		next, err := migrate(state, raw)
		if err != nil {
			return state, err
		}
		state = next
	}
	return state, nil
}

func mergePreferenceDefaults(stored Preferences) Preferences {
	defaults := toStoragePreferences(DefaultPreferences())
	if stored.UIZoom <= 0 {
		stored.UIZoom = defaults.UIZoom
	}
	return stored
}

// LoadStoredWindowState reads the persisted window state directly from disk
// without instantiating a Manager. Intended for use before wails.Run() so the
// window can be restored at its previous size, position, and maximised state.
// Returns nil when no state has been written yet or the file is unreadable.
func LoadStoredWindowState(appName string) *contracts.WindowState {
	store, err := NewStore(appName)
	if err != nil {
		return nil
	}
	data, err := store.ReadState()
	if err != nil {
		return nil
	}
	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return nil
	}
	return fromStorageWindowState(state.Preferences.WindowState)
}

// LoadStoredState reads the persisted workspace state directly from disk
// without instantiating a Manager. Returns nil when no state has been written
// yet or the file is unreadable.
func LoadStoredState(appName string) *State {
	store, err := NewStore(appName)
	if err != nil {
		return nil
	}
	data, err := store.ReadState()
	if err != nil {
		return nil
	}
	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return nil
	}
	migrated, err := migrateState(state, data)
	if err != nil {
		return nil
	}
	return &migrated
}

// StoredStateRequiresElevation reports whether startup should relaunch elevated
// before restoring persisted panes. When the startup workspace is unambiguous
// (reopen-last with an existing active workspace) only that workspace decides;
// otherwise the check stays on the safe side and requires elevation when any
// stored workspace holds registry-backed pane locations, so the initial restore
// does not fall back to a generic root before elevated access is available.
func StoredStateRequiresElevation(appName string) bool {
	state := LoadStoredState(appName)
	return state != nil && storedStateRequiresElevation(*state)
}

func storedStateRequiresElevation(state State) bool {
	if len(state.Workspaces) == 0 {
		return false
	}

	if state.Preferences.ReopenLastWorkspace && state.ActiveWorkspaceID != "" {
		if workspace, ok := state.Workspaces[state.ActiveWorkspaceID]; ok {
			return workspaceRequiresElevation(workspace)
		}
	}

	// The startup workspace is ambiguous otherwise (nothing is auto-restored),
	// so stay on the safe side and pre-elevate if any stored workspace needs
	// elevated paths.
	for _, workspace := range state.Workspaces {
		if workspaceRequiresElevation(workspace) {
			return true
		}
	}
	return false
}

func workspaceRequiresElevation(workspace Config) bool {
	return paneRequiresElevation(workspace.LeftPane) || paneRequiresElevation(workspace.RightPane)
}

func paneRequiresElevation(pane PaneState) bool {
	for _, tab := range pane.Tabs {
		if tabRequiresElevation(tab) {
			return true
		}
	}
	return false
}

func tabRequiresElevation(tab TabInfo) bool {
	candidates := []string{
		tab.Path,
		tab.ViewerPath(),
	}
	if tab.Location != nil {
		candidates = append(candidates, tab.Location.Location)
	}
	for _, candidate := range candidates {
		if pathref.IsRegistry(pathref.Normalize(candidate)) {
			return true
		}
	}
	return false
}

func (t TabInfo) ViewerPath() string {
	if t.Viewer == nil {
		return ""
	}
	if t.Viewer.TargetPath != "" {
		return t.Viewer.TargetPath
	}
	return t.Viewer.SourceDir
}

func DefaultPreferences() contracts.UserPreferences {
	return contracts.UserPreferences{
		Theme:                   "system",
		AccentColor:             "",
		UIZoom:                  100,
		CompactMode:             false,
		AnimationsEnabled:       true,
		DefaultShowHidden:       false,
		DefaultSortBy:           "name",
		DefaultSortOrder:        "asc",
		EnableKeyboardShortcuts: true,
		DoubleClickAction:       "open",
		ConfirmDelete:           true,
		ConfirmOnClose:          true,
		ReopenLastWorkspace:     true,
		MemoryRoot:              "",
	}
}

func toStorageWorkspaceConfig(source contracts.WorkspaceConfig) Config {
	return Config{
		ID:             source.ID,
		Name:           source.Name,
		Icon:           source.Icon,
		Color:          source.Color,
		LeftPane:       toStoragePaneState(source.LeftPane),
		RightPane:      toStoragePaneState(source.RightPane),
		ShowHidden:     source.ShowHidden,
		ActivePaneID:   source.ActivePaneID,
		SplitRatio:     source.SplitRatio,
		CreatedAt:      source.CreatedAt,
		LastAccessedAt: source.LastAccessedAt,
	}
}

func fromStorageWorkspaceConfig(source Config) contracts.WorkspaceConfig {
	return contracts.WorkspaceConfig{
		ID:             source.ID,
		Name:           source.Name,
		Icon:           source.Icon,
		Color:          source.Color,
		LeftPane:       fromStoragePaneState(source.LeftPane),
		RightPane:      fromStoragePaneState(source.RightPane),
		ShowHidden:     source.ShowHidden,
		ActivePaneID:   source.ActivePaneID,
		SplitRatio:     source.SplitRatio,
		CreatedAt:      source.CreatedAt,
		LastAccessedAt: source.LastAccessedAt,
	}
}

func toStoragePaneState(source contracts.PaneState) PaneState {
	tabs := make([]TabInfo, 0, len(source.Tabs))
	for _, tab := range source.Tabs {
		tabs = append(tabs, toStorageTabInfo(tab))
	}
	return PaneState{
		ID:          source.ID,
		Tabs:        tabs,
		ActiveTabID: source.ActiveTabID,
	}
}

func fromStoragePaneState(source PaneState) contracts.PaneState {
	tabs := make([]contracts.TabInfo, 0, len(source.Tabs))
	for _, tab := range source.Tabs {
		tabs = append(tabs, fromStorageTabInfo(tab))
	}
	return contracts.PaneState{
		ID:          source.ID,
		Tabs:        tabs,
		ActiveTabID: source.ActiveTabID,
	}
}

func toStorageTabInfo(source contracts.TabInfo) TabInfo {
	var location *LocationRef
	if source.Location != nil {
		location = &LocationRef{
			Location: source.Location.Location,
			Scheme:   source.Location.Scheme,
			Kind:     source.Location.Kind,
		}
	}

	var viewer *ViewerTab
	if source.Viewer != nil {
		viewer = &ViewerTab{
			TargetPath:  source.Viewer.TargetPath,
			ViewerID:    source.Viewer.ViewerID,
			AutoFollow:  source.Viewer.AutoFollow,
			SourcePane:  source.Viewer.SourcePane,
			SourceTabID: source.Viewer.SourceTabID,
			SourceDir:   source.Viewer.SourceDir,
		}
	}

	return TabInfo{
		ID:            source.ID,
		Title:         source.Title,
		Path:          source.Path,
		Location:      location,
		SurfaceID:     source.SurfaceID,
		Kind:          source.Kind,
		Viewer:        viewer,
		IsClosable:    source.IsClosable,
		IsActive:      source.IsActive,
		IsPinned:      source.IsPinned,
		CreatedAt:     source.CreatedAt,
		ViewStateJson: source.ViewStateJson,
	}
}

func fromStorageTabInfo(source TabInfo) contracts.TabInfo {
	var location *contracts.LocationRef
	if source.Location != nil {
		location = &contracts.LocationRef{
			Location: source.Location.Location,
			Scheme:   source.Location.Scheme,
			Kind:     source.Location.Kind,
		}
	}

	var viewer *contracts.ViewerTab
	if source.Viewer != nil {
		viewer = &contracts.ViewerTab{
			TargetPath:  source.Viewer.TargetPath,
			ViewerID:    source.Viewer.ViewerID,
			AutoFollow:  source.Viewer.AutoFollow,
			SourcePane:  source.Viewer.SourcePane,
			SourceTabID: source.Viewer.SourceTabID,
			SourceDir:   source.Viewer.SourceDir,
		}
	}

	return contracts.TabInfo{
		ID:            source.ID,
		Title:         source.Title,
		Path:          source.Path,
		Location:      location,
		SurfaceID:     source.SurfaceID,
		Kind:          source.Kind,
		Viewer:        viewer,
		IsClosable:    source.IsClosable,
		IsActive:      source.IsActive,
		IsPinned:      source.IsPinned,
		CreatedAt:     source.CreatedAt,
		ViewStateJson: source.ViewStateJson,
	}
}

func toStoragePreferences(source contracts.UserPreferences) Preferences {
	return Preferences{
		Theme:                   source.Theme,
		AccentColor:             source.AccentColor,
		UIZoom:                  source.UIZoom,
		CompactMode:             source.CompactMode,
		AnimationsEnabled:       source.AnimationsEnabled,
		WindowState:             toStorageWindowState(source.WindowState),
		DefaultShowHidden:       source.DefaultShowHidden,
		DefaultSortBy:           source.DefaultSortBy,
		DefaultSortOrder:        source.DefaultSortOrder,
		EnableKeyboardShortcuts: source.EnableKeyboardShortcuts,
		DoubleClickAction:       source.DoubleClickAction,
		ConfirmDelete:           source.ConfirmDelete,
		ConfirmOnClose:          source.ConfirmOnClose,
		ReopenLastWorkspace:     source.ReopenLastWorkspace,
		MemoryRoot:              source.MemoryRoot,
	}
}

func fromStoragePreferences(source Preferences) contracts.UserPreferences {
	return contracts.UserPreferences{
		Theme:                   source.Theme,
		AccentColor:             source.AccentColor,
		UIZoom:                  source.UIZoom,
		CompactMode:             source.CompactMode,
		AnimationsEnabled:       source.AnimationsEnabled,
		WindowState:             fromStorageWindowState(source.WindowState),
		DefaultShowHidden:       source.DefaultShowHidden,
		DefaultSortBy:           source.DefaultSortBy,
		DefaultSortOrder:        source.DefaultSortOrder,
		EnableKeyboardShortcuts: source.EnableKeyboardShortcuts,
		DoubleClickAction:       source.DoubleClickAction,
		ConfirmDelete:           source.ConfirmDelete,
		ConfirmOnClose:          source.ConfirmOnClose,
		ReopenLastWorkspace:     source.ReopenLastWorkspace,
		MemoryRoot:              source.MemoryRoot,
	}
}

func toStorageWindowState(source *contracts.WindowState) *WindowState {
	if source == nil {
		return nil
	}
	return &WindowState{
		Width:       source.Width,
		Height:      source.Height,
		X:           source.X,
		Y:           source.Y,
		Maximised:   source.Maximised,
		HasPosition: source.HasPosition,
	}
}

func fromStorageWindowState(source *WindowState) *contracts.WindowState {
	if source == nil {
		return nil
	}
	return &contracts.WindowState{
		Width:       source.Width,
		Height:      source.Height,
		X:           source.X,
		Y:           source.Y,
		Maximised:   source.Maximised,
		HasPosition: source.HasPosition,
	}
}
