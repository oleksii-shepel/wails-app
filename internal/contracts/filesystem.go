package contracts

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
	IsSymlink   bool     `json:"isSymlink,omitempty"`
	IsNested    bool     `json:"isNested"`
	NestedChain []string `json:"nestedChain,omitempty"`
}

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
