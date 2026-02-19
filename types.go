package main

// ============================================================================
// OS Info
// ============================================================================

type OSType string

const (
	OSWindows OSType = "windows"
	OSLinux   OSType = "linux"
	OSDarwin  OSType = "macos"
	OSUnknown OSType = "unknown"
)

type OSInfoResponse struct {
	OS          OSType  `json:"os"`
	Version     string  `json:"version"`
	Arch        string  `json:"arch"`
	Hostname    string  `json:"hostname"`
	SystemDrive *string `json:"systemDrive"`
}

// ============================================================================
// Drives
// ============================================================================

type DriveType string

const (
	DriveFixed     DriveType = "fixed"
	DriveRemovable DriveType = "removable"
	DriveNetwork   DriveType = "network"
	DriveCDROM     DriveType = "cdrom"
	DriveRAMDisk   DriveType = "ramdisk"
	DriveUnknown   DriveType = "unknown"
)

type DriveInfo struct {
	Name       string    `json:"name"`
	Path       string    `json:"path"`
	DriveType  DriveType `json:"driveType"`
	TotalSpace int64     `json:"totalSpace"`
	FreeSpace  int64     `json:"freeSpace"`
	FileSystem *string   `json:"fileSystem"`
}

type DrivesList struct {
	Drives []DriveInfo `json:"drives"`
}

// ============================================================================
// File System Data Types
// ============================================================================

type FileType string

const (
	FileTypeFile      FileType = "FILE"
	FileTypeArchive   FileType = "ARCHIVE"
	FileTypeDirectory FileType = "DIRECTORY"
	FileTypeSymlink   FileType = "SYMLINK"
)

type FileInfo struct {
	Name        string   `json:"name"`
	Path        string   `json:"path"`
	Type        FileType `json:"type"`
	Size        int64    `json:"size"`
	Created     int64    `json:"created"`
	Modified    int64    `json:"modified"`
	Accessed    int64    `json:"accessed"`
	Permissions string   `json:"permissions"`
	IsHidden    bool     `json:"isHidden"`
	IsNested    bool     `json:"isNested"`
	NestedChain []string `json:"nestedChain,omitempty"`
}

// Add this struct to fix "undefined: ArchiveListing"
type ArchiveListing struct {
	ArchivePath string     `json:"archivePath"`
	InnerPath   string     `json:"innerPath"`
	NestedChain []string   `json:"nestedChain,omitempty"`
	Entries     []FileInfo `json:"entries"`
}

type DirectoryListing struct {
	Path      string     `json:"path"`
	Entries   []FileInfo `json:"entries"`
	TotalSize int64      `json:"totalSize"`
}

type FileContent struct {
	Path     string `json:"path"`
	Content  string `json:"content"`
	Encoding string `json:"encoding"`
	Size     int64  `json:"size"`
}

type OperationResult struct {
	Success       bool     `json:"success"`
	Message       string   `json:"message,omitempty"`
	AffectedPaths []string `json:"affectedPaths,omitempty"`
}

type SearchResult struct {
	Path         string     `json:"path"`
	Matches      []FileInfo `json:"matches"`
	TotalMatches int        `json:"totalMatches"`
}

// ============================================================================
// Response Envelope
// ============================================================================

type ResponseStatus string

const (
	StatusSuccess ResponseStatus = "SUCCESS"
	StatusError   ResponseStatus = "ERROR"
)

type ErrorDetail struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
}

type AppResponse struct {
	CommandID string         `json:"commandId"`
	Status    ResponseStatus `json:"status"`
	Timestamp int64          `json:"timestamp"`
	Data      interface{}    `json:"data,omitempty"`
	Error     *ErrorDetail   `json:"error,omitempty"`
}

// ============================================================================
// Navigation History
// ============================================================================

type HistoryEntry struct {
	Path      string `json:"path"`
	Timestamp int64  `json:"timestamp"`
	PaneID    string `json:"paneId"`
}

type NavigationHistory struct {
	Entries    []HistoryEntry `json:"entries"`
	MaxEntries int            `json:"maxEntries"`
}

// ============================================================================
// Tabs & Workspace
// ============================================================================

type TabInfo struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Path      string `json:"path"`
	IsActive  bool   `json:"isActive"`
	IsPinned  bool   `json:"isPinned"`
	CreatedAt int64  `json:"createdAt"`
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

// ============================================================================
// Filter System
// ============================================================================

type FilterType string

const (
	FilterName      FilterType = "name"
	FilterExtension FilterType = "ext"
	FilterType_     FilterType = "type"
	FilterSize      FilterType = "size"
	FilterModified  FilterType = "modified"
	FilterGlob      FilterType = "glob"
)

type FilterCriteria struct {
	Type     FilterType `json:"type"`
	Value    string     `json:"value"`
	Operator string     `json:"operator,omitempty"`
}

type FilterQuery struct {
	Text          string           `json:"text,omitempty"`
	Criteria      []FilterCriteria `json:"criteria,omitempty"`
	CaseSensitive bool             `json:"caseSensitive"`
}

// ============================================================================
// User Preferences
// ============================================================================

type UserPreferences struct {
	Theme                   string `json:"theme"`
	DefaultShowHidden       bool   `json:"defaultShowHidden"`
	DefaultSortBy           string `json:"defaultSortBy"`
	DefaultSortOrder        string `json:"defaultSortOrder"`
	EnableKeyboardShortcuts bool   `json:"enableKeyboardShortcuts"`
	DoubleClickAction       string `json:"doubleClickAction"`
	ConfirmDelete           bool   `json:"confirmDelete"`
}
