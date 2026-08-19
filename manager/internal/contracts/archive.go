package contracts

type ArchiveEntriesRequest struct {
	FsPath         string   `json:"fsPath"`
	NestedChain    []string `json:"nestedChain"`
	Entries        []string `json:"entries"`
	StripPrefix    string   `json:"stripPrefix,omitempty"`
	DestinationDir string   `json:"destinationDir"`
	FlattenPaths   bool     `json:"flattenPaths"`
}

type ArchiveEntriesResult struct {
	OperationID   string           `json:"operationId"`
	Total         int              `json:"total"`
	Succeeded     int              `json:"succeeded"`
	Failed        int              `json:"failed"`
	SourceRemoved bool             `json:"sourceRemoved"`
	Errors        []BatchItemError `json:"errors,omitempty"`
}