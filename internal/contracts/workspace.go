package contracts

type TabInfo struct {
	ID            string     `json:"id"`
	Title         string     `json:"title"`
	Path          string     `json:"path"`
	Kind          string     `json:"kind,omitempty"`
	Viewer        *ViewerTab `json:"viewer,omitempty"`
	IsClosable    *bool      `json:"isClosable,omitempty"`
	IsActive      bool       `json:"isActive"`
	IsPinned      bool       `json:"isPinned"`
	CreatedAt     int64      `json:"createdAt"`
	ViewStateJson string     `json:"viewStateJson,omitempty"`
}

type ViewerTab struct {
	TargetPath  string `json:"targetPath"`
	ViewerID    string `json:"viewerId,omitempty"`
	AutoFollow  bool   `json:"autoFollow,omitempty"`
	SourcePane  string `json:"sourcePane,omitempty"`
	SourceTabID string `json:"sourceTabId,omitempty"`
	SourceDir   string `json:"sourceDir,omitempty"`
}

type PaneState struct {
	ID          string    `json:"id"`
	Tabs        []TabInfo `json:"tabs"`
	ActiveTabID string    `json:"activeTabId"`
}

type WorkspaceConfig struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Icon           string    `json:"icon,omitempty"`
	Color          string    `json:"color,omitempty"`
	LeftPane       PaneState `json:"leftPane"`
	RightPane      PaneState `json:"rightPane"`
	ShowHidden     bool      `json:"showHidden"`
	ActivePaneID   string    `json:"activePaneId,omitempty"`
	SplitRatio     float64   `json:"splitRatio,omitempty"`
	CreatedAt      int64     `json:"createdAt"`
	LastAccessedAt int64     `json:"lastAccessedAt"`
}

type WorkspaceList struct {
	Workspaces        []WorkspaceConfig `json:"workspaces"`
	ActiveWorkspaceID string            `json:"activeWorkspaceId"`
}

type UserPreferences struct {
	Theme                   string       `json:"theme"`
	AccentColor             string       `json:"accentColor"`
	UIZoom                  int          `json:"uiZoom"`
	CompactMode             bool         `json:"compactMode"`
	AnimationsEnabled       bool         `json:"animationsEnabled"`
	WindowState             *WindowState `json:"windowState,omitempty"`
	DefaultShowHidden       bool         `json:"defaultShowHidden"`
	DefaultSortBy           string       `json:"defaultSortBy"`
	DefaultSortOrder        string       `json:"defaultSortOrder"`
	EnableKeyboardShortcuts bool         `json:"enableKeyboardShortcuts"`
	DoubleClickAction       string       `json:"doubleClickAction"`
	ConfirmDelete           bool         `json:"confirmDelete"`
	ConfirmOnClose          bool         `json:"confirmOnClose"`
	ReopenLastWorkspace     bool         `json:"reopenLastWorkspace"`
	MemoryRoot              string       `json:"memoryRoot,omitempty"`
}

type WindowState struct {
	Width     int  `json:"width"`
	Height    int  `json:"height"`
	X         int  `json:"x"`
	Y         int  `json:"y"`
	Maximised bool `json:"maximised"`
	// HasPosition distinguishes "X/Y were captured" from "X/Y default to 0".
	// Legacy records without this field are treated as having a position iff
	// X or Y is non-zero.
	HasPosition bool `json:"hasPosition,omitempty"`
}
