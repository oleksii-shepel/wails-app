package workspace

import (
	"os"
	"path/filepath"
	"testing"

	contracts "wails-app/internal/contracts"
)

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	store := &Store{statePath: filepath.Join(t.TempDir(), "state.json")}
	m := newManager(store)
	return m
}

func sampleWorkspaceConfig(id string) contracts.WorkspaceConfig {
	return contracts.WorkspaceConfig{
		ID:   id,
		Name: "Test " + id,
		LeftPane: contracts.PaneState{
			ID: "left",
			Tabs: []contracts.TabInfo{
				{ID: "t1", Title: "Home", Path: "/home"},
			},
			ActiveTabID: "t1",
		},
		RightPane: contracts.PaneState{
			ID: "right",
			Tabs: []contracts.TabInfo{
				{ID: "t2", Title: "Docs", Path: "/docs"},
			},
			ActiveTabID: "t2",
		},
		CreatedAt: 1234567890,
	}
}

func TestSaveWorkspaceRequiresID(t *testing.T) {
	m := newTestManager(t)
	if err := m.SaveWorkspace(contracts.WorkspaceConfig{ID: ""}); err == nil {
		t.Fatal("expected error for empty workspace ID")
	}
}

func TestSaveAndListWorkspaces(t *testing.T) {
	m := newTestManager(t)
	w1 := sampleWorkspaceConfig("w1")
	w2 := sampleWorkspaceConfig("w2")

	if err := m.SaveWorkspace(w1); err != nil {
		t.Fatalf("save w1 failed: %v", err)
	}
	if err := m.SaveWorkspace(w2); err != nil {
		t.Fatalf("save w2 failed: %v", err)
	}

	list := m.ListWorkspaces()
	if len(list.Workspaces) != 2 {
		t.Fatalf("expected 2 workspaces, got %d", len(list.Workspaces))
	}
	if list.ActiveWorkspaceID == "" {
		t.Fatal("expected active workspace to be set automatically")
	}
}

func TestLoadWorkspaceSetsActive(t *testing.T) {
	m := newTestManager(t)
	w1 := sampleWorkspaceConfig("w1")
	w2 := sampleWorkspaceConfig("w2")
	_ = m.SaveWorkspace(w1)
	_ = m.SaveWorkspace(w2)

	loaded, err := m.LoadWorkspace("w2")
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if loaded.ID != "w2" {
		t.Fatalf("expected loaded ID w2, got %s", loaded.ID)
	}

	list := m.ListWorkspaces()
	if list.ActiveWorkspaceID != "w2" {
		t.Fatalf("expected active workspace w2, got %s", list.ActiveWorkspaceID)
	}
}

func TestLoadWorkspaceNotFound(t *testing.T) {
	m := newTestManager(t)
	if _, err := m.LoadWorkspace("missing"); err == nil {
		t.Fatal("expected error for missing workspace")
	}
}

func TestDeleteWorkspace(t *testing.T) {
	m := newTestManager(t)
	w1 := sampleWorkspaceConfig("w1")
	w2 := sampleWorkspaceConfig("w2")
	_ = m.SaveWorkspace(w1)
	_ = m.SaveWorkspace(w2)

	if err := m.DeleteWorkspace("w1"); err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	list := m.ListWorkspaces()
	if len(list.Workspaces) != 1 {
		t.Fatalf("expected 1 workspace after delete, got %d", len(list.Workspaces))
	}
	if list.ActiveWorkspaceID != "w2" {
		t.Fatalf("expected active to fall back to w2, got %s", list.ActiveWorkspaceID)
	}
}

func TestDeleteWorkspaceNotFound(t *testing.T) {
	m := newTestManager(t)
	if err := m.DeleteWorkspace("missing"); err == nil {
		t.Fatal("expected error deleting missing workspace")
	}
}

func TestDeleteLastWorkspaceClearsActive(t *testing.T) {
	m := newTestManager(t)
	w1 := sampleWorkspaceConfig("w1")
	_ = m.SaveWorkspace(w1)
	_ = m.DeleteWorkspace("w1")

	list := m.ListWorkspaces()
	if len(list.Workspaces) != 0 {
		t.Fatalf("expected 0 workspaces, got %d", len(list.Workspaces))
	}
	if list.ActiveWorkspaceID != "" {
		t.Fatalf("expected empty active ID, got %s", list.ActiveWorkspaceID)
	}
}

func TestSetActiveWorkspace(t *testing.T) {
	m := newTestManager(t)
	w1 := sampleWorkspaceConfig("w1")
	_ = m.SaveWorkspace(w1)

	if err := m.SetActiveWorkspace("w1"); err != nil {
		t.Fatalf("set active failed: %v", err)
	}
	if m.ListWorkspaces().ActiveWorkspaceID != "w1" {
		t.Fatal("expected w1 to be active")
	}
}

func TestSetActiveWorkspaceMissing(t *testing.T) {
	m := newTestManager(t)
	if err := m.SetActiveWorkspace("missing"); err == nil {
		t.Fatal("expected error for missing workspace")
	}
}

func TestPreferencesRoundTrip(t *testing.T) {
	m := newTestManager(t)
	prefs := contracts.UserPreferences{
		Theme:       "dark",
		AccentColor: "#ff0000",
		UIZoom:      120,
		CompactMode: true,
	}
	if err := m.SavePreferences(prefs); err != nil {
		t.Fatalf("save preferences failed: %v", err)
	}

	loaded := m.GetPreferences()
	if loaded.Theme != "dark" || loaded.UIZoom != 120 {
		t.Fatalf("preferences mismatch: %+v", loaded)
	}
}

func TestPersistAndLoad(t *testing.T) {
	m := newTestManager(t)
	w1 := sampleWorkspaceConfig("w1")
	_ = m.SaveWorkspace(w1)
	_ = m.SetActiveWorkspace("w1")
	_ = m.SavePreferences(contracts.UserPreferences{Theme: "light", UIZoom: 110})

	// Create a new manager pointing at the same store
	m2 := newManager(m.store)
	if err := m2.load(); err != nil {
		t.Fatalf("load failed: %v", err)
	}

	list := m2.ListWorkspaces()
	if len(list.Workspaces) != 1 {
		t.Fatalf("expected 1 workspace after load, got %d", len(list.Workspaces))
	}
	if list.ActiveWorkspaceID != "w1" {
		t.Fatalf("expected active w1 after load, got %s", list.ActiveWorkspaceID)
	}
	if m2.GetPreferences().Theme != "light" {
		t.Fatalf("expected theme light after load, got %s", m2.GetPreferences().Theme)
	}
}

func TestTransientManagerDoesNotPersist(t *testing.T) {
	m := NewTransientManager()
	w1 := sampleWorkspaceConfig("w1")
	if err := m.SaveWorkspace(w1); err != nil {
		t.Fatalf("save failed: %v", err)
	}
	// No store attached; load should be no-op and in-memory state retained
	list := m.ListWorkspaces()
	if len(list.Workspaces) != 1 {
		t.Fatalf("expected 1 workspace in transient manager, got %d", len(list.Workspaces))
	}
}

func TestDefaultPreferences(t *testing.T) {
	d := DefaultPreferences()
	if d.Theme != "system" {
		t.Fatalf("expected default theme 'system', got %s", d.Theme)
	}
	if d.UIZoom != 100 {
		t.Fatalf("expected default zoom 100, got %d", d.UIZoom)
	}
	if !d.EnableKeyboardShortcuts {
		t.Fatal("expected keyboard shortcuts enabled by default")
	}
}

func TestMergePreferenceDefaults(t *testing.T) {
	stored := Preferences{Theme: "dark"}
	merged := mergePreferenceDefaults(stored)
	if merged.Theme != "dark" {
		t.Fatalf("expected theme preserved, got %s", merged.Theme)
	}
	if merged.UIZoom <= 0 {
		t.Fatalf("expected UIZoom defaulted, got %d", merged.UIZoom)
	}
}

func TestMigrateStateV0ToV1(t *testing.T) {
	state := State{SchemaVersion: 0, Workspaces: map[string]Config{"w1": {ID: "w1"}}}
	migrated, err := migrateState(state, nil)
	if err != nil {
		t.Fatalf("migration failed: %v", err)
	}
	if migrated.SchemaVersion != CurrentStateSchemaVersion {
		t.Fatalf("expected schema version %d, got %d", CurrentStateSchemaVersion, migrated.SchemaVersion)
	}
}

func TestMigrateStateAlreadyCurrent(t *testing.T) {
	state := State{SchemaVersion: CurrentStateSchemaVersion}
	migrated, err := migrateState(state, nil)
	if err != nil {
		t.Fatalf("migration failed: %v", err)
	}
	if migrated.SchemaVersion != CurrentStateSchemaVersion {
		t.Fatalf("expected unchanged version")
	}
}

func TestMigrateStateMissingMigration(t *testing.T) {
	// Use a version below current but with no registered migration
	state := State{SchemaVersion: -1}
	if _, err := migrateState(state, nil); err == nil {
		t.Fatal("expected error for missing migration")
	}
}

func TestLoadStoredWindowStateMissing(t *testing.T) {
	ws := LoadStoredWindowState("nonexistent-app-name-12345")
	if ws != nil {
		t.Fatal("expected nil for missing state")
	}
}

func TestWindowStateConversion(t *testing.T) {
	original := &contracts.WindowState{Width: 1920, Height: 1080, X: 100, Y: 200, Maximised: true, HasPosition: true}
	stored := toStorageWindowState(original)
	if stored == nil || stored.Width != 1920 {
		t.Fatal("toStorageWindowState failed")
	}
	back := fromStorageWindowState(stored)
	if back == nil || back.Width != 1920 || !back.Maximised || !back.HasPosition {
		t.Fatal("fromStorageWindowState failed")
	}
	if fromStorageWindowState(nil) != nil {
		t.Fatal("expected nil for nil input")
	}
}

func TestConversionRoundTrip(t *testing.T) {
	original := contracts.WorkspaceConfig{
		ID:           "abc",
		Name:         "My Workspace",
		Icon:         "folder",
		Color:        "#ff00ff",
		ShowHidden:   true,
		ActivePaneID: "right",
		SplitRatio:   0.6,
		LeftPane: contracts.PaneState{
			ID: "left",
			Tabs: []contracts.TabInfo{
				{
					ID: "tab1", Title: "Tab 1", Path: "/a", SurfaceID: "dir",
					Location: &contracts.LocationRef{Location: "search://?path=/a&pattern=todo", Scheme: "search", Kind: "query"},
					Viewer: &contracts.ViewerTab{
						TargetPath: "/a/file.txt",
						ViewerID:   "text",
						AutoFollow: true,
					},
					IsClosable: &[]bool{true}[0],
					IsActive:   true,
					IsPinned:   false,
				},
			},
			ActiveTabID: "tab1",
		},
		RightPane: contracts.PaneState{
			ID:          "right",
			Tabs:        []contracts.TabInfo{},
			ActiveTabID: "",
		},
		CreatedAt:      1000,
		LastAccessedAt: 2000,
	}

	stored := toStorageWorkspaceConfig(original)
	back := fromStorageWorkspaceConfig(stored)

	if back.ID != original.ID || back.Name != original.Name {
		t.Fatal("basic fields mismatch")
	}
	if back.ActivePaneID != original.ActivePaneID {
		t.Fatalf("active pane id mismatch: %s vs %s", back.ActivePaneID, original.ActivePaneID)
	}
	if back.SplitRatio != original.SplitRatio {
		t.Fatalf("split ratio mismatch")
	}
	if back.LeftPane.ActiveTabID != original.LeftPane.ActiveTabID {
		t.Fatal("left pane mismatch")
	}
	if back.LeftPane.Tabs[0].Location == nil || back.LeftPane.Tabs[0].Location.Scheme != "search" || back.LeftPane.Tabs[0].Location.Kind != "query" {
		t.Fatal("tab location lost in round-trip")
	}
	if back.LeftPane.Tabs[0].SurfaceID != "dir" {
		t.Fatalf("surface id lost in round-trip: %q", back.LeftPane.Tabs[0].SurfaceID)
	}
	if back.LeftPane.Tabs[0].Viewer == nil {
		t.Fatal("viewer lost in round-trip")
	}
	if back.LeftPane.Tabs[0].Viewer.TargetPath != "/a/file.txt" {
		t.Fatal("viewer target path mismatch")
	}
	if back.LeftPane.Tabs[0].IsClosable == nil || !*back.LeftPane.Tabs[0].IsClosable {
		t.Fatal("isClosable lost in round-trip")
	}
}

func TestServiceDelegation(t *testing.T) {
	m := newTestManager(t)
	svc := NewService(m)

	w1 := sampleWorkspaceConfig("w1")
	if err := svc.SaveWorkspace(w1); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	list := svc.GetWorkspaces()
	if len(list.Workspaces) != 1 {
		t.Fatalf("expected 1 workspace, got %d", len(list.Workspaces))
	}

	loaded, err := svc.LoadWorkspace("w1")
	if err != nil || loaded.ID != "w1" {
		t.Fatalf("load failed: %v", err)
	}

	if err := svc.DeleteWorkspace("w1"); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	if len(svc.GetWorkspaces().Workspaces) != 0 {
		t.Fatal("expected 0 workspaces after delete")
	}

	prefs := contracts.UserPreferences{Theme: "dark", UIZoom: 125}
	if err := svc.SavePreferences(prefs); err != nil {
		t.Fatalf("save preferences failed: %v", err)
	}
	if svc.GetPreferences().Theme != "dark" {
		t.Fatal("preferences delegation failed")
	}
}

func TestStoreReadWrite(t *testing.T) {
	tmp := t.TempDir()
	store := &Store{statePath: filepath.Join(tmp, "test.json")}

	data := []byte(`{"hello":"world"}`)
	if err := store.WriteState(data); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	read, err := store.ReadState()
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if string(read) != string(data) {
		t.Fatalf("read mismatch: %s vs %s", string(read), string(data))
	}

	if store.StatePath() != filepath.Join(tmp, "test.json") {
		t.Fatal("state path mismatch")
	}
}

func TestStoreReadMissing(t *testing.T) {
	tmp := t.TempDir()
	store := &Store{statePath: filepath.Join(tmp, "missing.json")}
	if _, err := store.ReadState(); !os.IsNotExist(err) {
		t.Fatalf("expected not-exist error, got %v", err)
	}
}