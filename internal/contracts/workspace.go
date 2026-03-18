package contracts

type TabInfo struct {
	ID         string     `json:"id"`
	Title      string     `json:"title"`
	Path       string     `json:"path"`
	Kind       string     `json:"kind,omitempty"`
	Viewer     *ViewerTab `json:"viewer,omitempty"`
	IsClosable *bool      `json:"isClosable,omitempty"`
	IsActive   bool       `json:"isActive"`
	IsPinned   bool       `json:"isPinned"`
	CreatedAt  int64      `json:"createdAt"`
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
	CreatedAt      int64     `json:"createdAt"`
	LastAccessedAt int64     `json:"lastAccessedAt"`
}

type WorkspaceList struct {
	Workspaces        []WorkspaceConfig `json:"workspaces"`
	ActiveWorkspaceID string            `json:"activeWorkspaceId"`
}

type UserPreferences struct {
	Theme                   string `json:"theme"`
	DefaultShowHidden       bool   `json:"defaultShowHidden"`
	DefaultSortBy           string `json:"defaultSortBy"`
	DefaultSortOrder        string `json:"defaultSortOrder"`
	EnableKeyboardShortcuts bool   `json:"enableKeyboardShortcuts"`
	DoubleClickAction       string `json:"doubleClickAction"`
	ConfirmDelete           bool   `json:"confirmDelete"`
}
