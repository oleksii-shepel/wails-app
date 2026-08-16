package contracts

type ConflictResolution string

const (
	ConflictSkip      ConflictResolution = "skip"
	ConflictOverwrite ConflictResolution = "overwrite"
	ConflictRename    ConflictResolution = "rename"
	ConflictCancel    ConflictResolution = "cancel"
)

type ConflictEvent struct {
	OperationID string `json:"operationId"`
	ConflictID  string `json:"conflictId"`
	SourcePath  string `json:"sourcePath"`
	DestPath    string `json:"destPath"`
	SourceSize  int64  `json:"sourceSize"`
	DestSize    int64  `json:"destSize"`
	SourceMtime int64  `json:"sourceMtime"`
	DestMtime   int64  `json:"destMtime"`
	IsDirectory bool   `json:"isDirectory"`
}

type ConflictResolutionRequest struct {
	Resolution ConflictResolution `json:"resolution"`
	ApplyToAll bool               `json:"applyToAll"`
}

type BatchItem struct {
	Source           string `json:"source"`
	Destination      string `json:"destination"`
	ExactDestination bool   `json:"exactDestination,omitempty"`
}

type BatchRequest struct {
	Items     []BatchItem `json:"items"`
	Recursive bool        `json:"recursive"`
}

type BatchAffectedPath struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
}

type BatchResult struct {
	OperationID   string              `json:"operationId"`
	Total         int                 `json:"total"`
	Succeeded     int                 `json:"succeeded"`
	Skipped       int                 `json:"skipped"`
	Failed        int                 `json:"failed"`
	Errors        []BatchItemError    `json:"errors,omitempty"`
	AffectedPaths []BatchAffectedPath `json:"affectedPaths,omitempty"`
}

type BatchDeleteRequest struct {
	Paths     []string `json:"paths"`
	Recursive bool     `json:"recursive"`
	UseTrash  *bool    `json:"useTrash"`
}

type BatchExtractRequest struct {
	Archives             []string `json:"archives"`
	DestinationDir       string   `json:"destinationDir"`
	CreateSubdirectory   bool     `json:"createSubdirectory"`
	ExpandNestedArchives bool     `json:"expandNestedArchives"`
	MaxNestDepth         int      `json:"maxNestDepth"`
	FlattenSingleRoot    bool     `json:"flattenSingleRoot"`
}

type BatchExtractResult struct {
	OperationID    string           `json:"operationId"`
	Total          int              `json:"total"`
	Succeeded      int              `json:"succeeded"`
	Skipped        int              `json:"skipped"`
	Failed         int              `json:"failed"`
	ExtractedFiles int64            `json:"extractedFiles"`
	ExtractedBytes int64            `json:"extractedBytes"`
	Errors         []BatchItemError `json:"errors,omitempty"`
}

type SelectiveExtractItem struct {
	ArchivePath    string   `json:"archivePath"`
	NestedChain    []string `json:"nestedChain"`
	Entries        []string `json:"entries"`
	StripPrefix    string   `json:"stripPrefix,omitempty"`
	DestinationDir string   `json:"destinationDir,omitempty"`
	FlattenPaths   bool     `json:"flattenPaths"`
}

type BatchSelectiveExtractRequest struct {
	Items          []SelectiveExtractItem `json:"items"`
	DestinationDir string                 `json:"destinationDir"`
}

type BatchSelectiveExtractResult struct {
	OperationID    string           `json:"operationId"`
	Total          int              `json:"total"`
	Succeeded      int              `json:"succeeded"`
	Skipped        int              `json:"skipped"`
	Failed         int              `json:"failed"`
	ExtractedFiles int64            `json:"extractedFiles"`
	ExtractedBytes int64            `json:"extractedBytes"`
	Errors         []BatchItemError `json:"errors,omitempty"`
}
