package workspace

import "testing"

func TestStoredStateRequiresElevation(t *testing.T) {
	t.Run("active workspace registry tab requires elevation", func(t *testing.T) {
		state := State{
			ActiveWorkspaceID: "registry",
			Preferences: Preferences{
				ReopenLastWorkspace: true,
			},
			Workspaces: map[string]Config{
				"registry": {
					LeftPane: PaneState{
						Tabs: []TabInfo{
							{
								Path: "registry://HKEY_LOCAL_MACHINE/Software",
							},
						},
					},
				},
				"filesystem": {
					LeftPane: PaneState{
						Tabs: []TabInfo{
							{
								Path: "filesystem://c/Users",
							},
						},
					},
				},
			},
		}

		if !storedStateRequiresElevation(state) {
			t.Fatal("expected registry workspace to require elevation")
		}
	})

	t.Run("non-active registry workspace is ignored when reopening last workspace", func(t *testing.T) {
		state := State{
			ActiveWorkspaceID: "filesystem",
			Preferences: Preferences{
				ReopenLastWorkspace: true,
			},
			Workspaces: map[string]Config{
				"filesystem": {
					LeftPane: PaneState{
						Tabs: []TabInfo{
							{
								Path: "filesystem://c/Users",
							},
						},
					},
				},
				"registry": {
					LeftPane: PaneState{
						Tabs: []TabInfo{
							{
								Path: "registry://HKEY_CURRENT_USER/Software",
							},
						},
					},
				},
			},
		}

		if storedStateRequiresElevation(state) {
			t.Fatal("did not expect non-active registry workspace to require elevation when reopening the last workspace")
		}
	})

	t.Run("any registry workspace requires elevation when startup workspace is ambiguous", func(t *testing.T) {
		state := State{
			Preferences: Preferences{
				ReopenLastWorkspace: false,
			},
			Workspaces: map[string]Config{
				"filesystem": {
					LeftPane: PaneState{
						Tabs: []TabInfo{
							{
								Path: "filesystem://c/Users",
							},
						},
					},
				},
				"registry": {
					RightPane: PaneState{
						Tabs: []TabInfo{
							{
								Location: &LocationRef{Location: "registry://HKEY_USERS"},
							},
						},
					},
				},
			},
		}

		if !storedStateRequiresElevation(state) {
			t.Fatal("expected registry workspace to require elevation when startup workspace is ambiguous")
		}
	})
}
